package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	CONN_HOST = "127.0.0.1"
	// CONN_PORT = 9100
	// CONN_TYPE     = "tcp"
	// PrintWidth    = 4
	// PrintHeight   = 6
	// StopRun       = false
	Running         = false
	SaveToFile      = false
	PrintMode       = 0
	SelectedPrinter Printer
	LabelRelayGroup RelayGroup
	// FilePath      = ""
	// DPI           PrinterDPI
	// PrintRotation = 0
)

type TCPServer struct {
	listener net.Listener
	quit     chan any
	wg       sync.WaitGroup
}

type PrinterDPI struct {
	Dpi         int    `json:"value"`
	Description string `json:"desc"`
}

func (a *App) NewTCPServer() *TCPServer {
	s := &TCPServer{
		quit: make(chan interface{}),
	}
	addressString := fmt.Sprintf("%s:%d", CONN_HOST, int(a.Settings.PrinterPort))

	l, err := net.Listen("tcp", addressString)
	if err != nil {
		var dialog runtime.MessageDialogOptions
		dialog.Title = "Error Starting Printer Server"
		dialog.Message = err.Error()
		dialog.Type = runtime.ErrorDialog
		runtime.MessageDialog(a.ctx, dialog)
		fmt.Println(err)
		runtime.EventsEmit(a.ctx, "Unblock")

		return nil
	}
	s.listener = l
	s.wg.Add(1)
	Running = true
	runtime.EventsEmit(a.ctx, "Unblock")

	go a.serve()

	return s
}
func (a *App) serve() {

	defer a.tcp.wg.Done()

	for {
		conn, err := a.tcp.listener.Accept()
		if err != nil {
			select {
			case <-a.tcp.quit:
				return
			default:
				log.Println("accept error", err)
			}
		} else {
			a.tcp.wg.Add(1)
			go func() {
				go a.handleRequest(conn, strconv.Itoa(int(a.Settings.PrintWidth)), strconv.Itoa(int(a.Settings.PrintHeight)))
				a.tcp.wg.Done()
			}()
		}
	}
}
func (s *TCPServer) GetStatus(a App) bool {
	addressString := net.JoinHostPort(CONN_HOST, fmt.Sprintf("%d", a.Settings.PrinterPort))
	conn, err := net.Dial("tcp", addressString)
	if err != nil {
		return false
	} else {
		conn.Close()
		return true
	}
}
func (s *TCPServer) Stop() {
	close(s.quit)
	s.listener.Close()
	waitTimeout(&s.wg, 1*time.Second)
	Running = false
}
func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false
	case <-time.After(timeout):
		return true
	}
}
func (a *App) SendToLabelary(zpl string, width string, height string) error {

	if zpl == "" {
		return nil
	}
	res, err := a.CallLabelary(zpl, 0, int(a.Settings.PrintWidth), int(a.Settings.PrintHeight))
	if err != nil {
		fmt.Println("Error calling Labelary:", err)
		return nil
	}

	defer res.Body.Close()

	var imageBytes [][]byte
	imageByte, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	if strings.Contains(string(imageByte), "ERROR: Requested 1st label but ZPL generated no labels") {
		return nil
	}
	imageBytes = append(imageBytes, imageByte)
	countOfLabel := res.Header.Get("x-total-count")
	if countOfLabel != "" && countOfLabel != "0" && countOfLabel != "1" {
		labelCounts, err := strconv.Atoi(countOfLabel)
		if err != nil {
			fmt.Println("Error converting label count:", err)
			return nil
		}
		for i := 1; i < labelCounts; i++ {
			time.Sleep(250 * time.Millisecond)
			res, err := a.CallLabelary(zpl, i, int(a.Settings.PrintWidth), int(a.Settings.PrintHeight))
			if err != nil {
				fmt.Println("Error calling Labelary:", err)
				return nil
			}
			defer res.Body.Close()
			imageByte, err := io.ReadAll(res.Body)
			if err != nil {
				panic(err)
			}
			if strings.Contains(string(imageByte), "ERROR: Requested 1st label but ZPL generated no labels") {
				return nil
			}
			imageBytes = append(imageBytes, imageByte)
		}
	}
	for _, v := range imageBytes {
		base64String := base64.StdEncoding.EncodeToString(v)

		runtime.EventsEmit(a.ctx, "NewPrint", base64String)
		if SaveToFile {
			fname := fmt.Sprintf("%s\\label-print-%d_%d_%d-%d-%d-%d-%d.png", a.Settings.PrintPath, time.Now().Month(), time.Now().Day(), time.Now().Year(), time.Now().Hour(), time.Now().Minute(), time.Now().Second(), time.Now().Nanosecond())

			f, err := os.Create(fname)
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = f.Write(imageByte)

			if err != nil {
				return err
			}
			fmt.Printf("%s created!", fname)
		}
	}

	return nil
}
func (a *App) CallLabelary(zpl string, printNumber int, width int, height int) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.TODO(), "POST", fmt.Sprintf("http://api.labelary.com/v1/printers/%ddpmm/labels/%dx%d/%d/", a.Settings.PrinterDPI.Dpi, width, height, printNumber), strings.NewReader(zpl))
	if err != nil {
		fmt.Printf("client: could not create request: %s\n", err)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "image/png")
	req.Header.Set("X-Rotation", strconv.Itoa(int(a.Settings.PrintRotation)))
	client := http.Client{
		Timeout: 30 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// SendPDFToIPPPrinter sends a PDF byte array to an IPP printer at the given IP address and port

// PrintPDFBytesToLocalPrinter prints a PDF byte array to a specified local printer using Windows built-in tools (no external dependencies)
func PrintPNGBytesToLocalPrinter(pngBytes []byte, printerName string) error {
	tmpFile, err := os.CreateTemp("", "temp-*.png")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpFilePath := tmpFile.Name()
	defer os.Remove(tmpFilePath)

	_, err = tmpFile.Write(pngBytes)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write PDF bytes to temp file: %w", err)
	}
	tmpFile.Close()

	cmd := exec.Command("mspaint.exe", "/pt", tmpFilePath, printerName)

	err = cmd.Run()
	if err != nil {
		fmt.Printf("Attempted to print PNG using mspaint. Error (if any): %v\n", err)
		if exitError, ok := err.(*exec.ExitError); ok {
			fmt.Printf("mspaint stderr: %s\n", string(exitError.Stderr))
		}
		return err
	}
	fmt.Println("PNG sent to printer via mspaint.")
	return nil
}

// ProcessAndSendToPrinter processes a print job for a given printer type and destination
func (a *App) ProcessAndSendToPrinter(printerType, ipAddress string, port int, zpl string) error {
	if printerType == "Zebra" {
		// Forward the string to port 9100 (raw socket)
		if port == 0 {
			port = 9100
		}
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ipAddress, fmt.Sprintf("%d", port)), 10*time.Second)
		if err != nil {
			return fmt.Errorf("failed to connect to Zebra printer: %w", err)
		}
		defer conn.Close()
		_, err = conn.Write([]byte(zpl))
		if err != nil {
			return fmt.Errorf("failed to send data to Zebra printer: %w", err)
		}
		return nil
	}
	if printerType == "IPP" {
		// Convert ZPL to PNG (using Labelary)
		if zpl == "" {
			return nil
		}
		res, err := a.CallLabelary(zpl, 0, int(a.Settings.PrintWidth), int(a.Settings.PrintHeight))
		if err != nil {
			fmt.Println("Error calling Labelary:", err)
			return nil
		}

		defer res.Body.Close()

		var imageBytes [][]byte
		imageByte, err := io.ReadAll(res.Body)
		if err != nil {
			panic(err)
		}
		if strings.Contains(string(imageByte), "ERROR: Requested 1st label but ZPL generated no labels") {
			return nil
		}
		imageBytes = append(imageBytes, imageByte)
		countOfLabel := res.Header.Get("x-total-count")
		if countOfLabel != "" && countOfLabel != "0" && countOfLabel != "1" {
			labelCounts, err := strconv.Atoi(countOfLabel)
			if err != nil {
				fmt.Println("Error converting label count:", err)
				return nil
			}
			for i := 1; i < labelCounts; i++ {
				time.Sleep(250 * time.Millisecond)
				res, err := a.CallLabelary(zpl, i, int(a.Settings.PrintWidth), int(a.Settings.PrintHeight))
				if err != nil {
					fmt.Println("Error calling Labelary:", err)
					return nil
				}
				defer res.Body.Close()
				imageByte, err := io.ReadAll(res.Body)
				if err != nil {
					panic(err)
				}
				if strings.Contains(string(imageByte), "ERROR: Requested 1st label but ZPL generated no labels") {
					return nil
				}
				imageBytes = append(imageBytes, imageByte)
			}
		}
		// Convert PNG to PDF
		for _, bytes := range imageBytes {
			PrintPNGBytesToLocalPrinter(bytes, "Brother HL-L3290CDW series")
		}

	}
	return fmt.Errorf("unsupported printer type: %s", printerType)
}

// Handles incoming requests.
func (a *App) handleRequest(conn net.Conn, width string, height string) {

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
	switch PrintMode {
	case 0:
		err := a.SendToLabelary(messageString, width, height)
		if err != nil {
			fmt.Println(err)
		}
	case 1:
		//ZPL to network Printer
		a.ProcessAndSendToPrinter(SelectedPrinter.PrinterType, SelectedPrinter.IPAddress, SelectedPrinter.PrinterPort, messageString)
		return
	case 2:
		//Printer Relay
		a.ProcessRelayGroup(messageString)
		return
	default:
		return
	}
}

func (a *App) ProcessRelayGroup(zpl string) {
	for _, printerID := range LabelRelayGroup.PrinterIDs {
		printer, err := GetPrinterByID(a.db, printerID)
		if err != nil {
			fmt.Println("Error getting printer by ID:", err)
			continue
		}
		a.ProcessAndSendToPrinter(printer.PrinterType, printer.IPAddress, printer.PrinterPort, zpl)

	}
}

// QueryInstalledPrinters returns a slice of printer names installed on the local Windows machine
func QueryInstalledPrinters() ([]string, error) {
	// This function uses PowerShell to get the list of printers
	cmd := exec.Command("powershell", "-Command", "Get-Printer | Select-Object -ExpandProperty Name")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	var printers []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			printers = append(printers, trimmed)
		}
	}
	return printers, nil
}
