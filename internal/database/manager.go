package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type DatabaseInfo struct {
	Name      string    `json:"name"`
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	UpdatedAt time.Time `json:"updated_at"`
	IsActive  bool      `json:"is_active"`
}

type Manager struct {
	dataDir    string
	activeName string
	db         *sql.DB
	mu         sync.RWMutex
}

// NewManager creates a manager for SQLite database files
func NewManager(dataDir string) (*Manager, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	mgr := &Manager{
		dataDir: dataDir,
	}

	// Switch to default or first available DB
	dbFiles, _ := mgr.ListDatabases()
	targetName := "default"
	if len(dbFiles) > 0 {
		targetName = dbFiles[0].Name
	}

	if err := mgr.SwitchDatabase(targetName); err != nil {
		return nil, fmt.Errorf("failed to initialize active database: %w", err)
	}

	return mgr, nil
}

// GetDB returns current active database and its name
func (m *Manager) GetDB() (*sql.DB, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.db, m.activeName
}

// ListDatabases lists all available sqlite databases in the data directory
func (m *Manager) ListDatabases() ([]DatabaseInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries, err := os.ReadDir(m.dataDir)
	if err != nil {
		return nil, err
	}

	var dbs []DatabaseInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".db")
		dbs = append(dbs, DatabaseInfo{
			Name:      name,
			Filename:  entry.Name(),
			Size:      info.Size(),
			UpdatedAt: info.ModTime(),
			IsActive:  name == m.activeName,
		})
	}
	return dbs, nil
}

// sanitizeName ensures database name is safe
func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")
	name = strings.ReplaceAll(name, "..", "")
	return strings.ToLower(name)
}

// CreateDatabase creates a new sqlite database file and switches to it
func (m *Manager) CreateDatabase(name string) error {
	name = sanitizeName(name)
	if name == "" {
		return fmt.Errorf("database name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return m.switchDatabaseLocked(name)
}

// SwitchDatabase switches to an existing or new database
func (m *Manager) SwitchDatabase(name string) error {
	name = sanitizeName(name)
	if name == "" {
		return fmt.Errorf("database name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return m.switchDatabaseLocked(name)
}

func (m *Manager) switchDatabaseLocked(name string) error {
	if m.db != nil {
		_ = m.db.Close()
	}

	filePath := filepath.Join(m.dataDir, name+".db")
	dsn := fmt.Sprintf("%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", filePath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		_ = db.Close()
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	m.db = db
	m.activeName = name
	return nil
}

// DeleteDatabase deletes a database file
func (m *Manager) DeleteDatabase(name string) error {
	name = sanitizeName(name)
	if name == "" {
		return fmt.Errorf("database name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeName == name {
		_ = m.db.Close()
		m.db = nil
		m.activeName = ""
	}

	filePath := filepath.Join(m.dataDir, name+".db")
	_ = os.Remove(filePath + "-wal")
	_ = os.Remove(filePath + "-shm")
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove database file: %w", err)
	}

	// Switch to default or another DB if active was deleted
	if m.db == nil {
		return m.switchDatabaseLocked("default")
	}

	return nil
}

// Close closes current active database
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}
