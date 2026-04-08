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
	WorkflowPlay      WorkflowType = "play"
	WorkflowReview    WorkflowType = "review"
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
	PlaybookFile     string     // Path to saved playbook copy (play sessions only)
	PlayState        string     // JSON-encoded play state (play sessions only)
	PID              int
	DeletedAt        *time.Time // nil if not deleted
	ParentID         string     // ID of parent play session (empty if top-level)
}

// HasTmuxLocation reports whether this session has a recorded tmux location.
func (s *Session) HasTmuxLocation() bool {
	return s.TmuxSession != ""
}

// CreateSession creates a new session in the database
func (db *DB) CreateSession(s *Session) error {
	query := `
		INSERT INTO sessions (
			id, workflow_type, status, working_directory, task_description, prefix,
			claude_session_id, tmux_session, tmux_window, tmux_pane, output_file, playbook_file, play_state, pid, parent_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.conn.Exec(query,
		s.ID, s.WorkflowType, s.Status, s.WorkingDirectory, s.TaskDescription, s.Prefix,
		s.ClaudeSessionID, s.TmuxSession, s.TmuxWindow, s.TmuxPane, s.OutputFile, s.PlaybookFile, s.PlayState, s.PID, s.ParentID,
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
		       output_file, playbook_file, play_state, pid, deleted_at, parent_id
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
		       output_file, playbook_file, play_state, pid, deleted_at, parent_id
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
			       output_file, playbook_file, play_state, pid, deleted_at, parent_id
			FROM sessions WHERE status = ? ORDER BY created_at DESC, rowid DESC
		`
		args = append(args, status)
	} else {
		query = `
			SELECT id, created_at, updated_at, workflow_type, status, working_directory,
			       task_description, prefix, claude_session_id, tmux_session, tmux_window, tmux_pane,
			       output_file, playbook_file, play_state, pid, deleted_at, parent_id
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
			playbook_file = ?,
			play_state = ?,
			pid = ?,
			parent_id = ?
		WHERE id = ?
	`
	_, err := db.conn.Exec(query,
		s.WorkflowType, s.Status, s.WorkingDirectory, s.TaskDescription, s.Prefix,
		s.ClaudeSessionID, s.TmuxSession, s.TmuxWindow, s.TmuxPane, s.OutputFile, s.PlaybookFile, s.PlayState, s.PID,
		s.ParentID, s.ID,
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

// UpdatePlayState updates the play state JSON for a play session
func (db *DB) UpdatePlayState(id, stateJSON string) error {
	query := `UPDATE sessions SET updated_at = CURRENT_TIMESTAMP, play_state = ? WHERE id = ?`
	_, err := db.conn.Exec(query, stateJSON, id)
	if err != nil {
		return fmt.Errorf("update play state: %w", err)
	}
	return nil
}

// ListAbandonedPlaySessions retrieves all abandoned top-level play sessions, newest first
func (db *DB) ListAbandonedPlaySessions() ([]*Session, error) {
	query := `
		SELECT id, created_at, updated_at, workflow_type, status, working_directory,
		       task_description, prefix, claude_session_id, tmux_session, tmux_window, tmux_pane,
		       output_file, playbook_file, play_state, pid, deleted_at, parent_id
		FROM sessions
		WHERE workflow_type = 'play' AND status = 'abandoned'
		AND (parent_id IS NULL OR parent_id = '')
		ORDER BY created_at DESC, rowid DESC
	`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query abandoned play sessions: %w", err)
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

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

// scanSessionFrom scans a session from any scanner (Row or Rows).
func scanSessionFrom(s scanner) (*Session, error) {
	var sess Session
	var taskDesc, prefix, claudeID, outputFile, playbookFile, playState, parentID sql.NullString
	var pid sql.NullInt64
	var deletedAt sql.NullTime

	err := s.Scan(
		&sess.ID, &sess.CreatedAt, &sess.UpdatedAt, &sess.WorkflowType, &sess.Status, &sess.WorkingDirectory,
		&taskDesc, &prefix, &claudeID, &sess.TmuxSession, &sess.TmuxWindow, &sess.TmuxPane,
		&outputFile, &playbookFile, &playState, &pid, &deletedAt, &parentID,
	)
	if err != nil {
		return nil, err
	}

	sess.TaskDescription = taskDesc.String
	sess.Prefix = prefix.String
	sess.ClaudeSessionID = claudeID.String
	sess.OutputFile = outputFile.String
	sess.PlaybookFile = playbookFile.String
	sess.PlayState = playState.String
	sess.PID = int(pid.Int64)
	if deletedAt.Valid {
		sess.DeletedAt = &deletedAt.Time
	}
	sess.ParentID = parentID.String

	return &sess, nil
}

// scanSession scans a single row into a Session
func scanSession(row *sql.Row) (*Session, error) {
	s, err := scanSessionFrom(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan session: %w", err)
	}
	return s, nil
}

// scanSessionRows scans rows into a Session
func scanSessionRows(rows *sql.Rows) (*Session, error) {
	s, err := scanSessionFrom(rows)
	if err != nil {
		return nil, fmt.Errorf("scan session: %w", err)
	}
	return s, nil
}

// ListDeletedSessions retrieves all sessions with deleted status
func (db *DB) ListDeletedSessions() ([]*Session, error) {
	query := `
		SELECT id, created_at, updated_at, workflow_type, status, working_directory,
		       task_description, prefix, claude_session_id, tmux_session, tmux_window, tmux_pane,
		       output_file, playbook_file, play_state, pid, deleted_at, parent_id
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
