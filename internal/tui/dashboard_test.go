package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/agentic-camerata/cmt/internal/db"
)

func TestNewDashboard(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	dashboard := NewDashboard(database)

	if dashboard == nil {
		t.Fatal("NewDashboard() returned nil")
	}

	if dashboard.db != database {
		t.Error("Dashboard database not set correctly")
	}

	if dashboard.focus != focusList {
		t.Errorf("Initial focus = %d, want %d (focusList)", dashboard.focus, focusList)
	}

	if !dashboard.loading {
		t.Error("Dashboard should be in loading state initially")
	}
}

func TestDashboardInit(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	dashboard := NewDashboard(database)
	cmd := dashboard.Init()

	if cmd == nil {
		t.Error("Init() returned nil command")
	}
}

func TestDashboardUpdate(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Create test sessions
	sessions := []*db.Session{
		{ID: "test-1", WorkflowType: db.WorkflowResearch, Status: db.StatusWaiting, WorkingDirectory: "/tmp", TmuxSession: "main", TmuxWindow: 0, TmuxPane: 0},
		{ID: "test-2", WorkflowType: db.WorkflowPlan, Status: db.StatusCompleted, WorkingDirectory: "/tmp", TmuxSession: "main", TmuxWindow: 0, TmuxPane: 0},
	}
	for _, s := range sessions {
		database.CreateSession(s)
	}

	dashboard := NewDashboard(database)
	dashboard.width = 120
	dashboard.height = 40

	t.Run("quit on q", func(t *testing.T) {
		model, cmd := dashboard.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		_ = model

		// Check if quit command was returned
		if cmd == nil {
			t.Error("Expected quit command")
		}
	})

	t.Run("quit on ctrl+c", func(t *testing.T) {
		model, cmd := dashboard.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		_ = model

		if cmd == nil {
			t.Error("Expected quit command")
		}
	})

	t.Run("tab switches focus when info visible", func(t *testing.T) {
		d := NewDashboard(database)
		d.width = 120
		d.height = 40
		d.showInfo = true // Info panel must be visible for tab to switch focus

		initialFocus := d.focus

		model, _ := d.Update(tea.KeyMsg{Type: tea.KeyTab})
		d = model.(*Dashboard)

		if d.focus == initialFocus {
			t.Error("Tab should switch focus when info panel is visible")
		}

		// Tab again should switch back
		model, _ = d.Update(tea.KeyMsg{Type: tea.KeyTab})
		d = model.(*Dashboard)

		if d.focus != initialFocus {
			t.Error("Second tab should switch back to initial focus")
		}
	})

	t.Run("j moves selection down", func(t *testing.T) {
		d := NewDashboard(database)
		d.width = 120
		d.height = 40
		d.sessions = sessions
		d.selected = 0
		d.focus = focusList

		model, _ := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		d = model.(*Dashboard)

		if d.selected != 1 {
			t.Errorf("selected = %d, want 1", d.selected)
		}
	})

	t.Run("k moves selection up", func(t *testing.T) {
		d := NewDashboard(database)
		d.width = 120
		d.height = 40
		d.sessions = sessions
		d.selected = 1
		d.focus = focusList

		model, _ := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		d = model.(*Dashboard)

		if d.selected != 0 {
			t.Errorf("selected = %d, want 0", d.selected)
		}
	})

	t.Run("selection bounds check - top", func(t *testing.T) {
		d := NewDashboard(database)
		d.width = 120
		d.height = 40
		d.sessions = sessions
		d.selected = 0
		d.focus = focusList

		model, _ := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		d = model.(*Dashboard)

		if d.selected != 0 {
			t.Errorf("selected = %d, want 0 (should not go negative)", d.selected)
		}
	})

	t.Run("selection bounds check - bottom", func(t *testing.T) {
		d := NewDashboard(database)
		d.width = 120
		d.height = 40
		d.sessions = sessions
		d.selected = len(sessions) - 1
		d.focus = focusList

		model, _ := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		d = model.(*Dashboard)

		if d.selected != len(sessions)-1 {
			t.Errorf("selected = %d, want %d (should not exceed list)", d.selected, len(sessions)-1)
		}
	})

	t.Run("r refreshes sessions", func(t *testing.T) {
		d := NewDashboard(database)
		d.width = 120
		d.height = 40

		_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

		if cmd == nil {
			t.Error("Expected refresh command")
		}
	})

	t.Run("window size message updates dimensions", func(t *testing.T) {
		d := NewDashboard(database)

		model, _ := d.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
		d = model.(*Dashboard)

		if d.width != 200 {
			t.Errorf("width = %d, want 200", d.width)
		}
		if d.height != 50 {
			t.Errorf("height = %d, want 50", d.height)
		}
	})

	t.Run("sessions loaded message updates state", func(t *testing.T) {
		d := NewDashboard(database)
		d.loading = true

		model, _ := d.Update(sessionsLoadedMsg{sessions: sessions, err: nil})
		d = model.(*Dashboard)

		if d.loading {
			t.Error("loading should be false after sessions loaded")
		}
		if len(d.sessions) != len(sessions) {
			t.Errorf("sessions count = %d, want %d", len(d.sessions), len(sessions))
		}
	})
}

func TestDashboardView(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	t.Run("shows loading when no dimensions", func(t *testing.T) {
		d := NewDashboard(database)
		view := d.View()

		if !strings.Contains(view, "Loading") {
			t.Error("View should show loading when width is 0")
		}
	})

	t.Run("shows error when present", func(t *testing.T) {
		d := NewDashboard(database)
		d.width = 120
		d.height = 40
		d.err = &testError{msg: "test error"}

		view := d.View()

		if !strings.Contains(view, "Error") {
			t.Error("View should show error")
		}
		if !strings.Contains(view, "test error") {
			t.Error("View should contain error message")
		}
	})

	t.Run("shows no sessions message", func(t *testing.T) {
		d := NewDashboard(database)
		d.width = 120
		d.height = 40
		d.sessions = nil

		view := d.View()

		if !strings.Contains(view, "No sessions") {
			t.Error("View should show 'No sessions' message")
		}
	})

	t.Run("shows sessions", func(t *testing.T) {
		sessions := []*db.Session{
			{ID: "view-1", WorkflowType: db.WorkflowResearch, Status: db.StatusWaiting, WorkingDirectory: "/tmp", TmuxSession: "main", TmuxWindow: 0, TmuxPane: 0},
		}
		database.CreateSession(sessions[0])

		d := NewDashboard(database)
		d.width = 120
		d.height = 40
		d.sessions = sessions

		view := d.View()

		if !strings.Contains(view, "view-1") {
			t.Error("View should contain session ID")
		}
		if !strings.Contains(view, "Sessions") {
			t.Error("View should contain Sessions panel title")
		}
	})
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"just now", 30 * time.Second, "now"},
		{"minutes", 5 * time.Minute, "5m"},
		{"hours", 2 * time.Hour, "2h"},
		{"days", 48 * time.Hour, "2d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timestamp := time.Now().Add(-tt.duration)
			got := formatAge(timestamp)

			if got != tt.want {
				t.Errorf("formatAge() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLayoutCalculations(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	d := NewDashboard(database)
	d.width = 100
	d.height = 50

	t.Run("list width is full width minus borders", func(t *testing.T) {
		got := d.listWidth()
		want := d.width - 2 // Full width minus borders

		if got != want {
			t.Errorf("listWidth() = %d, want %d", got, want)
		}
	})

	t.Run("info width fills remainder", func(t *testing.T) {
		infoW := d.infoWidth()

		if infoW > d.width {
			t.Errorf("infoWidth(%d) > width(%d)", infoW, d.width)
		}
	})

	t.Run("content height accounts for header/footer", func(t *testing.T) {
		got := d.contentHeight()

		if got >= d.height {
			t.Errorf("contentHeight() = %d, should be less than height %d", got, d.height)
		}
	})
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// setupTestDB creates a temporary database for testing
func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	return database
}
