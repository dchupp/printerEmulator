package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	CONN_HOST = "127.0.0.1"
	CONN_PORT = "9100"
	CONN_TYPE = "tcp"
)

func main() {
	// Listen for incoming connections.
	l, err := net.Listen(CONN_TYPE, CONN_HOST+":"+CONN_PORT)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	// Close the listener when the application closes.
	defer l.Close()
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Starting Printer Emulator.....")
	fmt.Println("Printer Emulation Settings:")
	fmt.Println("Width of label (in):")
	width, err := reader.ReadString('\n')
	fmt.Println("Height of Label (in)")
	height, err := reader.ReadString('\n')
	fmt.Println("Listening on " + CONN_HOST + ":" + CONN_PORT + " for print jobs")
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		// Handle connections in a new goroutine.
		go handleRequest(conn, width, height)
	}
}

// Handles incoming requests.
func handleRequest(conn net.Conn, width string, height string) {

	// Close connection when this function ends
	defer func() {
		conn.Close()
	}()

	timeoutDuration := 5 * time.Second
	bufferReader := bufio.NewReader(conn)
	var lines []string
	for {

		conn.SetReadDeadline(time.Now().Add(timeoutDuration))

		for {
			line, err := bufferReader.ReadString('\n')
			if err != nil {
				lines = append(lines, line)
				if err != io.EOF {
					fmt.Println(err)
				}
				break
			}
			lines = append(lines, line)

		}
		break
	}

	messageString := strings.Join(lines, "")
	fmt.Println(messageString)
	resp, err := http.Post(fmt.Sprintf("http://api.labelary.com/v1/printers/8dpmm/labels/%sx%s/0/"))
}
