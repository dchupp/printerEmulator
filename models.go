package main

import (
	"database/sql"
)

type Settings struct {
	SettingID      int        `json:"settingID"`
	PrintWidth     float64    `json:"printWidth"`
	PrintHeight    float64    `json:"printHeight"`
	PrintRotation  float64    `json:"printRotation"`
	PrinterPort    float64    `json:"printerPort"`
	PrintPath      string     `json:"printerPath"`
	PrinterDPI     PrinterDPI `json:"printerDPI"`
	DefaultPrinter int        `json:"defaultPrinter"`
}

type Printer struct {
	PrinterID   int    `json:"printerID"`
	PrinterName string `json:"printerName"`
	IPAddress   string `json:"ipAddress"`
	PrinterPort int    `json:"printerPort"`
	PrinterType string `json:"printerType"`
}

// SettingsDB provides methods to interact with the settings table
// Only one row should ever exist in the settings table
func (s *Settings) SaveToDB(db *sql.DB) error {
	_, err := db.Exec(`
		INSERT INTO settings (
			settingID, printWidth, printHeight, printRotation, printerPort, printPath, printerDPI_value, printerDPI_desc, defaultPrinter
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(settingID) DO UPDATE SET
			printWidth=excluded.printWidth,
			printHeight=excluded.printHeight,
			printRotation=excluded.printRotation,
			printerPort=excluded.printerPort,
			printPath=excluded.printPath,
			printerDPI_value=excluded.printerDPI_value,
			printerDPI_desc=excluded.printerDPI_desc,
			defaultPrinter=excluded.defaultPrinter
	`,
		s.SettingID,
		s.PrintWidth,
		s.PrintHeight,
		s.PrintRotation,
		s.PrinterPort,
		s.PrintPath,
		s.PrinterDPI.Dpi,
		s.PrinterDPI.Description,
		s.DefaultPrinter,
	)
	if err != nil {
		println("Error saving settings to DB:", err.Error())
	}
	return err
}

func LoadSettingsFromDB(db *sql.DB) (*Settings, error) {
	row := db.QueryRow(`SELECT settingID, printWidth, printHeight, printRotation, printerPort, printPath, printerDPI_value, printerDPI_desc, defaultPrinter FROM settings LIMIT 1`)
	var s Settings
	var dpiValue int
	var dpiDesc string
	err := row.Scan(&s.SettingID, &s.PrintWidth, &s.PrintHeight, &s.PrintRotation, &s.PrinterPort, &s.PrintPath, &dpiValue, &dpiDesc, &s.DefaultPrinter)
	if err != nil {
		println("Error loading settings from DB:", err.Error())
		return nil, err
	}
	s.PrinterDPI = PrinterDPI{Dpi: dpiValue, Description: dpiDesc}
	return &s, nil
}

func InitSettingsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS settings (
			settingID INTEGER PRIMARY KEY,
			printWidth REAL,
			printHeight REAL,
			printRotation REAL,
			printerPort REAL,
			printPath TEXT,
			printerDPI_value INTEGER,
			printerDPI_desc TEXT,
			defaultPrinter INTEGER
		)
	`)
	if err != nil {
		println("Error initializing settings table:", err.Error())
	}
	return err
}
