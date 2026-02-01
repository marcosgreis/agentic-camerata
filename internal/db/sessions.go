package db

import (
	"database/sql"
	"fmt"
	"time"
)

// WorkflowType represents the type of coding workflow
type WorkflowType string

const (
	WorkflowGeneral   WorkflowType = "general"
	WorkflowResearch  WorkflowType = "research"
	WorkflowPlan      WorkflowType = "plan"
	WorkflowImplement WorkflowType = "implement"
	WorkflowFix       WorkflowType = "fix"
)

// SessionStatus represents the status of a session
type SessionStatus string

const (
	StatusWaiting   SessionStatus = "waiting"
	StatusWorking   SessionStatus = "working"
	StatusCompleted SessionStatus = "completed"
	StatusAbandoned SessionStatus = "abandoned"
	StatusKilled    SessionStatus = "killed"
	StatusDeleted   SessionStatus = "deleted"
	StatusRestored  SessionStatus = "restored"
)

// Session represents a Claude coding session
type Session struct {
	ID               string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	WorkflowType     WorkflowType
	Status           SessionStatus
	WorkingDirectory string
	TaskDescription  string
	Prefix           string // CMT_PREFIX environment variable value
	ClaudeSessionID  string
	TmuxSession      string
	TmuxWindow       int
	TmuxPane         int
	OutputFile       string
	PID              int
	DeletedAt        *time.Time // nil if not deleted
}

// CreateSession creates a new session in the database
func (db *DB) CreateSession(s *Session) error {
	query := `
		INSERT INTO sessions (
			id, workflow_type, status, working_directory, task_description, prefix,
			claude_session_id, tmux_session, tmux_window, tmux_pane, output_file, pid
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.conn.Exec(query,
		s.ID, s.WorkflowType, s.Status, s.WorkingDirectory, s.TaskDescription, s.Prefix,
		s.ClaudeSessionID, s.TmuxSession, s.TmuxWindow, s.TmuxPane, s.OutputFile, s.PID,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// GetSession retrieves a session by ID
func (db *DB) GetSession(id string) (*Session, error) {
	query := `
		SELECT id, created_at, updated_at, workflow_type, status, working_directory,
		       task_description, prefix, claude_session_id, tmux_session, tmux_window, tmux_pane,
		       output_file, pid, deleted_at
		FROM sessions WHERE id = ?
	`
	row := db.conn.QueryRow(query, id)
	return scanSession(row)
}

// GetLastSession retrieves the most recent session
func (db *DB) GetLastSession() (*Session, error) {
	query := `
		SELECT id, created_at, updated_at, workflow_type, status, working_directory,
		       task_description, prefix, claude_session_id, tmux_session, tmux_window, tmux_pane,
		       output_file, pid, deleted_at
		FROM sessions ORDER BY created_at DESC, rowid DESC LIMIT 1
	`
	row := db.conn.QueryRow(query)
	return scanSession(row)
}

// ListSessions retrieves all sessions, optionally filtered by status
// When status is empty, excludes deleted sessions
func (db *DB) ListSessions(status SessionStatus) ([]*Session, error) {
	var query string
	var args []interface{}

	if status != "" {
		query = `
			SELECT id, created_at, updated_at, workflow_type, status, working_directory,
			       task_description, prefix, claude_session_id, tmux_session, tmux_window, tmux_pane,
			       output_file, pid, deleted_at
			FROM sessions WHERE status = ? ORDER BY created_at DESC, rowid DESC
		`
		args = append(args, status)
	} else {
		query = `
			SELECT id, created_at, updated_at, workflow_type, status, working_directory,
			       task_description, prefix, claude_session_id, tmux_session, tmux_window, tmux_pane,
			       output_file, pid, deleted_at
			FROM sessions WHERE status != 'deleted' ORDER BY created_at DESC, rowid DESC
		`
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		s, err := scanSessionRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}

	return sessions, rows.Err()
}

// UpdateSession updates a session in the database
func (db *DB) UpdateSession(s *Session) error {
	query := `
		UPDATE sessions SET
			updated_at = CURRENT_TIMESTAMP,
			workflow_type = ?,
			status = ?,
			working_directory = ?,
			task_description = ?,
			prefix = ?,
			claude_session_id = ?,
			tmux_session = ?,
			tmux_window = ?,
			tmux_pane = ?,
			output_file = ?,
			pid = ?
		WHERE id = ?
	`
	_, err := db.conn.Exec(query,
		s.WorkflowType, s.Status, s.WorkingDirectory, s.TaskDescription, s.Prefix,
		s.ClaudeSessionID, s.TmuxSession, s.TmuxWindow, s.TmuxPane, s.OutputFile, s.PID,
		s.ID,
	)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}
	return nil
}

// UpdateSessionStatus updates just the status of a session
func (db *DB) UpdateSessionStatus(id string, status SessionStatus) error {
	query := `UPDATE sessions SET updated_at = CURRENT_TIMESTAMP, status = ? WHERE id = ?`
	_, err := db.conn.Exec(query, status, id)
	if err != nil {
		return fmt.Errorf("update session status: %w", err)
	}
	return nil
}

// UpdateSessionPID updates the PID of a session
func (db *DB) UpdateSessionPID(id string, pid int) error {
	query := `UPDATE sessions SET updated_at = CURRENT_TIMESTAMP, pid = ? WHERE id = ?`
	_, err := db.conn.Exec(query, pid, id)
	if err != nil {
		return fmt.Errorf("update session pid: %w", err)
	}
	return nil
}

// UpdateClaudeSessionID updates the Claude session ID
func (db *DB) UpdateClaudeSessionID(id, claudeID string) error {
	query := `UPDATE sessions SET updated_at = CURRENT_TIMESTAMP, claude_session_id = ? WHERE id = ?`
	_, err := db.conn.Exec(query, claudeID, id)
	if err != nil {
		return fmt.Errorf("update claude session id: %w", err)
	}
	return nil
}

// DeleteSession deletes a session by ID
func (db *DB) DeleteSession(id string) error {
	query := `DELETE FROM sessions WHERE id = ?`
	_, err := db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// scanSession scans a single row into a Session
func scanSession(row *sql.Row) (*Session, error) {
	var s Session
	var taskDesc, prefix, claudeID, outputFile sql.NullString
	var pid sql.NullInt64
	var deletedAt sql.NullTime

	err := row.Scan(
		&s.ID, &s.CreatedAt, &s.UpdatedAt, &s.WorkflowType, &s.Status, &s.WorkingDirectory,
		&taskDesc, &prefix, &claudeID, &s.TmuxSession, &s.TmuxWindow, &s.TmuxPane,
		&outputFile, &pid, &deletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan session: %w", err)
	}

	s.TaskDescription = taskDesc.String
	s.Prefix = prefix.String
	s.ClaudeSessionID = claudeID.String
	s.OutputFile = outputFile.String
	s.PID = int(pid.Int64)
	if deletedAt.Valid {
		s.DeletedAt = &deletedAt.Time
	}

	return &s, nil
}

// scanSessionRows scans rows into a Session
func scanSessionRows(rows *sql.Rows) (*Session, error) {
	var s Session
	var taskDesc, prefix, claudeID, outputFile sql.NullString
	var pid sql.NullInt64
	var deletedAt sql.NullTime

	err := rows.Scan(
		&s.ID, &s.CreatedAt, &s.UpdatedAt, &s.WorkflowType, &s.Status, &s.WorkingDirectory,
		&taskDesc, &prefix, &claudeID, &s.TmuxSession, &s.TmuxWindow, &s.TmuxPane,
		&outputFile, &pid, &deletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan session: %w", err)
	}

	s.TaskDescription = taskDesc.String
	s.Prefix = prefix.String
	s.ClaudeSessionID = claudeID.String
	s.OutputFile = outputFile.String
	s.PID = int(pid.Int64)
	if deletedAt.Valid {
		s.DeletedAt = &deletedAt.Time
	}

	return &s, nil
}

// ListDeletedSessions retrieves all sessions with deleted status
func (db *DB) ListDeletedSessions() ([]*Session, error) {
	query := `
		SELECT id, created_at, updated_at, workflow_type, status, working_directory,
		       task_description, prefix, claude_session_id, tmux_session, tmux_window, tmux_pane,
		       output_file, pid, deleted_at
		FROM sessions WHERE status = 'deleted' ORDER BY deleted_at DESC, rowid DESC
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query deleted sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		s, err := scanSessionRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}

	return sessions, rows.Err()
}

// SoftDeleteSession marks a session as deleted with timestamp
func (db *DB) SoftDeleteSession(id string) error {
	query := `UPDATE sessions SET updated_at = CURRENT_TIMESTAMP, status = 'deleted', deleted_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("soft delete session: %w", err)
	}
	return nil
}

// RestoreSession restores a deleted session to 'restored' status
func (db *DB) RestoreSession(id string) error {
	query := `UPDATE sessions SET updated_at = CURRENT_TIMESTAMP, status = 'restored', deleted_at = NULL WHERE id = ?`
	_, err := db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("restore session: %w", err)
	}
	return nil
}

// PruneDeletedSessions permanently removes sessions deleted more than 7 days ago
func (db *DB) PruneDeletedSessions() (int64, error) {
	query := `DELETE FROM sessions WHERE status = 'deleted' AND deleted_at < datetime('now', '-7 days')`
	result, err := db.conn.Exec(query)
	if err != nil {
		return 0, fmt.Errorf("prune deleted sessions: %w", err)
	}
	return result.RowsAffected()
}
