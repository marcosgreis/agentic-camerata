package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// DB wraps the SQLite database connection
type DB struct {
	conn *sql.DB
	path string
}

// Open opens or creates the database at the given path
func Open(path string) (*DB, error) {
	// Expand ~ to home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home directory: %w", err)
		}
		path = filepath.Join(home, path[2:])
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	success := false
	defer func() {
		if !success {
			conn.Close()
		}
	}()

	// Enable WAL mode for better concurrency
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	// Run migrations
	if _, err := conn.Exec(schema); err != nil {
		return nil, fmt.Errorf("run schema: %w", err)
	}

	// Migrate legacy 'active' status to 'waiting'
	if _, err := conn.Exec(`UPDATE sessions SET status = 'waiting' WHERE status = 'active'`); err != nil {
		return nil, fmt.Errorf("migrate legacy status: %w", err)
	}

	// Add session columns if they don't exist (for existing databases)
	if err := addColumnIfNotExists(conn, `ALTER TABLE sessions ADD COLUMN prefix TEXT`, "sessions prefix column"); err != nil {
		return nil, err
	}
	if err := addColumnIfNotExists(conn, `ALTER TABLE sessions ADD COLUMN deleted_at DATETIME`, "sessions deleted_at column"); err != nil {
		return nil, err
	}
	if err := addColumnIfNotExists(conn, `ALTER TABLE sessions ADD COLUMN parent_id TEXT`, "sessions parent_id column"); err != nil {
		return nil, err
	}
	if err := addColumnIfNotExists(conn, `ALTER TABLE sessions ADD COLUMN playbook_file TEXT`, "sessions playbook_file column"); err != nil {
		return nil, err
	}
	if err := addColumnIfNotExists(conn, `ALTER TABLE sessions ADD COLUMN play_state TEXT`, "sessions play_state column"); err != nil {
		return nil, err
	}

	// Create todos table if it doesn't exist (for existing databases)
	if _, err := conn.Exec(`CREATE TABLE IF NOT EXISTS todos (
		id TEXT PRIMARY KEY,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		status TEXT NOT NULL DEFAULT 'todo',
		summary TEXT NOT NULL,
		date DATETIME,
		source TEXT,
		url TEXT,
		channel TEXT,
		sender TEXT
	)`); err != nil {
		return nil, fmt.Errorf("migrate todos table: %w", err)
	}

	// Add todos columns if they don't exist
	if err := addColumnIfNotExists(conn, `ALTER TABLE todos ADD COLUMN idempotency_key TEXT`, "todos idempotency_key column"); err != nil {
		return nil, err
	}
	if err := addColumnIfNotExists(conn, `ALTER TABLE todos ADD COLUMN full_message TEXT`, "todos full_message column"); err != nil {
		return nil, err
	}
	if err := addColumnIfNotExists(conn, `ALTER TABLE todos ADD COLUMN deleted_at DATETIME`, "todos deleted_at column"); err != nil {
		return nil, err
	}

	// Create indexes (must be after column migrations)
	indexes := []struct {
		stmt        string
		description string
	}{
		{`CREATE INDEX IF NOT EXISTS idx_sessions_deleted ON sessions(deleted_at)`, "deleted_at index"},
		{`CREATE INDEX IF NOT EXISTS idx_todos_status ON todos(status)`, "todos status index"},
		{`CREATE INDEX IF NOT EXISTS idx_todos_created ON todos(created_at DESC)`, "todos created index"},
		{`CREATE UNIQUE INDEX IF NOT EXISTS idx_todos_idempotency ON todos(idempotency_key) WHERE idempotency_key IS NOT NULL`, "todos idempotency index"},
		{`CREATE INDEX IF NOT EXISTS idx_todos_deleted ON todos(deleted_at)`, "todos deleted_at index"},
	}
	for _, idx := range indexes {
		if _, err := conn.Exec(idx.stmt); err != nil {
			return nil, fmt.Errorf("create %s: %w", idx.description, err)
		}
	}

	// Recover stuck sessions: working sessions with dead PIDs should be marked as abandoned
	rows, err := conn.Query(`SELECT id, pid FROM sessions WHERE status = 'working' AND pid IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("query stuck sessions: %w", err)
	}

	var stuckIDs []string
	for rows.Next() {
		var id string
		var pid int
		if err := rows.Scan(&id, &pid); err != nil {
			continue
		}
		// Check if process is still running
		if !isProcessRunning(pid) {
			stuckIDs = append(stuckIDs, id)
		}
	}
	rows.Close()

	// Mark stuck sessions as abandoned
	for _, id := range stuckIDs {
		conn.Exec(`UPDATE sessions SET status = 'abandoned', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	}

	success = true
	return &DB{conn: conn, path: path}, nil
}

// addColumnIfNotExists runs an ALTER TABLE ADD COLUMN and ignores "duplicate column" errors.
func addColumnIfNotExists(conn *sql.DB, stmt, description string) error {
	_, err := conn.Exec(stmt)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return fmt.Errorf("migrate %s: %w", description, err)
	}
	return nil
}

// isProcessRunning checks if a process with the given PID is still running
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Send signal 0 to check if process exists.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// Path returns the database file path
func (db *DB) Path() string {
	return db.path
}
