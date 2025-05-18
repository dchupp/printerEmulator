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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	CONN_HOST     = "127.0.0.1"
	CONN_PORT     = 9100
	CONN_TYPE     = "tcp"
	PrintWidth    = 4
	PrintHeight   = 6
	StopRun       = false
	Running       = false
	SaveToFile    = false
	FilePath      = ""
	DPI           PrinterDPI
	PrintRotation = 0
)

type TCPServer struct {
	listener net.Listener
	quit     chan interface{}
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
	addressString := fmt.Sprintf("%s:%d", CONN_HOST, CONN_PORT)

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
				go a.handleRequest(conn, strconv.Itoa(PrintWidth), strconv.Itoa(PrintHeight))
				a.tcp.wg.Done()
			}()
		}
	}
}
func (s *TCPServer) GetStatus() bool {
	addressString := net.JoinHostPort(CONN_HOST, fmt.Sprintf("%d", CONN_PORT))
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
	res, err := a.CallLabelary(zpl, 0, PrintWidth, PrintHeight)
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
			res, err := a.CallLabelary(zpl, i, PrintWidth, PrintHeight)
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
			fname := fmt.Sprintf("%s\\label-print-%d_%d_%d-%d-%d-%d-%d.png", FilePath, time.Now().Month(), time.Now().Day(), time.Now().Year(), time.Now().Hour(), time.Now().Minute(), time.Now().Second(), time.Now().Nanosecond())

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
	req, err := http.NewRequestWithContext(context.TODO(), "POST", fmt.Sprintf("http://api.labelary.com/v1/printers/%ddpmm/labels/%dx%d/%d/", DPI.Dpi, width, height, printNumber), strings.NewReader(zpl))
	if err != nil {
		fmt.Printf("client: could not create request: %s\n", err)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "image/png")
	req.Header.Set("X-Rotation", strconv.Itoa(PrintRotation))
	client := http.Client{
		Timeout: 30 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
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
	err := a.SendToLabelary(messageString, width, height)
	if err != nil {
		fmt.Println(err)
	}
}
