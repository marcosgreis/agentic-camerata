package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Venue represents a pinned venue directory
type Venue struct {
	Directory string
	CreatedAt time.Time
}

// AddVenue pins a directory as a venue
func (db *DB) AddVenue(directory string) error {
	_, err := db.conn.Exec(`INSERT OR IGNORE INTO venues (directory) VALUES (?)`, directory)
	if err != nil {
		return fmt.Errorf("add venue: %w", err)
	}
	return nil
}

// RemoveVenue removes a pinned venue
func (db *DB) RemoveVenue(directory string) error {
	_, err := db.conn.Exec(`DELETE FROM venues WHERE directory = ?`, directory)
	if err != nil {
		return fmt.Errorf("remove venue: %w", err)
	}
	return nil
}

// ListVenues returns all pinned venues ordered by creation time
func (db *DB) ListVenues() ([]*Venue, error) {
	rows, err := db.conn.Query(`SELECT directory, created_at FROM venues ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("query venues: %w", err)
	}
	defer rows.Close()

	var venues []*Venue
	for rows.Next() {
		var v Venue
		if err := rows.Scan(&v.Directory, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan venue: %w", err)
		}
		venues = append(venues, &v)
	}
	return venues, rows.Err()
}

// IsVenuePinned checks if a directory is pinned
func (db *DB) IsVenuePinned(directory string) (bool, error) {
	var exists int
	err := db.conn.QueryRow(`SELECT 1 FROM venues WHERE directory = ? LIMIT 1`, directory).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check venue pinned: %w", err)
	}
	return true, nil
}
