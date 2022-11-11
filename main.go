package main

import (
	"bufio"
	"context"
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
	fmt.Println("Github: https://github.com/dchupp/printerEmulator")
	fmt.Println("_________________________________________________")
	fmt.Println()
	fmt.Println("Starting Printer Emulator.....")
	fmt.Println("Printer Emulation Settings:")
	fmt.Println("Width of label (in):")
	width, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	width = strings.Replace(width, "\r\n", "", -1)
	fmt.Println("Height of Label (in)")
	height, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	height = strings.Replace(height, "\r\n", "", -1)
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

	messageString := strings.Join(lines, "")
	fmt.Println(messageString)
	err := SendToLabelary(messageString, width, height)
	if err != nil {
		fmt.Println(err)
	}
}

func SendToLabelary(zpl string, width string, height string) error {

	req, err := http.NewRequestWithContext(context.TODO(), "POST", fmt.Sprintf("http://api.labelary.com/v1/printers/8dpmm/labels/%sx%s/0/", width, height), strings.NewReader(zpl))
	if err != nil {
		fmt.Printf("client: could not create request: %s\n", err)
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "image/png")
	client := http.Client{
		Timeout: 30 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	fname := fmt.Sprintf("label-print-%d_%d_%d-%d-%d-%d.png", time.Now().Month(), time.Now().Day(), time.Now().Year(), time.Now().Hour(), time.Now().Minute(), time.Now().Second())

	f, err := os.Create(fname)

	if err != nil {
		return err
	}
	defer res.Body.Close()
	_, err = io.Copy(f, res.Body)

	if err != nil {
		return err
	}
	fmt.Printf("%s created!", fname)
	defer f.Close()
	return nil
}
