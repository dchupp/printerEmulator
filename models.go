package main

type Settings struct {
	SettingID      int     `json:"settingID"`
	PrintWidth     float64 `json:"printWidth"`
	PrintHeight    float64 `json:"printHeight"`
	PrintRotation  float64 `json:"printRotation"`
	PrinterPort    float64 `json:"printerPort"`
	DefaultPrinter int     `json:"defaultPrinter"`
}

type Printer struct {
	PrinterID   int    `json:"printerID"`
	PrinterName string `json:"printerName"`
	IPAddress   string `json:"ipAddress"`
	PrinterPort int    `json:"printerPort"`
	PrinterType string `json:"printerType"`
}
