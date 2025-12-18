package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image/png"
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

	"github.com/jung-kurt/gofpdf"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// IPP constants for direct communication
const (
	ippOperationPrintJob = 0x0002
	ippTagOperation      = 0x01
	ippTagJob            = 0x02
	ippTagEnd            = 0x03
	ippTagCharset        = 0x47
	ippTagLanguage       = 0x48
	ippTagUri            = 0x45
	ippTagName           = 0x42
	ippTagMimeType       = 0x49
	ippTagInteger        = 0x21
	ippContentTypeIPP    = "application/ipp"
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
			go func(c net.Conn) {
				defer a.tcp.wg.Done()
				a.handleRequest(c, strconv.Itoa(int(a.Settings.PrintWidth)), strconv.Itoa(int(a.Settings.PrintHeight)))
			}(conn)
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

			_, err = f.Write(v) // Fixed: was writing wrong variable (imageByte instead of v)

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

// writeIPPAttribute writes a string attribute to the buffer
func writeIPPAttribute(buf *bytes.Buffer, tag int8, name string, value []byte) {
	binary.Write(buf, binary.BigEndian, tag)
	binary.Write(buf, binary.BigEndian, int16(len(name)))
	buf.WriteString(name)
	binary.Write(buf, binary.BigEndian, int16(len(value)))
	buf.Write(value)
}

// writeIPPIntAttribute writes an integer attribute to the buffer
func writeIPPIntAttribute(buf *bytes.Buffer, tag int8, name string, value int32) {
	binary.Write(buf, binary.BigEndian, tag)
	binary.Write(buf, binary.BigEndian, int16(len(name)))
	buf.WriteString(name)
	binary.Write(buf, binary.BigEndian, int16(4)) // integer is always 4 bytes
	binary.Write(buf, binary.BigEndian, value)
}

// sendDirectIPPPrintJob sends a raw IPP Print-Job request with PDF data
func sendDirectIPPPrintJob(host string, port int, endpoint string, useTLS bool, pdfData []byte, documentName string) (int, error) {
	proto := "http"
	if useTLS {
		proto = "https"
	}
	url := fmt.Sprintf("%s://%s:%d%s", proto, host, port, endpoint)
	printerURI := fmt.Sprintf("ipp://%s:%d%s", host, port, endpoint)

	// Build IPP Print-Job request
	buf := new(bytes.Buffer)

	// Version 2.0
	binary.Write(buf, binary.BigEndian, int8(2)) // major
	binary.Write(buf, binary.BigEndian, int8(0)) // minor
	// Operation
	binary.Write(buf, binary.BigEndian, int16(ippOperationPrintJob))
	// Request ID
	binary.Write(buf, binary.BigEndian, int32(1))

	// Operation attributes
	binary.Write(buf, binary.BigEndian, int8(ippTagOperation))

	// attributes-charset
	writeIPPAttribute(buf, ippTagCharset, "attributes-charset", []byte("utf-8"))
	// attributes-natural-language
	writeIPPAttribute(buf, ippTagLanguage, "attributes-natural-language", []byte("en-us"))
	// printer-uri
	writeIPPAttribute(buf, ippTagUri, "printer-uri", []byte(printerURI))
	// requesting-user-name
	writeIPPAttribute(buf, ippTagName, "requesting-user-name", []byte("ZPLPrinterEmulator"))
	// job-name
	writeIPPAttribute(buf, ippTagName, "job-name", []byte(documentName))
	// document-format
	writeIPPAttribute(buf, ippTagMimeType, "document-format", []byte("application/pdf"))

	// Job attributes section
	binary.Write(buf, binary.BigEndian, int8(ippTagJob))
	// copies (integer)
	writeIPPIntAttribute(buf, ippTagInteger, "copies", 1)

	// End tag
	binary.Write(buf, binary.BigEndian, int8(ippTagEnd))

	// Combine IPP request with PDF data
	fullRequest := append(buf.Bytes(), pdfData...)

	// Send HTTP request
	client := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(fullRequest))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", ippContentTypeIPP)
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(fullRequest)))

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if len(body) < 8 {
		return 0, fmt.Errorf("response too short: %d bytes", len(body))
	}

	// Check status code
	statusCode := int16(body[2])<<8 | int16(body[3])
	if statusCode != 0 {
		return 0, fmt.Errorf("IPP error: status code 0x%04x", statusCode)
	}

	// Job submitted successfully
	fmt.Printf("IPP print job submitted successfully to %s\n", host)
	return 0, nil
}

// convertPNGToPDF converts PNG image bytes to PDF bytes
func convertPNGToPDF(pngBytes []byte, widthInches, heightInches float64) ([]byte, error) {
	// Decode PNG
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to decode PNG: %w", err)
	}

	// Create PDF with custom page size matching label dimensions
	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		UnitStr: "in",
		Size:    gofpdf.SizeType{Wd: widthInches, Ht: heightInches},
	})
	pdf.SetMargins(0, 0, 0)
	pdf.AddPage()

	// Register and add image
	imgName := "label"
	pdf.RegisterImageOptionsReader(imgName, gofpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(pngBytes))

	// Calculate image dimensions to fit the page
	bounds := img.Bounds()
	imgWidth := float64(bounds.Dx())
	imgHeight := float64(bounds.Dy())

	// Scale to fit page while maintaining aspect ratio
	scaleX := widthInches / (imgWidth / 72.0)  // Assume 72 DPI for calculation
	scaleY := heightInches / (imgHeight / 72.0)
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}

	// Center the image on the page
	finalWidth := (imgWidth / 72.0) * scale
	finalHeight := (imgHeight / 72.0) * scale
	x := (widthInches - finalWidth) / 2
	y := (heightInches - finalHeight) / 2

	pdf.ImageOptions(imgName, x, y, finalWidth, finalHeight, false, gofpdf.ImageOptions{}, 0, "")

	// Write to buffer
	var buf bytes.Buffer
	err = pdf.Output(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	return buf.Bytes(), nil
}

// sendPNGDirectlyToIPP tries to send PNG directly via IPP (fallback if printer supports it)
func sendPNGDirectlyToIPP(host string, port int, endpoint string, useTLS bool, pngData []byte, documentName string) error {
	proto := "http"
	if useTLS {
		proto = "https"
	}
	url := fmt.Sprintf("%s://%s:%d%s", proto, host, port, endpoint)
	printerURI := fmt.Sprintf("ipp://%s:%d%s", host, port, endpoint)

	// Build IPP Print-Job request for PNG
	buf := new(bytes.Buffer)

	// Version 2.0
	binary.Write(buf, binary.BigEndian, int8(2))
	binary.Write(buf, binary.BigEndian, int8(0))
	binary.Write(buf, binary.BigEndian, int16(ippOperationPrintJob))
	binary.Write(buf, binary.BigEndian, int32(1))

	binary.Write(buf, binary.BigEndian, int8(ippTagOperation))
	writeIPPAttribute(buf, ippTagCharset, "attributes-charset", []byte("utf-8"))
	writeIPPAttribute(buf, ippTagLanguage, "attributes-natural-language", []byte("en-us"))
	writeIPPAttribute(buf, ippTagUri, "printer-uri", []byte(printerURI))
	writeIPPAttribute(buf, ippTagName, "requesting-user-name", []byte("ZPLPrinterEmulator"))
	writeIPPAttribute(buf, ippTagName, "job-name", []byte(documentName))
	writeIPPAttribute(buf, ippTagMimeType, "document-format", []byte("image/png"))

	binary.Write(buf, binary.BigEndian, int8(ippTagJob))
	writeIPPIntAttribute(buf, ippTagInteger, "copies", 1)
	binary.Write(buf, binary.BigEndian, int8(ippTagEnd))

	fullRequest := append(buf.Bytes(), pngData...)

	client := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(fullRequest))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", ippContentTypeIPP)
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(fullRequest)))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if len(body) < 8 {
		return fmt.Errorf("response too short: %d bytes", len(body))
	}

	statusCode := int16(body[2])<<8 | int16(body[3])
	if statusCode != 0 {
		return fmt.Errorf("IPP error: status code 0x%04x", statusCode)
	}

	return nil
}

// PrintPNGBytesToLocalPrinter prints a PNG byte array to a specified local printer using Windows built-in tools (no external dependencies)
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
	return a.ProcessAndSendToPrinterWithIPP(printerType, ipAddress, port, zpl, "/ipp/print", false)
}

// ProcessAndSendToPrinterWithIPP processes a print job with full IPP support
func (a *App) ProcessAndSendToPrinterWithIPP(printerType, ipAddress string, port int, zpl string, ippEndpoint string, useTLS bool) error {
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

		// Set defaults
		if port == 0 {
			port = 631
		}
		if ippEndpoint == "" {
			ippEndpoint = "/ipp/print"
		}

		res, err := a.CallLabelary(zpl, 0, int(a.Settings.PrintWidth), int(a.Settings.PrintHeight))
		if err != nil {
			fmt.Println("Error calling Labelary:", err)
			return err
		}
		defer res.Body.Close()

		var imageBytes [][]byte
		imageByte, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("failed to read Labelary response: %w", err)
		}
		if strings.Contains(string(imageByte), "ERROR: Requested 1st label but ZPL generated no labels") {
			return nil
		}
		imageBytes = append(imageBytes, imageByte)

		// Handle multi-label ZPL
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
					continue
				}
				defer res.Body.Close()
				imageByte, err := io.ReadAll(res.Body)
				if err != nil {
					continue
				}
				if strings.Contains(string(imageByte), "ERROR: Requested 1st label but ZPL generated no labels") {
					continue
				}
				imageBytes = append(imageBytes, imageByte)
			}
		}

		// Send each label to the IPP printer
		for i, pngBytes := range imageBytes {
			documentName := fmt.Sprintf("ZPL-Label-%d-%d", time.Now().Unix(), i)

			// First try: Convert PNG to PDF and send via IPP
			pdfBytes, err := convertPNGToPDF(pngBytes, a.Settings.PrintWidth, a.Settings.PrintHeight)
			if err != nil {
				fmt.Printf("Failed to convert PNG to PDF: %v, trying PNG fallback\n", err)
				// Fallback: Try sending PNG directly
				err = sendPNGDirectlyToIPP(ipAddress, port, ippEndpoint, useTLS, pngBytes, documentName)
				if err != nil {
					fmt.Printf("IPP PNG fallback also failed: %v\n", err)
					return err
				}
				continue
			}

			// Send PDF via IPP
			_, err = sendDirectIPPPrintJob(ipAddress, port, ippEndpoint, useTLS, pdfBytes, documentName)
			if err != nil {
				fmt.Printf("IPP PDF print failed: %v, trying PNG fallback\n", err)
				// Fallback: Try sending PNG directly
				err = sendPNGDirectlyToIPP(ipAddress, port, ippEndpoint, useTLS, pngBytes, documentName)
				if err != nil {
					fmt.Printf("IPP PNG fallback also failed: %v\n", err)
					return err
				}
			}
		}
		return nil
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
		a.ProcessAndSendToPrinterWithIPP(SelectedPrinter.PrinterType, SelectedPrinter.IPAddress, SelectedPrinter.PrinterPort, messageString, SelectedPrinter.IPPEndpoint, SelectedPrinter.UseTLS)
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
		a.ProcessAndSendToPrinterWithIPP(printer.PrinterType, printer.IPAddress, printer.PrinterPort, zpl, printer.IPPEndpoint, printer.UseTLS)
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
