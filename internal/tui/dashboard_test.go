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

func TestBuildSessionTree(t *testing.T) {
	now := time.Now()
	makeSession := func(id, parentID string, status db.SessionStatus, offsetSec int) *db.Session {
		return &db.Session{
			ID:        id,
			ParentID:  parentID,
			Status:    status,
			CreatedAt: now.Add(time.Duration(offsetSec) * time.Second),
		}
	}

	t.Run("empty list", func(t *testing.T) {
		nodes := buildSessionTree(nil)
		if len(nodes) != 0 {
			t.Errorf("expected 0 nodes, got %d", len(nodes))
		}
	})

	t.Run("all top-level", func(t *testing.T) {
		sessions := []*db.Session{
			makeSession("a", "", db.StatusCompleted, 0),
			makeSession("b", "", db.StatusWaiting, 1),
		}
		nodes := buildSessionTree(sessions)
		if len(nodes) != 2 {
			t.Fatalf("expected 2 nodes, got %d", len(nodes))
		}
		// Running should come first
		if nodes[0].session.ID != "b" {
			t.Errorf("expected running session 'b' first, got '%s'", nodes[0].session.ID)
		}
		for _, n := range nodes {
			if n.depth != 0 {
				t.Errorf("session %s: expected depth 0, got %d", n.session.ID, n.depth)
			}
		}
	})

	t.Run("parent with children", func(t *testing.T) {
		parent := makeSession("parent", "", db.StatusWorking, 0)
		child1 := makeSession("child1", "parent", db.StatusCompleted, 1)
		child2 := makeSession("child2", "parent", db.StatusWorking, 2)
		sessions := []*db.Session{child2, parent, child1} // shuffled

		nodes := buildSessionTree(sessions)
		if len(nodes) != 3 {
			t.Fatalf("expected 3 nodes, got %d", len(nodes))
		}
		if nodes[0].session.ID != "parent" {
			t.Errorf("expected parent first, got %s", nodes[0].session.ID)
		}
		if nodes[1].session.ID != "child1" {
			t.Errorf("expected child1 second (oldest), got %s", nodes[1].session.ID)
		}
		if nodes[2].session.ID != "child2" {
			t.Errorf("expected child2 third, got %s", nodes[2].session.ID)
		}
		if nodes[0].depth != 0 {
			t.Errorf("parent depth: got %d, want 0", nodes[0].depth)
		}
		if nodes[1].depth != 1 {
			t.Errorf("child1 depth: got %d, want 1", nodes[1].depth)
		}
	})

	t.Run("orphaned child treated as top-level", func(t *testing.T) {
		orphan := makeSession("orphan", "missing-parent", db.StatusCompleted, 0)
		nodes := buildSessionTree([]*db.Session{orphan})
		if len(nodes) != 1 {
			t.Fatalf("expected 1 node, got %d", len(nodes))
		}
		if nodes[0].depth != 0 {
			t.Errorf("orphan depth: got %d, want 0", nodes[0].depth)
		}
	})

	t.Run("grandchild nesting", func(t *testing.T) {
		parent := makeSession("p", "", db.StatusWorking, 0)
		child := makeSession("c", "p", db.StatusWorking, 1)
		grand := makeSession("g", "c", db.StatusCompleted, 2)
		nodes := buildSessionTree([]*db.Session{grand, child, parent})
		if len(nodes) != 3 {
			t.Fatalf("expected 3 nodes, got %d", len(nodes))
		}
		if nodes[2].depth != 2 {
			t.Errorf("grandchild depth: got %d, want 2", nodes[2].depth)
		}
	})

	t.Run("completed child of running parent stays inRunning", func(t *testing.T) {
		parent := makeSession("p", "", db.StatusWorking, 0)
		child := makeSession("c", "p", db.StatusCompleted, 1)
		nodes := buildSessionTree([]*db.Session{parent, child})
		if !nodes[1].inRunning {
			t.Error("completed child of running parent should be inRunning")
		}
	})

	t.Run("completed child of completed parent is not inRunning", func(t *testing.T) {
		parent := makeSession("p", "", db.StatusCompleted, 0)
		child := makeSession("c", "p", db.StatusCompleted, 1)
		nodes := buildSessionTree([]*db.Session{parent, child})
		if nodes[0].inRunning {
			t.Error("completed parent should not be inRunning")
		}
		if nodes[1].inRunning {
			t.Error("completed child of completed parent should not be inRunning")
		}
	})

	t.Run("running child of completed parent is inRunning itself", func(t *testing.T) {
		parent := makeSession("p", "", db.StatusCompleted, 0)
		child := makeSession("c", "p", db.StatusWorking, 1)
		nodes := buildSessionTree([]*db.Session{parent, child})
		// Parent is completed with no running ancestor: not inRunning
		if nodes[0].inRunning {
			t.Error("completed parent with no running ancestor should not be inRunning")
		}
		// Child is running itself: inRunning regardless of parent
		if !nodes[1].inRunning {
			t.Error("running child should be inRunning")
		}
	})
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
