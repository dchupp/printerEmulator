package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Store persists settings, network printers and relay groups in SQLite.
//
// It reuses the database file and tables of earlier versions, so printers,
// relay groups and settings survive the upgrade.
type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	// WAL keeps writes from blocking reads; busy_timeout avoids SQLITE_BUSY
	// when the UI and a print job touch the database at the same time.
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS app_settings (id INTEGER PRIMARY KEY CHECK (id = 1), data TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS printers (
			printerID INTEGER PRIMARY KEY AUTOINCREMENT,
			printerName TEXT NOT NULL,
			ipAddress TEXT NOT NULL,
			printerPort INTEGER NOT NULL,
			printerType TEXT NOT NULL,
			ippEndpoint TEXT DEFAULT '/ipp/print',
			useTLS INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS relay_groups (
			groupID INTEGER PRIMARY KEY AUTOINCREMENT,
			printerIDs TEXT NOT NULL
		)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	// Columns added after the first release. SQLite has no ADD COLUMN IF NOT
	// EXISTS, so a "duplicate column" error means the column is already there.
	for _, q := range []string{
		`ALTER TABLE printers ADD COLUMN ippEndpoint TEXT DEFAULT '/ipp/print'`,
		`ALTER TABLE printers ADD COLUMN useTLS INTEGER DEFAULT 0`,
		`ALTER TABLE relay_groups ADD COLUMN name TEXT DEFAULT ''`,
	} {
		if _, err := s.db.Exec(q); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// LoadSettings returns the saved settings, importing them from the 2.x
// settings table the first time, or the defaults.
func (s *Store) LoadSettings() Settings {
	var data string
	if err := s.db.QueryRow(`SELECT data FROM app_settings WHERE id = 1`).Scan(&data); err == nil {
		st := DefaultSettings()
		if json.Unmarshal([]byte(data), &st) == nil {
			return st.Normalize()
		}
	}
	st := s.importLegacySettings()
	_ = s.SaveSettings(st)
	return st
}

func (s *Store) importLegacySettings() Settings {
	st := DefaultSettings()
	var w, h, rot, port float64
	var path string
	var dpmm, auto int
	err := s.db.QueryRow(`SELECT printWidth, printHeight, printRotation, printerPort, COALESCE(printPath, ''),
		printerDPI_value, COALESCE(autoStartServer, 0) FROM settings LIMIT 1`).Scan(&w, &h, &rot, &port, &path, &dpmm, &auto)
	if err != nil {
		return st
	}
	st.LabelWidth, st.LabelHeight = w, h
	st.Rotation = int(rot)
	st.Port = int(port)
	st.SaveDir = path
	st.DPMM = dpmm
	st.AutoStartServer = auto != 0
	return st.Normalize()
}

func (s *Store) SaveSettings(st Settings) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO app_settings (id, data) VALUES (1, ?)
		ON CONFLICT(id) DO UPDATE SET data = excluded.data`, string(b))
	return err
}

const printerColumns = `printerID, printerName, ipAddress, printerPort, printerType,
	COALESCE(ippEndpoint, '/ipp/print'), COALESCE(useTLS, 0)`

func scanPrinter(row interface{ Scan(...any) error }) (Printer, error) {
	var p Printer
	var tls int
	err := row.Scan(&p.PrinterID, &p.PrinterName, &p.IPAddress, &p.PrinterPort, &p.PrinterType, &p.IPPEndpoint, &tls)
	p.UseTLS = tls != 0
	return p, err
}

func (s *Store) Printers() ([]Printer, error) {
	rows, err := s.db.Query(`SELECT ` + printerColumns + ` FROM printers ORDER BY printerName COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Printer{}
	for rows.Next() {
		p, err := scanPrinter(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

var ErrNotFound = errors.New("not found")

func (s *Store) Printer(id int) (Printer, error) {
	p, err := scanPrinter(s.db.QueryRow(`SELECT `+printerColumns+` FROM printers WHERE printerID = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return p, fmt.Errorf("printer %d: %w", id, ErrNotFound)
	}
	return p, err
}

func (s *Store) SavePrinter(p Printer) (Printer, error) {
	p, err := p.Normalize()
	if err != nil {
		return p, err
	}
	tls := 0
	if p.UseTLS {
		tls = 1
	}
	if p.PrinterID == 0 {
		res, err := s.db.Exec(`INSERT INTO printers (printerName, ipAddress, printerPort, printerType, ippEndpoint, useTLS)
			VALUES (?, ?, ?, ?, ?, ?)`, p.PrinterName, p.IPAddress, p.PrinterPort, p.PrinterType, p.IPPEndpoint, tls)
		if err != nil {
			return p, err
		}
		id, err := res.LastInsertId()
		p.PrinterID = int(id)
		return p, err
	}
	_, err = s.db.Exec(`UPDATE printers SET printerName=?, ipAddress=?, printerPort=?, printerType=?, ippEndpoint=?, useTLS=?
		WHERE printerID=?`, p.PrinterName, p.IPAddress, p.PrinterPort, p.PrinterType, p.IPPEndpoint, tls, p.PrinterID)
	return p, err
}

// DeletePrinter removes a printer and drops it from every relay group.
func (s *Store) DeletePrinter(id int) error {
	groups, err := s.RelayGroups()
	if err != nil {
		return err
	}
	for _, g := range groups {
		kept := g.PrinterIDs[:0]
		for _, pid := range g.PrinterIDs {
			if pid != id {
				kept = append(kept, pid)
			}
		}
		if len(kept) != len(g.PrinterIDs) {
			g.PrinterIDs = kept
			if _, err := s.SaveRelayGroup(g); err != nil {
				return err
			}
		}
	}
	_, err = s.db.Exec(`DELETE FROM printers WHERE printerID = ?`, id)
	return err
}

func scanGroup(row interface{ Scan(...any) error }) (RelayGroup, error) {
	var g RelayGroup
	var ids string
	if err := row.Scan(&g.GroupID, &g.Name, &ids); err != nil {
		return g, err
	}
	if err := json.Unmarshal([]byte(ids), &g.PrinterIDs); err != nil {
		g.PrinterIDs = []int{}
	}
	if g.Name == "" {
		g.Name = fmt.Sprintf("Group %d", g.GroupID)
	}
	return g, nil
}

func (s *Store) RelayGroups() ([]RelayGroup, error) {
	rows, err := s.db.Query(`SELECT groupID, COALESCE(name, ''), printerIDs FROM relay_groups ORDER BY groupID`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RelayGroup{}
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) RelayGroup(id int) (RelayGroup, error) {
	g, err := scanGroup(s.db.QueryRow(`SELECT groupID, COALESCE(name, ''), printerIDs FROM relay_groups WHERE groupID = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return g, fmt.Errorf("relay group %d: %w", id, ErrNotFound)
	}
	return g, err
}

func (s *Store) SaveRelayGroup(g RelayGroup) (RelayGroup, error) {
	g.Name = strings.TrimSpace(g.Name)
	if g.PrinterIDs == nil {
		g.PrinterIDs = []int{}
	}
	ids, err := json.Marshal(g.PrinterIDs)
	if err != nil {
		return g, err
	}
	if g.GroupID == 0 {
		res, err := s.db.Exec(`INSERT INTO relay_groups (name, printerIDs) VALUES (?, ?)`, g.Name, string(ids))
		if err != nil {
			return g, err
		}
		id, err := res.LastInsertId()
		g.GroupID = int(id)
		if g.Name == "" {
			g.Name = fmt.Sprintf("Group %d", g.GroupID)
		}
		return g, err
	}
	_, err = s.db.Exec(`UPDATE relay_groups SET name = ?, printerIDs = ? WHERE groupID = ?`, g.Name, string(ids), g.GroupID)
	return g, err
}

func (s *Store) DeleteRelayGroup(id int) error {
	_, err := s.db.Exec(`DELETE FROM relay_groups WHERE groupID = ?`, id)
	return err
}
