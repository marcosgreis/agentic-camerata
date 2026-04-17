package db

import (
	"database/sql"
	"fmt"
	"time"
)

// TodoStatus represents the completion state of a todo item
type TodoStatus string

const (
	TodoStatusTodo    TodoStatus = "todo"
	TodoStatusDone    TodoStatus = "done"
	TodoStatusDeleted TodoStatus = "deleted"
)

// Todo represents a single todo entry
type Todo struct {
	ID             string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Status         TodoStatus
	Summary        string
	Date           *time.Time
	Source         *string
	URL            *string
	Channel        *string
	Sender         *string
	IdempotencyKey *string
	FullMessage    *string
	DeletedAt      *time.Time
}

// CreateTodo inserts a new todo into the database.
// If an IdempotencyKey is set and a todo with the same key already exists,
// the existing todo's ID is copied back to t and nil is returned (no new row).
func (db *DB) CreateTodo(t *Todo) error {
	if t.IdempotencyKey != nil {
		existing, err := db.GetTodoByIdempotencyKey(*t.IdempotencyKey)
		if err != nil {
			return fmt.Errorf("check idempotency key: %w", err)
		}
		if existing != nil {
			t.ID = existing.ID
			return nil
		}
	}
	query := `
		INSERT INTO todos (id, status, summary, date, source, url, channel, sender, idempotency_key, full_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.conn.Exec(query,
		t.ID, t.Status, t.Summary,
		nullTime(t.Date), nullString(t.Source), nullString(t.URL),
		nullString(t.Channel), nullString(t.Sender),
		nullString(t.IdempotencyKey), nullString(t.FullMessage),
	)
	if err != nil {
		return fmt.Errorf("insert todo: %w", err)
	}
	return nil
}

// GetTodoByIdempotencyKey retrieves a todo by its idempotency key
func (db *DB) GetTodoByIdempotencyKey(key string) (*Todo, error) {
	query := `
		SELECT id, created_at, updated_at, status, summary, date, source, url, channel, sender, idempotency_key, full_message, deleted_at
		FROM todos WHERE idempotency_key = ?
	`
	row := db.conn.QueryRow(query, key)
	return scanTodo(row)
}

// GetTodo retrieves a todo by ID
func (db *DB) GetTodo(id string) (*Todo, error) {
	query := `
		SELECT id, created_at, updated_at, status, summary, date, source, url, channel, sender, idempotency_key, full_message, deleted_at
		FROM todos WHERE id = ?
	`
	row := db.conn.QueryRow(query, id)
	return scanTodo(row)
}

// ListTodos retrieves todos optionally filtered by status. Empty status returns all non-deleted.
func (db *DB) ListTodos(status TodoStatus) ([]*Todo, error) {
	var (
		query string
		args  []interface{}
	)

	if status == TodoStatusDeleted {
		query = `
			SELECT id, created_at, updated_at, status, summary, date, source, url, channel, sender, idempotency_key, full_message, deleted_at
			FROM todos WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC
		`
	} else if status != "" {
		query = `
			SELECT id, created_at, updated_at, status, summary, date, source, url, channel, sender, idempotency_key, full_message, deleted_at
			FROM todos WHERE status = ? AND deleted_at IS NULL ORDER BY created_at DESC
		`
		args = append(args, status)
	} else {
		query = `
			SELECT id, created_at, updated_at, status, summary, date, source, url, channel, sender, idempotency_key, full_message, deleted_at
			FROM todos WHERE deleted_at IS NULL ORDER BY created_at DESC
		`
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query todos: %w", err)
	}
	defer rows.Close()

	var todos []*Todo
	for rows.Next() {
		t, err := scanTodoRows(rows)
		if err != nil {
			return nil, err
		}
		todos = append(todos, t)
	}

	return todos, rows.Err()
}

// TodoFilter holds optional search criteria for todos.
// All non-nil fields are combined with AND logic.
type TodoFilter struct {
	ID             *string
	IdempotencyKey *string
	Status         *TodoStatus
	URL            *string
	Sender         *string
	Source         *string
	IncludeDeleted bool
}

// SearchTodos retrieves todos matching the filter criteria.
// All non-nil filter fields are combined with AND.
func (db *DB) SearchTodos(f TodoFilter) ([]*Todo, error) {
	query := `
		SELECT id, created_at, updated_at, status, summary, date, source, url, channel, sender, idempotency_key, full_message, deleted_at
		FROM todos WHERE 1=1
	`
	var args []interface{}

	if !f.IncludeDeleted {
		query += " AND deleted_at IS NULL"
	}
	if f.ID != nil {
		query += " AND id = ?"
		args = append(args, *f.ID)
	}
	if f.IdempotencyKey != nil {
		query += " AND idempotency_key = ?"
		args = append(args, *f.IdempotencyKey)
	}
	if f.Status != nil {
		query += " AND status = ?"
		args = append(args, *f.Status)
	}
	if f.URL != nil {
		query += " AND url = ?"
		args = append(args, *f.URL)
	}
	if f.Sender != nil {
		query += " AND sender = ?"
		args = append(args, *f.Sender)
	}
	if f.Source != nil {
		query += " AND source = ?"
		args = append(args, *f.Source)
	}

	query += " ORDER BY created_at DESC"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search todos: %w", err)
	}
	defer rows.Close()

	var todos []*Todo
	for rows.Next() {
		t, err := scanTodoRows(rows)
		if err != nil {
			return nil, err
		}
		todos = append(todos, t)
	}

	return todos, rows.Err()
}

// UpdateTodo updates all mutable fields of a todo
func (db *DB) UpdateTodo(t *Todo) error {
	query := `
		UPDATE todos SET
			updated_at = CURRENT_TIMESTAMP,
			status = ?,
			summary = ?,
			date = ?,
			source = ?,
			url = ?,
			channel = ?,
			sender = ?,
			full_message = ?
		WHERE id = ?
	`
	_, err := db.conn.Exec(query,
		t.Status, t.Summary,
		nullTime(t.Date), nullString(t.Source), nullString(t.URL),
		nullString(t.Channel), nullString(t.Sender),
		nullString(t.FullMessage),
		t.ID,
	)
	if err != nil {
		return fmt.Errorf("update todo: %w", err)
	}
	return nil
}

// DeleteTodo soft-deletes a todo by setting deleted_at timestamp
func (db *DB) DeleteTodo(id string) error {
	_, err := db.conn.Exec(
		`UPDATE todos SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("delete todo: %w", err)
	}
	return nil
}

// scanTodo scans a single row into a Todo
func scanTodo(row *sql.Row) (*Todo, error) {
	var t Todo
	var date, deletedAt sql.NullTime
	var source, url, channel, sender, idempotencyKey, fullMessage sql.NullString

	err := row.Scan(
		&t.ID, &t.CreatedAt, &t.UpdatedAt, &t.Status, &t.Summary,
		&date, &source, &url, &channel, &sender, &idempotencyKey, &fullMessage,
		&deletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan todo: %w", err)
	}

	if date.Valid {
		t.Date = &date.Time
	}
	if source.Valid {
		t.Source = &source.String
	}
	if url.Valid {
		t.URL = &url.String
	}
	if channel.Valid {
		t.Channel = &channel.String
	}
	if sender.Valid {
		t.Sender = &sender.String
	}
	if idempotencyKey.Valid {
		t.IdempotencyKey = &idempotencyKey.String
	}
	if fullMessage.Valid {
		t.FullMessage = &fullMessage.String
	}
	if deletedAt.Valid {
		t.DeletedAt = &deletedAt.Time
	}

	return &t, nil
}

// scanTodoRows scans a rows cursor into a Todo
func scanTodoRows(rows *sql.Rows) (*Todo, error) {
	var t Todo
	var date, deletedAt sql.NullTime
	var source, url, channel, sender, idempotencyKey, fullMessage sql.NullString

	err := rows.Scan(
		&t.ID, &t.CreatedAt, &t.UpdatedAt, &t.Status, &t.Summary,
		&date, &source, &url, &channel, &sender, &idempotencyKey, &fullMessage,
		&deletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan todo: %w", err)
	}

	if date.Valid {
		t.Date = &date.Time
	}
	if source.Valid {
		t.Source = &source.String
	}
	if url.Valid {
		t.URL = &url.String
	}
	if channel.Valid {
		t.Channel = &channel.String
	}
	if sender.Valid {
		t.Sender = &sender.String
	}
	if idempotencyKey.Valid {
		t.IdempotencyKey = &idempotencyKey.String
	}
	if fullMessage.Valid {
		t.FullMessage = &fullMessage.String
	}
	if deletedAt.Valid {
		t.DeletedAt = &deletedAt.Time
	}

	return &t, nil
}

// nullString converts a *string to sql.NullString
func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

// nullTime converts a *time.Time to sql.NullTime
func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}
