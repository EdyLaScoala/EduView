package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/kbinani/screenshot"
	"github.com/nfnt/resize"
	"image"
	"image/jpeg"
	"net"
	"os"
	"strings"
	"time"
)

func resizeImage(img image.Image, width, height uint) image.Image {
	return resize.Resize(width, height, img, resize.Lanczos3)
}

func main() {
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println(err)
	}

	var ipAddress string = "192.168.3.197:8093"

	for ipAddress == "" {
		ipAddress = scanForServer()
	}

	bounds := screenshot.GetDisplayBounds(0)
	var x int = 2 * bounds.Dx() / 3
	var y int = 2 * bounds.Dy() / 3

	for {
		conn, err := connectToServer(ipAddress)
		if err != nil {
			fmt.Println("Trying to reconnect in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		}
		handleConnection(conn, x, y, hostname)
	}
}

func scanForServer() string {
	localIPs, err := getLocalIPs()
	if err != nil {
		fmt.Println("Error getting local IPs:", err)
	}

	for _, ip := range localIPs {
		for i := 1; i <= 254; i++ {
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
		fmt.Println("Connection error:", err)
		return nil, err
	}
	fmt.Println("Connected to server")
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
			fmt.Println("Disconnected from server")
			return
		}
	}
}

func sendHostname(conn net.Conn, hostname string) {
	hostnameBytes := []byte(hostname)
	size := int64(len(hostnameBytes))
	err := binary.Write(conn, binary.LittleEndian, size)
	if err != nil {
		fmt.Println("Error sending hostname size:", err)
		return
	}

	_, err = conn.Write(hostnameBytes)
	if err != nil {
		fmt.Println("Error sending hostname:", err)
	}
	fmt.Println("Sent hostname:", hostname)
}

func sendImage(conn net.Conn, imageData []byte) error {
	size := int64(len(imageData))
	fmt.Println("Sending image size:", size)
	err := binary.Write(conn, binary.LittleEndian, size)
	if err != nil {
		return err
	}

	fmt.Println("Sending image data")
	_, err = conn.Write(imageData)
	if err != nil {
		return err
	}

	fmt.Println("Image sent")
	return nil
}
