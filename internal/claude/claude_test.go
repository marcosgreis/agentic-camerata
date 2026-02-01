package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentic-camerata/cmt/internal/db"
)

func TestGetPromptPrefix(t *testing.T) {
	tests := []struct {
		name       string
		command    CommandType
		commentTag string
		wantPrefix string
	}{
		{
			name:       "new command has no prefix",
			command:    CommandNew,
			wantPrefix: "",
		},
		{
			name:       "research command has skill prefix",
			command:    CommandResearch,
			wantPrefix: "/research_codebase",
		},
		{
			name:       "plan command has skill prefix",
			command:    CommandPlan,
			wantPrefix: "/create_plan",
		},
		{
			name:       "implement command has skill prefix",
			command:    CommandImplement,
			wantPrefix: "/implement_plan implement all phases ignoring any manual verification steps",
		},
		{
			name:       "fix-test command has instruction prefix",
			command:    CommandFixTest,
			wantPrefix: "Analyze and fix the failing test at:",
		},
		{
			name:       "look-and-fix command has instruction prefix",
			command:    CommandLookAndFix,
			wantPrefix: "Take a look at this repo and search for comments tagged with CMT and propose how to solve them. If a class name or filename is provided as a parameter, focus the search on that specific file or class.",
		},
		{
			name:       "quick command has no prefix",
			command:    CommandQuick,
			wantPrefix: "",
		},
		{
			name:       "unknown command has no prefix",
			command:    CommandType("unknown"),
			wantPrefix: "",
		},
		{
			name:       "look-and-fix with custom tag",
			command:    CommandLookAndFix,
			commentTag: "TODO",
			wantPrefix: "Take a look at this repo and search for comments tagged with TODO and propose how to solve them. If a class name or filename is provided as a parameter, focus the search on that specific file or class.",
		},
		{
			name:       "look-and-fix with empty tag defaults to CMT",
			command:    CommandLookAndFix,
			commentTag: "",
			wantPrefix: "Take a look at this repo and search for comments tagged with CMT and propose how to solve them. If a class name or filename is provided as a parameter, focus the search on that specific file or class.",
		},
		{
			name:       "non-look-and-fix ignores commentTag",
			command:    CommandResearch,
			commentTag: "SOMETHING",
			wantPrefix: "/research_codebase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPromptPrefix(tt.command, tt.commentTag)
			if got != tt.wantPrefix {
				t.Errorf("GetPromptPrefix() = %q, want %q", got, tt.wantPrefix)
			}
		})
	}
}

// TODO: Add prompt content tests when prompts are implemented
// func TestResearchPromptContent(t *testing.T) { ... }
// func TestPlanPromptContent(t *testing.T) { ... }
// func TestImplementPromptContent(t *testing.T) { ... }

func TestNewRunner(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	runner, err := NewRunner(database)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	if runner == nil {
		t.Fatal("NewRunner() returned nil")
	}

	if runner.db != database {
		t.Error("Runner database not set correctly")
	}

	// Check output directory was created
	home, _ := os.UserHomeDir()
	outputDir := filepath.Join(home, ".config", "cmt", "output")
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Error("Output directory was not created")
	}
}

func TestBuildCommand(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	runner, _ := NewRunner(database)

	tests := []struct {
		name        string
		opts        RunOptions
		wantArgs    []string
		notWantArgs []string
	}{
		{
			name: "general workflow no prompt",
			opts: RunOptions{
				Command:      CommandNew,
				WorkflowType: db.WorkflowGeneral,
			},
			notWantArgs: []string{"--system-prompt"},
		},
		{
			name: "research workflow",
			opts: RunOptions{
				Command:         CommandResearch,
				WorkflowType:    db.WorkflowResearch,
				TaskDescription: "test topic",
			},
			wantArgs:    []string{"/research_codebase test topic"},
			notWantArgs: []string{"--system-prompt"},
		},
		{
			name: "with task description",
			opts: RunOptions{
				Command:         CommandNew,
				WorkflowType:    db.WorkflowGeneral,
				TaskDescription: "test task",
			},
			wantArgs:    []string{"test task"},
			notWantArgs: []string{"--prompt", "-p"},
		},
		{
			name: "autonomous mode enabled",
			opts: RunOptions{
				Command:        CommandNew,
				WorkflowType:   db.WorkflowGeneral,
				AutonomousMode: true,
			},
			wantArgs: []string{"--dangerously-skip-permissions"},
		},
		{
			name: "autonomous mode with task",
			opts: RunOptions{
				Command:         CommandNew,
				WorkflowType:    db.WorkflowGeneral,
				TaskDescription: "test task",
				AutonomousMode:  true,
			},
			wantArgs: []string{"--dangerously-skip-permissions", "test task"},
		},
		{
			name: "autonomous mode disabled by default",
			opts: RunOptions{
				Command:         CommandNew,
				WorkflowType:    db.WorkflowGeneral,
				TaskDescription: "test task",
			},
			notWantArgs: []string{"--dangerously-skip-permissions"},
		},
		{
			name: "look-and-fix with custom comment tag",
			opts: RunOptions{
				Command:         CommandLookAndFix,
				WorkflowType:    db.WorkflowFix,
				TaskDescription: "auth bug",
				CommentTag:      "FIXME",
			},
			wantArgs:    []string{"comments tagged with FIXME"},
			notWantArgs: []string{"comments tagged with CMT"},
		},
		{
			name: "look-and-fix defaults to CMT tag",
			opts: RunOptions{
				Command:         CommandLookAndFix,
				WorkflowType:    db.WorkflowFix,
				TaskDescription: "auth bug",
			},
			wantArgs: []string{"comments tagged with CMT"},
		},
		{
			name: "resume interactive picker",
			opts: RunOptions{
				Command:         CommandNew,
				WorkflowType:    db.WorkflowGeneral,
				ResumeSessionID: "*",
			},
			wantArgs:    []string{"--resume"},
			notWantArgs: []string{"--resume *"},
		},
		{
			name: "resume specific session",
			opts: RunOptions{
				Command:         CommandNew,
				WorkflowType:    db.WorkflowGeneral,
				ResumeSessionID: "abc123",
			},
			wantArgs: []string{"--resume", "abc123"},
		},
		{
			name: "no resume by default",
			opts: RunOptions{
				Command:      CommandNew,
				WorkflowType: db.WorkflowGeneral,
			},
			notWantArgs: []string{"--resume"},
		},
		{
			name: "resume with task description",
			opts: RunOptions{
				Command:         CommandNew,
				WorkflowType:    db.WorkflowGeneral,
				TaskDescription: "fix the bug",
				ResumeSessionID: "*",
			},
			wantArgs: []string{"--resume", "fix the bug"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := runner.buildCommand(tt.opts)

			args := strings.Join(cmd.Args, " ")

			for _, want := range tt.wantArgs {
				if !strings.Contains(args, want) {
					t.Errorf("Command args %q does not contain %q", args, want)
				}
			}

			for _, notWant := range tt.notWantArgs {
				if strings.Contains(args, notWant) {
					t.Errorf("Command args %q should not contain %q", args, notWant)
				}
			}
		})
	}
}

func TestRunRequiresTmux(t *testing.T) {
	// Ensure we're not in tmux for this test
	original := os.Getenv("TMUX")
	defer os.Setenv("TMUX", original)
	os.Unsetenv("TMUX")

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	runner, _ := NewRunner(database)

	err = runner.Run(nil, RunOptions{
		WorkflowType: db.WorkflowGeneral,
	})

	if err == nil {
		t.Error("Run() should return error when not in tmux")
	}

	if !strings.Contains(err.Error(), "tmux") {
		t.Errorf("Error should mention tmux, got: %v", err)
	}
}

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
