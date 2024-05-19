package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/kbinani/screenshot"
	"github.com/nfnt/resize"
	"image"
	"image/jpeg"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	mu            sync.Mutex
	clients       = make(map[string]net.Conn)
	currentClient string
	imgChan       = make(chan image.Image)
)

func resizeImage(img image.Image, width, height uint) image.Image {
	return resize.Resize(width, height, img, resize.Lanczos3)
}

func main() {
	// Determină calea completă către directorul de loguri
	logDir := filepath.Join("C:", "ProgramData", "EduViewClient")
	logFilePath := filepath.Join(logDir, "EduViewClient.log")

	// Creează directorul de loguri dacă nu există
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		err := os.MkdirAll(logDir, 0755)
		if err != nil {
			log.Fatalf("Failed to create log directory: %v", err)
		}
	}

	// Creează un fișier de log
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)

	log.Println("Starting EduViewClient...")

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Error getting hostname: %v", err)
	}

	var ipAddress string

	for ipAddress == "" {
		ipAddress = scanForServer()
		if ipAddress == "" {
			log.Println("Server not found, retrying in 5 seconds...")
			time.Sleep(5 * time.Second)
		}
	}

	bounds := screenshot.GetDisplayBounds(0)
	var x int = 2 * bounds.Dx() / 3
	var y int = 2 * bounds.Dy() / 3

	for {
		conn, err := connectToServer(ipAddress)
		if err != nil {
			log.Println("Trying to reconnect in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		}
		handleConnection(conn, x, y, hostname)
	}
}

func scanForServer() string {
	localIPs, err := getLocalIPs()
	if err != nil {
		log.Fatalf("Error getting local IPs: %v", err)
	}

	for _, ip := range localIPs {
		for i := 1; i <= 254; i++ {
			address := fmt.Sprintf("%s.%d:8093", ip, i)
			conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
			if err == nil {
				conn.Close()
				log.Printf("Found server at: %s", address)
				return address
			}
		}
	}
	return ""
}

func getLocalIPs() ([]string, error) {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String()[:strings.LastIndex(ipnet.IP.String(), ".")])
			}
		}
	}
	return ips, nil
}

func connectToServer(IP_ADDRESS string) (net.Conn, error) {
	conn, err := net.Dial("tcp", IP_ADDRESS)
	if err != nil {
		log.Printf("Connection error: %v", err)
		return nil, err
	}
	log.Println("Connected to server")
	return conn, nil
}

func handleConnection(conn net.Conn, x, y int, hostname string) {
	defer conn.Close()
	sendHostname(conn, hostname)

	for {
		img, err := screenshot.CaptureDisplay(0)
		if err != nil {
			log.Printf("Error capturing display: %v", err)
			return
		}
		resizedImg := resizeImage(img, uint(x), uint(y))

		var buf bytes.Buffer
		err = jpeg.Encode(&buf, resizedImg, nil)
		if err != nil {
			log.Printf("Error encoding image: %v", err)
			return
		}

		err = sendImage(conn, buf.Bytes())
		if err != nil {
			log.Printf("Error sending image: %v", err)
			return
		}
	}
}

func sendHostname(conn net.Conn, hostname string) {
	hostnameBytes := []byte(hostname)
	size := int64(len(hostnameBytes))
	err := binary.Write(conn, binary.LittleEndian, size)
	if err != nil {
		log.Printf("Error sending hostname size: %v", err)
		return
	}

	_, err = conn.Write(hostnameBytes)
	if err != nil {
		log.Printf("Error sending hostname: %v", err)
	}
	log.Println("Sent hostname:", hostname)
}

func sendImage(conn net.Conn, imageData []byte) error {
	size := int64(len(imageData))
	log.Println("Sending image size:", size)
	err := binary.Write(conn, binary.LittleEndian, size)
	if err != nil {
		return err
	}

	log.Println("Sending image data")
	_, err = conn.Write(imageData)
	if err != nil {
		return err
	}

	log.Println("Image sent")
	return nil
}
