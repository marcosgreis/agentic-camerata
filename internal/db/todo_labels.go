package db

import (
	"fmt"
	"strings"
)

// AddTodoLabels adds one or more labels to a todo. Labels are trimmed,
// de-duplicated case-insensitively, and blank labels are ignored. Adding an
// existing label is a no-op.
func (db *DB) AddTodoLabels(id string, labels []string) error {
	labels = normalizeLabels(labels)
	if len(labels) == 0 {
		return nil
	}
	exists, err := db.todoExists(id)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("todo %q not found", id)
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin label transaction: %w", err)
	}
	defer tx.Rollback()

	for _, label := range labels {
		if _, err := tx.Exec(
			"INSERT OR IGNORE INTO todo_labels (todo_id, label) VALUES (?, ?)",
			id, label,
		); err != nil {
			return fmt.Errorf("add label %q: %w", label, err)
		}
	}
	if _, err := tx.Exec("UPDATE todos SET updated_at = CURRENT_TIMESTAMP WHERE id = ?", id); err != nil {
		return fmt.Errorf("touch todo: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit labels: %w", err)
	}
	return nil
}

// RemoveTodoLabels removes one or more labels from a todo (case-insensitive).
// Removing a label that is not present is a no-op.
func (db *DB) RemoveTodoLabels(id string, labels []string) error {
	labels = normalizeLabels(labels)
	if len(labels) == 0 {
		return nil
	}
	exists, err := db.todoExists(id)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("todo %q not found", id)
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin label transaction: %w", err)
	}
	defer tx.Rollback()

	for _, label := range labels {
		if _, err := tx.Exec(
			"DELETE FROM todo_labels WHERE todo_id = ? AND label = ? COLLATE NOCASE",
			id, label,
		); err != nil {
			return fmt.Errorf("remove label %q: %w", label, err)
		}
	}
	if _, err := tx.Exec("UPDATE todos SET updated_at = CURRENT_TIMESTAMP WHERE id = ?", id); err != nil {
		return fmt.Errorf("touch todo: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit labels: %w", err)
	}
	return nil
}

// TodoLabels returns the labels attached to a todo, alphabetically.
func (db *DB) TodoLabels(id string) ([]string, error) {
	rows, err := db.conn.Query(
		"SELECT label FROM todo_labels WHERE todo_id = ? ORDER BY label COLLATE NOCASE", id,
	)
	if err != nil {
		return nil, fmt.Errorf("query labels: %w", err)
	}
	defer rows.Close()

	labels := []string{}
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			return nil, fmt.Errorf("scan label: %w", err)
		}
		labels = append(labels, label)
	}
	return labels, rows.Err()
}

// TodoLabelCount is a label together with the number of todos carrying it.
type TodoLabelCount struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// ListTodoLabels returns every label in use with its todo count, alphabetically.
func (db *DB) ListTodoLabels() ([]TodoLabelCount, error) {
	rows, err := db.conn.Query(
		"SELECT label, COUNT(*) FROM todo_labels GROUP BY label COLLATE NOCASE ORDER BY label COLLATE NOCASE",
	)
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	defer rows.Close()

	out := []TodoLabelCount{}
	for rows.Next() {
		var lc TodoLabelCount
		if err := rows.Scan(&lc.Label, &lc.Count); err != nil {
			return nil, fmt.Errorf("scan label count: %w", err)
		}
		out = append(out, lc)
	}
	return out, rows.Err()
}

// normalizeLabels trims whitespace, drops blanks, and de-duplicates
// case-insensitively while preserving the first spelling seen.
func normalizeLabels(labels []string) []string {
	seen := make(map[string]bool, len(labels))
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		key := strings.ToLower(label)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, label)
	}
	return out
}

// todoExists reports whether a todo row exists (including soft-deleted rows).
func (db *DB) todoExists(id string) (bool, error) {
	var n int
	if err := db.conn.QueryRow("SELECT COUNT(1) FROM todos WHERE id = ?", id).Scan(&n); err != nil {
		return false, fmt.Errorf("check todo: %w", err)
	}
	return n > 0, nil
}

// loadTodoLabels fills Labels on the given todos with one query per chunk of
// ids, keeping list/search at a constant number of queries.
func (db *DB) loadTodoLabels(todos []*Todo) error {
	if len(todos) == 0 {
		return nil
	}
	byID := make(map[string]*Todo, len(todos))
	ids := make([]interface{}, 0, len(todos))
	seen := make(map[string]bool, len(todos))
	for _, t := range todos {
		if t == nil {
			continue
		}
		t.Labels = []string{}
		byID[t.ID] = t
		if !seen[t.ID] {
			seen[t.ID] = true
			ids = append(ids, t.ID)
		}
	}

	const chunkSize = 400
	for start := 0; start < len(ids); start += chunkSize {
		end := start + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(chunk)), ",")
		rows, err := db.conn.Query(
			"SELECT todo_id, label FROM todo_labels WHERE todo_id IN ("+placeholders+") ORDER BY label COLLATE NOCASE",
			chunk...,
		)
		if err != nil {
			return fmt.Errorf("load labels: %w", err)
		}
		for rows.Next() {
			var id, label string
			if err := rows.Scan(&id, &label); err != nil {
				rows.Close()
				return fmt.Errorf("scan label: %w", err)
			}
			if t := byID[id]; t != nil {
				t.Labels = append(t.Labels, label)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("load labels: %w", err)
		}
		rows.Close()
	}
	return nil
}
