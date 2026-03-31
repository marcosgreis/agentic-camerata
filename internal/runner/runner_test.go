package runner

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/agentic-camerata/cmt/internal/db"
)

func TestActivityMonitor(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Create a test session
	session := &db.Session{
		ID:               "test-monitor",
		WorkflowType:     db.WorkflowGeneral,
		Status:           db.StatusWaiting,
		WorkingDirectory: "/tmp",
		TmuxSession:      "main",
		TmuxWindow:       0,
		TmuxPane:         0,
	}
	database.CreateSession(session)

	t.Run("onOutput transitions to working", func(t *testing.T) {
		monitor := newActivityMonitor("test-monitor", database)
		monitor.start()
		defer monitor.stop()

		// Initially not working
		monitor.mu.Lock()
		initialState := monitor.isWorking
		monitor.mu.Unlock()
		if initialState {
			t.Error("Monitor should not be working initially")
		}

		// Trigger output
		monitor.onOutput()

		// Should now be working
		monitor.mu.Lock()
		afterOutput := monitor.isWorking
		monitor.mu.Unlock()
		if !afterOutput {
			t.Error("Monitor should be working after onOutput()")
		}

		// Verify database was updated
		s, _ := database.GetSession("test-monitor")
		if s.Status != db.StatusWorking {
			t.Errorf("Session status = %v, want working", s.Status)
		}
	})

	t.Run("idle timeout transitions back to waiting", func(t *testing.T) {
		// Create a fresh session for this test
		session2 := &db.Session{
			ID:               "test-monitor-2",
			WorkflowType:     db.WorkflowGeneral,
			Status:           db.StatusWaiting,
			WorkingDirectory: "/tmp",
			TmuxSession:      "main",
			TmuxWindow:       0,
			TmuxPane:         0,
		}
		database.CreateSession(session2)

		monitor := newActivityMonitor("test-monitor-2", database)
		monitor.start()
		defer monitor.stop()

		// Trigger output to start working
		monitor.onOutput()

		// Wait longer than idle threshold
		time.Sleep(idleThreshold + 200*time.Millisecond)

		// Should have transitioned back to waiting
		monitor.mu.Lock()
		afterIdle := monitor.isWorking
		monitor.mu.Unlock()
		if afterIdle {
			t.Error("Monitor should return to waiting after idle timeout")
		}

		// Verify database was updated
		s, _ := database.GetSession("test-monitor-2")
		if s.Status != db.StatusWaiting {
			t.Errorf("Session status = %v, want waiting", s.Status)
		}
	})

	t.Run("stop cleanly shuts down the goroutine", func(t *testing.T) {
		monitor := newActivityMonitor("test-monitor", database)
		monitor.start()

		// Stop should not panic and should close the done channel
		monitor.stop()

		// Verify the done channel is closed by trying to receive
		select {
		case <-monitor.done:
			// Expected - channel is closed
		default:
			t.Error("Done channel should be closed after stop()")
		}
	})
}
