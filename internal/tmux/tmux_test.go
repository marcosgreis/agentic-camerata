package tmux

import (
	"os"
	"testing"
)

func TestInTmux(t *testing.T) {
	t.Run("returns true when TMUX env is set", func(t *testing.T) {
		original := os.Getenv("TMUX")
		defer os.Setenv("TMUX", original)

		os.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

		if !InTmux() {
			t.Error("InTmux() = false, want true when TMUX is set")
		}
	})

	t.Run("returns false when TMUX env is not set", func(t *testing.T) {
		original := os.Getenv("TMUX")
		defer os.Setenv("TMUX", original)

		os.Unsetenv("TMUX")

		if InTmux() {
			t.Error("InTmux() = true, want false when TMUX is not set")
		}
	})
}

func TestRequireTmux(t *testing.T) {
	t.Run("returns nil when in tmux", func(t *testing.T) {
		original := os.Getenv("TMUX")
		defer os.Setenv("TMUX", original)

		os.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

		if err := RequireTmux(); err != nil {
			t.Errorf("RequireTmux() error = %v, want nil", err)
		}
	})

	t.Run("returns error when not in tmux", func(t *testing.T) {
		original := os.Getenv("TMUX")
		defer os.Setenv("TMUX", original)

		os.Unsetenv("TMUX")

		err := RequireTmux()
		if err == nil {
			t.Error("RequireTmux() = nil, want error")
		}
		if err != ErrNotInTmux {
			t.Errorf("RequireTmux() error = %v, want ErrNotInTmux", err)
		}
	})
}

func TestLocationString(t *testing.T) {
	tests := []struct {
		name     string
		location Location
		want     string
	}{
		{
			name:     "simple location",
			location: Location{Session: "main", Window: 0, Pane: 0},
			want:     "main:0.0",
		},
		{
			name:     "with higher indices",
			location: Location{Session: "dev", Window: 2, Pane: 3},
			want:     "dev:2.3",
		},
		{
			name:     "session with special chars",
			location: Location{Session: "my-session", Window: 1, Pane: 0},
			want:     "my-session:1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.location.String()
			if got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCurrentLocation(t *testing.T) {
	t.Run("returns error when not in tmux", func(t *testing.T) {
		original := os.Getenv("TMUX")
		defer os.Setenv("TMUX", original)

		os.Unsetenv("TMUX")

		_, err := CurrentLocation()
		if err == nil {
			t.Error("CurrentLocation() = nil error, want error when not in tmux")
		}
	})

	// Integration test - only run if actually in tmux
	t.Run("returns valid location when in tmux", func(t *testing.T) {
		if os.Getenv("TMUX") == "" {
			t.Skip("skipping: not running in tmux")
		}

		loc, err := CurrentLocation()
		if err != nil {
			t.Fatalf("CurrentLocation() error = %v", err)
		}

		if loc.Session == "" {
			t.Error("Session is empty")
		}
		if loc.Window < 0 {
			t.Error("Window is negative")
		}
		if loc.Pane < 0 {
			t.Error("Pane is negative")
		}

		t.Logf("Current location: %s", loc.String())
	})
}

func TestJumpTo(t *testing.T) {
	t.Run("returns error when not in tmux", func(t *testing.T) {
		original := os.Getenv("TMUX")
		defer os.Setenv("TMUX", original)

		os.Unsetenv("TMUX")

		err := JumpTo(Location{Session: "main", Window: 0, Pane: 0})
		if err == nil {
			t.Error("JumpTo() = nil error, want error when not in tmux")
		}
	})

	// Integration test - only run if actually in tmux
	t.Run("jumps to current location successfully", func(t *testing.T) {
		if os.Getenv("TMUX") == "" {
			t.Skip("skipping: not running in tmux")
		}

		// Get current location
		loc, err := CurrentLocation()
		if err != nil {
			t.Fatalf("CurrentLocation() error = %v", err)
		}

		// Jump to current location (should succeed without changing anything)
		if err := JumpTo(*loc); err != nil {
			t.Errorf("JumpTo() error = %v", err)
		}
	})
}

func TestListSessions(t *testing.T) {
	// Integration test - only works if tmux server is running
	t.Run("lists sessions when tmux is available", func(t *testing.T) {
		if os.Getenv("TMUX") == "" {
			t.Skip("skipping: not running in tmux")
		}

		sessions, err := ListSessions()
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}

		if len(sessions) == 0 {
			t.Error("ListSessions() returned empty list, expected at least one session")
		}

		t.Logf("Found %d tmux sessions: %v", len(sessions), sessions)
	})
}

func TestNewBackgroundPane(t *testing.T) {
	t.Run("returns error when not in tmux", func(t *testing.T) {
		original := os.Getenv("TMUX")
		defer os.Setenv("TMUX", original)
		os.Unsetenv("TMUX")

		_, err := NewBackgroundPane("echo hello")
		if err == nil {
			t.Error("NewBackgroundPane() = nil error, want error when not in tmux")
		}
	})

	// Integration test - only run if actually in tmux
	t.Run("creates and kills a background pane", func(t *testing.T) {
		if os.Getenv("TMUX") == "" {
			t.Skip("skipping: not running in tmux")
		}

		paneID, err := NewBackgroundPane("sleep 10")
		if err != nil {
			t.Fatalf("NewBackgroundPane() error = %v", err)
		}
		if paneID == "" {
			t.Fatal("NewBackgroundPane() returned empty pane ID")
		}
		t.Logf("Created pane: %s", paneID)

		if !IsPaneAlive(paneID) {
			t.Error("IsPaneAlive() = false, want true for newly created pane")
		}

		if err := KillPane(paneID); err != nil {
			t.Errorf("KillPane() error = %v", err)
		}

		if IsPaneAlive(paneID) {
			t.Error("IsPaneAlive() = true, want false after killing pane")
		}
	})
}

func TestIsPaneAlive(t *testing.T) {
	t.Run("returns false for nonexistent pane", func(t *testing.T) {
		if IsPaneAlive("%999999") {
			t.Error("IsPaneAlive() = true for nonexistent pane ID, want false")
		}
	})
}

func TestKillPane(t *testing.T) {
	t.Run("returns error for nonexistent pane when in tmux", func(t *testing.T) {
		if os.Getenv("TMUX") == "" {
			t.Skip("skipping: not running in tmux")
		}

		// Killing a nonexistent pane should produce an error
		err := KillPane("%999999")
		if err == nil {
			t.Error("KillPane() = nil, want error for nonexistent pane")
		}
	})
}

func TestGetPaneWorkingDirectory(t *testing.T) {
	// Integration test - only run if actually in tmux
	t.Run("returns working directory when in tmux", func(t *testing.T) {
		if os.Getenv("TMUX") == "" {
			t.Skip("skipping: not running in tmux")
		}

		loc, err := CurrentLocation()
		if err != nil {
			t.Fatalf("CurrentLocation() error = %v", err)
		}

		dir, err := GetPaneWorkingDirectory(*loc)
		if err != nil {
			t.Fatalf("GetPaneWorkingDirectory() error = %v", err)
		}

		if dir == "" {
			t.Error("GetPaneWorkingDirectory() returned empty string")
		}

		t.Logf("Pane working directory: %s", dir)
	})
}
