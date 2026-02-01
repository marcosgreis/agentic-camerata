package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpen(t *testing.T) {
	t.Run("creates database file", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		db, err := Open(dbPath)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		defer db.Close()

		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			t.Error("database file was not created")
		}
	})

	t.Run("creates parent directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "nested", "dirs", "test.db")

		db, err := Open(dbPath)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		defer db.Close()

		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			t.Error("database file was not created")
		}
	})

	t.Run("returns correct path", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		db, err := Open(dbPath)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		defer db.Close()

		if db.Path() != dbPath {
			t.Errorf("Path() = %v, want %v", db.Path(), dbPath)
		}
	})

	t.Run("expands tilde in path", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("cannot get home directory")
		}

		// Use a temp subdir in home to avoid polluting home
		tmpName := filepath.Join(".cmt-test-" + time.Now().Format("20060102150405"))
		dbPath := "~/" + tmpName + "/test.db"
		expectedPath := filepath.Join(home, tmpName, "test.db")

		db, err := Open(dbPath)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		defer func() {
			db.Close()
			os.RemoveAll(filepath.Join(home, tmpName))
		}()

		if db.Path() != expectedPath {
			t.Errorf("Path() = %v, want %v", db.Path(), expectedPath)
		}
	})
}

func TestSessionCRUD(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	t.Run("create and get session", func(t *testing.T) {
		session := &Session{
			ID:               "test-001",
			WorkflowType:     WorkflowResearch,
			Status:           StatusWaiting,
			WorkingDirectory: "/home/user/project",
			TaskDescription:  "Test task",
			TmuxSession:      "main",
			TmuxWindow:       0,
			TmuxPane:         1,
		}

		if err := db.CreateSession(session); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		got, err := db.GetSession("test-001")
		if err != nil {
			t.Fatalf("GetSession() error = %v", err)
		}

		if got == nil {
			t.Fatal("GetSession() returned nil")
		}

		if got.ID != session.ID {
			t.Errorf("ID = %v, want %v", got.ID, session.ID)
		}
		if got.WorkflowType != session.WorkflowType {
			t.Errorf("WorkflowType = %v, want %v", got.WorkflowType, session.WorkflowType)
		}
		if got.Status != session.Status {
			t.Errorf("Status = %v, want %v", got.Status, session.Status)
		}
		if got.WorkingDirectory != session.WorkingDirectory {
			t.Errorf("WorkingDirectory = %v, want %v", got.WorkingDirectory, session.WorkingDirectory)
		}
		if got.TaskDescription != session.TaskDescription {
			t.Errorf("TaskDescription = %v, want %v", got.TaskDescription, session.TaskDescription)
		}
		if got.TmuxSession != session.TmuxSession {
			t.Errorf("TmuxSession = %v, want %v", got.TmuxSession, session.TmuxSession)
		}
	})

	t.Run("get non-existent session returns nil", func(t *testing.T) {
		got, err := db.GetSession("non-existent")
		if err != nil {
			t.Fatalf("GetSession() error = %v", err)
		}
		if got != nil {
			t.Errorf("GetSession() = %v, want nil", got)
		}
	})

	t.Run("update session", func(t *testing.T) {
		session := &Session{
			ID:               "test-update",
			WorkflowType:     WorkflowPlan,
			Status:           StatusWaiting,
			WorkingDirectory: "/tmp",
			TmuxSession:      "dev",
			TmuxWindow:       1,
			TmuxPane:         0,
		}

		if err := db.CreateSession(session); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		session.Status = StatusCompleted
		session.TaskDescription = "Updated task"

		if err := db.UpdateSession(session); err != nil {
			t.Fatalf("UpdateSession() error = %v", err)
		}

		got, _ := db.GetSession("test-update")
		if got.Status != StatusCompleted {
			t.Errorf("Status = %v, want %v", got.Status, StatusCompleted)
		}
		if got.TaskDescription != "Updated task" {
			t.Errorf("TaskDescription = %v, want %v", got.TaskDescription, "Updated task")
		}
	})

	t.Run("update session status", func(t *testing.T) {
		session := &Session{
			ID:               "test-status",
			WorkflowType:     WorkflowImplement,
			Status:           StatusWaiting,
			WorkingDirectory: "/tmp",
			TmuxSession:      "main",
			TmuxWindow:       0,
			TmuxPane:         0,
		}

		if err := db.CreateSession(session); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		if err := db.UpdateSessionStatus("test-status", StatusAbandoned); err != nil {
			t.Fatalf("UpdateSessionStatus() error = %v", err)
		}

		got, _ := db.GetSession("test-status")
		if got.Status != StatusAbandoned {
			t.Errorf("Status = %v, want %v", got.Status, StatusAbandoned)
		}
	})

	t.Run("update session PID", func(t *testing.T) {
		session := &Session{
			ID:               "test-pid",
			WorkflowType:     WorkflowGeneral,
			Status:           StatusWaiting,
			WorkingDirectory: "/tmp",
			TmuxSession:      "main",
			TmuxWindow:       0,
			TmuxPane:         0,
		}

		if err := db.CreateSession(session); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		if err := db.UpdateSessionPID("test-pid", 12345); err != nil {
			t.Fatalf("UpdateSessionPID() error = %v", err)
		}

		got, _ := db.GetSession("test-pid")
		if got.PID != 12345 {
			t.Errorf("PID = %v, want %v", got.PID, 12345)
		}
	})

	t.Run("update claude session ID", func(t *testing.T) {
		session := &Session{
			ID:               "test-claude-id",
			WorkflowType:     WorkflowGeneral,
			Status:           StatusWaiting,
			WorkingDirectory: "/tmp",
			TmuxSession:      "main",
			TmuxWindow:       0,
			TmuxPane:         0,
		}

		if err := db.CreateSession(session); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		if err := db.UpdateClaudeSessionID("test-claude-id", "claude-abc123"); err != nil {
			t.Fatalf("UpdateClaudeSessionID() error = %v", err)
		}

		got, _ := db.GetSession("test-claude-id")
		if got.ClaudeSessionID != "claude-abc123" {
			t.Errorf("ClaudeSessionID = %v, want %v", got.ClaudeSessionID, "claude-abc123")
		}
	})

	t.Run("delete session", func(t *testing.T) {
		session := &Session{
			ID:               "test-delete",
			WorkflowType:     WorkflowGeneral,
			Status:           StatusWaiting,
			WorkingDirectory: "/tmp",
			TmuxSession:      "main",
			TmuxWindow:       0,
			TmuxPane:         0,
		}

		if err := db.CreateSession(session); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		if err := db.DeleteSession("test-delete"); err != nil {
			t.Fatalf("DeleteSession() error = %v", err)
		}

		got, _ := db.GetSession("test-delete")
		if got != nil {
			t.Errorf("GetSession() = %v, want nil after delete", got)
		}
	})
}

func TestListSessions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test sessions
	sessions := []*Session{
		{ID: "list-1", WorkflowType: WorkflowResearch, Status: StatusWaiting, WorkingDirectory: "/tmp", TmuxSession: "main", TmuxWindow: 0, TmuxPane: 0},
		{ID: "list-2", WorkflowType: WorkflowPlan, Status: StatusCompleted, WorkingDirectory: "/tmp", TmuxSession: "main", TmuxWindow: 0, TmuxPane: 0},
		{ID: "list-3", WorkflowType: WorkflowImplement, Status: StatusWaiting, WorkingDirectory: "/tmp", TmuxSession: "main", TmuxWindow: 0, TmuxPane: 0},
		{ID: "list-4", WorkflowType: WorkflowGeneral, Status: StatusAbandoned, WorkingDirectory: "/tmp", TmuxSession: "main", TmuxWindow: 0, TmuxPane: 0},
	}

	for _, s := range sessions {
		if err := db.CreateSession(s); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	t.Run("list all sessions", func(t *testing.T) {
		got, err := db.ListSessions("")
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}

		if len(got) != 4 {
			t.Errorf("ListSessions() returned %d sessions, want 4", len(got))
		}

		// Should be ordered by created_at DESC
		if got[0].ID != "list-4" {
			t.Errorf("First session ID = %v, want list-4 (most recent)", got[0].ID)
		}
	})

	t.Run("list active sessions", func(t *testing.T) {
		got, err := db.ListSessions(StatusWaiting)
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}

		if len(got) != 2 {
			t.Errorf("ListSessions(active) returned %d sessions, want 2", len(got))
		}

		for _, s := range got {
			if s.Status != StatusWaiting {
				t.Errorf("Session %s has status %v, want active", s.ID, s.Status)
			}
		}
	})

	t.Run("list completed sessions", func(t *testing.T) {
		got, err := db.ListSessions(StatusCompleted)
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}

		if len(got) != 1 {
			t.Errorf("ListSessions(completed) returned %d sessions, want 1", len(got))
		}
	})
}

func TestGetLastSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	t.Run("returns nil when no sessions", func(t *testing.T) {
		got, err := db.GetLastSession()
		if err != nil {
			t.Fatalf("GetLastSession() error = %v", err)
		}
		if got != nil {
			t.Errorf("GetLastSession() = %v, want nil", got)
		}
	})

	t.Run("returns most recent session", func(t *testing.T) {
		sessions := []*Session{
			{ID: "last-1", WorkflowType: WorkflowGeneral, Status: StatusWaiting, WorkingDirectory: "/tmp", TmuxSession: "main", TmuxWindow: 0, TmuxPane: 0},
			{ID: "last-2", WorkflowType: WorkflowGeneral, Status: StatusWaiting, WorkingDirectory: "/tmp", TmuxSession: "main", TmuxWindow: 0, TmuxPane: 0},
			{ID: "last-3", WorkflowType: WorkflowGeneral, Status: StatusWaiting, WorkingDirectory: "/tmp", TmuxSession: "main", TmuxWindow: 0, TmuxPane: 0},
		}

		for _, s := range sessions {
			if err := db.CreateSession(s); err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}
			time.Sleep(10 * time.Millisecond)
		}

		got, err := db.GetLastSession()
		if err != nil {
			t.Fatalf("GetLastSession() error = %v", err)
		}

		if got.ID != "last-3" {
			t.Errorf("GetLastSession() ID = %v, want last-3", got.ID)
		}
	})
}

// setupTestDB creates a temporary database for testing
func setupTestDB(t *testing.T) *DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	return db
}
