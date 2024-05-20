package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/kardianos/service"
	"github.com/kbinani/screenshot"
	"github.com/nfnt/resize"
	"image"
	"image/jpeg"
	"log"
	"net"
	"os"
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

type program struct {
	exit chan struct{}
}

func resizeImage(img image.Image, width, height uint) image.Image {
	return resize.Resize(width, height, img, resize.Lanczos3)
}

func (p *program) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	p.exit = make(chan struct{})
	go p.run()
	return nil
}

func (p *program) run() {
	log.Println("Starting EduViewClient service...")

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	var ipAddress string

	for ipAddress == "" {
		ipAddress = scanForServer()
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

func (p *program) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	close(p.exit)
	log.Println("Stopping EduViewClient service...")
	return nil
}

func scanForServer() string {
	localIPs, err := getLocalIPs()
	if err != nil {
		log.Fatal("Error getting local IPs:", err)
	}

	for _, ip := range localIPs {
		for i := 1; i <= 254; i++ { // Scan range 1-254 for simplicity
			address := fmt.Sprintf("%s.%d:8093", ip, i)
			conn, err := net.DialTimeout("tcp", address, time.Millisecond)
			if err == nil {
				conn.Close()
				fmt.Println("Found server at:", address)
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
		log.Println("Connection error:", err)
		return nil, err
	}
	log.Println("Connected to server")
	return conn, nil
}

func handleConnection(conn net.Conn, x, y int, hostname string) {
	defer conn.Close()
	sendHostname(conn, hostname)

	for {
		img, _ := screenshot.CaptureDisplay(0)
		resizedImg := resizeImage(img, uint(x), uint(y))

		var buf bytes.Buffer
		jpeg.Encode(&buf, resizedImg, nil)

		err := sendImage(conn, buf.Bytes())
		if err != nil {
			log.Println("Disconnected from server")
			return
		}
	}
}

func sendHostname(conn net.Conn, hostname string) {
	hostnameBytes := []byte(hostname)
	size := int64(len(hostnameBytes))
	err := binary.Write(conn, binary.LittleEndian, size)
	if err != nil {
		log.Println("Error sending hostname size:", err)
		return
	}

	_, err = conn.Write(hostnameBytes)
	if err != nil {
		log.Println("Error sending hostname:", err)
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

func main() {
	// Creează un fișier de log
	logFile, err := os.OpenFile("EduViewClient.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)

	svcConfig := &service.Config{
		Name:        "EduViewClient",
		DisplayName: "EduView Client Service",
		Description: "This is a client service for EduView.",
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal("Error creating service:", err)
	}

	err = s.Run()
	if err != nil {
		log.Fatal("Error running service:", err)
	}
}
