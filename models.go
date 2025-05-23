package main

import (
	"database/sql"
	"encoding/json"
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

// RelayGroup represents a group of printer IDs
// e.g. [1,2,3]
type RelayGroup struct {
	GroupID    int   `json:"groupID"`
	PrinterIDs []int `json:"printerIDs"`
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

// Printer CRUD and table initialization
func InitPrintersTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS printers (
			printerID INTEGER PRIMARY KEY AUTOINCREMENT,
			printerName TEXT NOT NULL,
			ipAddress TEXT NOT NULL,
			printerPort INTEGER NOT NULL,
			printerType TEXT NOT NULL
		)`)
	if err != nil {
		println("Error initializing printers table:", err.Error())
	}
	return err
}

func AddPrinter(db *sql.DB, p *Printer) error {
	res, err := db.Exec(`
		INSERT INTO printers (printerName, ipAddress, printerPort, printerType)
		VALUES (?, ?, ?, ?)
	`, p.PrinterName, p.IPAddress, p.PrinterPort, p.PrinterType)
	if err != nil {
		println("Error adding printer:", err.Error())
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		p.PrinterID = int(id)
	}
	return err
}

func GetPrinters(db *sql.DB) ([]Printer, error) {
	rows, err := db.Query(`SELECT printerID, printerName, ipAddress, printerPort, printerType FROM printers`)
	if err != nil {
		println("Error getting printers:", err.Error())
		return nil, err
	}
	defer rows.Close()
	var printers []Printer
	for rows.Next() {
		var p Printer
		err := rows.Scan(&p.PrinterID, &p.PrinterName, &p.IPAddress, &p.PrinterPort, &p.PrinterType)
		if err != nil {
			println("Error scanning printer row:", err.Error())
			continue
		}
		printers = append(printers, p)
	}
	return printers, nil
}

func UpdatePrinter(db *sql.DB, p *Printer) error {
	_, err := db.Exec(`
		UPDATE printers SET printerName=?, ipAddress=?, printerPort=?, printerType=? WHERE printerID=?
	`, p.PrinterName, p.IPAddress, p.PrinterPort, p.PrinterType, p.PrinterID)
	if err != nil {
		println("Error updating printer:", err.Error())
	}
	return err
}

func DeletePrinter(db *sql.DB, printerID int) error {
	_, err := db.Exec(`DELETE FROM printers WHERE printerID=?`, printerID)
	if err != nil {
		println("Error deleting printer:", err.Error())
	}
	return err
}

// Initialize relay_groups table
func InitRelayGroupsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS relay_groups (
			groupID INTEGER PRIMARY KEY AUTOINCREMENT,
			printerIDs TEXT NOT NULL -- JSON array of printer IDs
		)`)
	if err != nil {
		println("Error initializing relay_groups table:", err.Error())
	}
	return err
}

// Add a relay group
func AddRelayGroup(db *sql.DB, printerIDs []int) error {
	idsJSON, err := json.Marshal(printerIDs)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO relay_groups (printerIDs) VALUES (?)`, string(idsJSON))
	if err != nil {
		println("Error adding relay group:", err.Error())
	}
	return err
}

// Get all relay groups
func GetRelayGroups(db *sql.DB) ([]RelayGroup, error) {
	rows, err := db.Query(`SELECT groupID, printerIDs FROM relay_groups`)
	if err != nil {
		println("Error getting relay groups:", err.Error())
		return nil, err
	}
	defer rows.Close()
	var groups []RelayGroup
	for rows.Next() {
		var g RelayGroup
		var idsJSON string
		err := rows.Scan(&g.GroupID, &idsJSON)
		if err != nil {
			println("Error scanning relay group row:", err.Error())
			continue
		}
		err = json.Unmarshal([]byte(idsJSON), &g.PrinterIDs)
		if err != nil {
			println("Error unmarshaling printerIDs:", err.Error())
			continue
		}
		groups = append(groups, g)
	}
	return groups, nil
}

// Delete a relay group by groupID
func DeleteRelayGroup(db *sql.DB, groupID int) error {
	_, err := db.Exec(`DELETE FROM relay_groups WHERE groupID=?`, groupID)
	if err != nil {
		println("Error deleting relay group:", err.Error())
	}
	return err
}

// GetPrinterByID looks up a printer by its printerID
func GetPrinterByID(db *sql.DB, printerID int) (*Printer, error) {
	row := db.QueryRow(`SELECT printerID, printerName, ipAddress, printerPort, printerType FROM printers WHERE printerID = ?`, printerID)
	var p Printer
	err := row.Scan(&p.PrinterID, &p.PrinterName, &p.IPAddress, &p.PrinterPort, &p.PrinterType)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, err
	}
	return &p, nil
}
