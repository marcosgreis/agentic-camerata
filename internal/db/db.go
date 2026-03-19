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

	// Enable WAL mode for better concurrency
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	// Run migrations
	if _, err := conn.Exec(schema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("run schema: %w", err)
	}

	// Migrate legacy 'active' status to 'waiting'
	if _, err := conn.Exec(`UPDATE sessions SET status = 'waiting' WHERE status = 'active'`); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate legacy status: %w", err)
	}

	// Add prefix column if it doesn't exist (for existing databases)
	_, err = conn.Exec(`ALTER TABLE sessions ADD COLUMN prefix TEXT`)
	if err != nil {
		// Ignore error if column already exists (SQLite returns "duplicate column name")
		if !strings.Contains(err.Error(), "duplicate column") {
			conn.Close()
			return nil, fmt.Errorf("migrate prefix column: %w", err)
		}
	}

	// Add deleted_at column if it doesn't exist (for existing databases)
	_, err = conn.Exec(`ALTER TABLE sessions ADD COLUMN deleted_at DATETIME`)
	if err != nil {
		// Ignore error if column already exists
		if !strings.Contains(err.Error(), "duplicate column") {
			conn.Close()
			return nil, fmt.Errorf("migrate deleted_at column: %w", err)
		}
	}

	// Create index on deleted_at (must be after column migration)
	_, err = conn.Exec(`CREATE INDEX IF NOT EXISTS idx_sessions_deleted ON sessions(deleted_at)`)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create deleted_at index: %w", err)
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
		conn.Close()
		return nil, fmt.Errorf("migrate todos table: %w", err)
	}
	if _, err := conn.Exec(`CREATE INDEX IF NOT EXISTS idx_todos_status ON todos(status)`); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate todos status index: %w", err)
	}
	if _, err := conn.Exec(`CREATE INDEX IF NOT EXISTS idx_todos_created ON todos(created_at DESC)`); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate todos created index: %w", err)
	}

	// Add idempotency_key column to todos if it doesn't exist
	_, err = conn.Exec(`ALTER TABLE todos ADD COLUMN idempotency_key TEXT`)
	if err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			conn.Close()
			return nil, fmt.Errorf("migrate todos idempotency_key column: %w", err)
		}
	}
	_, err = conn.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_todos_idempotency ON todos(idempotency_key) WHERE idempotency_key IS NOT NULL`)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create todos idempotency index: %w", err)
	}

	// Add full_message column to todos if it doesn't exist
	_, err = conn.Exec(`ALTER TABLE todos ADD COLUMN full_message TEXT`)
	if err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			conn.Close()
			return nil, fmt.Errorf("migrate todos full_message column: %w", err)
		}
	}

	// Add deleted_at column to todos if it doesn't exist
	_, err = conn.Exec(`ALTER TABLE todos ADD COLUMN deleted_at DATETIME`)
	if err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			conn.Close()
			return nil, fmt.Errorf("migrate todos deleted_at column: %w", err)
		}
	}
	_, err = conn.Exec(`CREATE INDEX IF NOT EXISTS idx_todos_deleted ON todos(deleted_at)`)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create todos deleted_at index: %w", err)
	}

	// Add parent_id column if it doesn't exist (for existing databases)
	_, err = conn.Exec(`ALTER TABLE sessions ADD COLUMN parent_id TEXT`)
	if err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			conn.Close()
			return nil, fmt.Errorf("migrate parent_id column: %w", err)
		}
	}

	// Add playbook_file column if it doesn't exist (for existing databases)
	_, err = conn.Exec(`ALTER TABLE sessions ADD COLUMN playbook_file TEXT`)
	if err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			conn.Close()
			return nil, fmt.Errorf("migrate playbook_file column: %w", err)
		}
	}

	// Recover stuck sessions: working sessions with dead PIDs should be marked as abandoned
	rows, err := conn.Query(`SELECT id, pid FROM sessions WHERE status = 'working' AND pid IS NOT NULL`)
	if err != nil {
		conn.Close()
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

	return &DB{conn: conn, path: path}, nil
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
