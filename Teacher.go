package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/kbinani/screenshot"
	"github.com/nfnt/resize"
	"image"
	"image/jpeg"
	"io"
	"net"
	"sync"
	"time"
)

var (
	mu            sync.Mutex
	clients       = make(map[string]net.Conn)
	currentClient string
	imgChan       = make(chan image.Image)
)

func main() {
	listener, err := net.Listen("tcp", ":8093")
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	defer listener.Close()

	showServerIP()

	fmt.Println("Server is listening on port 8093")

	go acceptConnections(listener)

	a := app.New()
	w := a.NewWindow("Received Video Stream")

	ipDropdown := widget.NewSelect([]string{}, func(value string) {
		mu.Lock()
		currentClient = value
		mu.Unlock()
		fmt.Println("Selected client:", value)
	})

	fyneImg := canvas.NewImageFromImage(nil)
	fyneImg.FillMode = canvas.ImageFillOriginal

	content := container.NewVBox(ipDropdown, fyneImg)
	w.SetContent(content)

	go func() {
		for img := range imgChan {
			fyneImg.Image = img
			fyneImg.Refresh()
		}
	}()

	go updateIPDropdown(ipDropdown)

	w.ShowAndRun()
}

func acceptConnections(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		hostname := receiveHostname(conn)
		if hostname == "" {
			fmt.Println("Failed to receive hostname, closing connection")
			conn.Close()
			continue
		}

		fmt.Println("Accepted connection from:", hostname)

		mu.Lock()
		clients[hostname] = conn
		if currentClient == "" {
			currentClient = hostname
		}
		mu.Unlock()

		go handleConnection(conn, hostname)
	}
}

func receiveHostname(conn net.Conn) string {
	var size int64
	err := binary.Read(conn, binary.LittleEndian, &size)
	if err != nil {
		fmt.Println("Error reading hostname size:", err)
		return ""
	}

	buf := make([]byte, size)
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		fmt.Println("Error reading hostname:", err)
		return ""
	}

	return string(buf)
}

func handleConnection(conn net.Conn, hostname string) {
	defer func() {
		mu.Lock()
		delete(clients, hostname)
		if currentClient == hostname {
			if len(clients) > 0 {
				for hn := range clients {
					currentClient = hn
					break
				}
			} else {
				currentClient = ""
			}
		}
		mu.Unlock()
		conn.Close()
		fmt.Println("Closed connection from:", hostname)
	}()

	for {
		var size int64
		err := binary.Read(conn, binary.LittleEndian, &size)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Client disconnected:", hostname)
				return
			}
			fmt.Println("Error reading image size from", hostname, ":", err)
			return
		}

		buf := make([]byte, size)
		_, err = io.ReadFull(conn, buf)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Client disconnected:", hostname)
				return
			}
			fmt.Println("Error receiving image from", hostname, ":", err)
			return
		}

		img, err := jpeg.Decode(bytes.NewReader(buf))
		if err != nil {
			fmt.Println("Error decoding image from", hostname, ":", err)
			return
		}

		screenBounds := screenshot.GetDisplayBounds(0)
		maxWidth := 4 * screenBounds.Dx() / 5
		maxHeight := 4 * screenBounds.Dy() / 5

		if img.Bounds().Dx() > maxWidth || img.Bounds().Dy() > maxHeight {
			img = resize.Resize(uint(maxWidth), uint(maxHeight), img, resize.Lanczos3)
		}

		mu.Lock()
		if hostname == currentClient {
			imgChan <- img
		}
		mu.Unlock()
	}
}

func updateIPDropdown(ipDropdown *widget.Select) {
	for {
		time.Sleep(5 * time.Second)

		mu.Lock()
		hostnames := make([]string, 0, len(clients))
		for hn := range clients {
			hostnames = append(hostnames, hn)
		}
		mu.Unlock()

		ipDropdown.Options = hostnames
		if len(hostnames) > 0 && currentClient == "" {
			currentClient = hostnames[0]
			ipDropdown.SetSelected(currentClient)
		}
	}
}

func showServerIP() {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Println("Error getting IP addresses:", err)
		return
	}

	fmt.Println("Server IP addresses:")
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				fmt.Println(ipnet.IP.String())
			}
		}
	}
}
