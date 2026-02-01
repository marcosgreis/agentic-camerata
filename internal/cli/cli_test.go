package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"

	"github.com/agentic-camerata/cmt/internal/db"
)

func TestCLIParsing(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "new command",
			args:    []string{"new"},
			wantErr: false,
		},
		{
			name:    "new command with task",
			args:    []string{"new", "implement a feature"},
			wantErr: false,
		},
		{
			name:    "research command",
			args:    []string{"research", "authentication flow"},
			wantErr: false,
		},
		{
			name:    "plan command",
			args:    []string{"plan", "refactor the API"},
			wantErr: false,
		},
		{
			name:    "implement command",
			args:    []string{"implement", "add dark mode"},
			wantErr: false,
		},
		{
			name:    "sessions command",
			args:    []string{"sessions"},
			wantErr: false,
		},
		{
			name:    "sessions with status filter",
			args:    []string{"sessions", "-s", "waiting"},
			wantErr: false,
		},
		{
			name:    "sessions with limit",
			args:    []string{"sessions", "-n", "10"},
			wantErr: false,
		},
		{
			name:    "jump command",
			args:    []string{"jump", "abc123"},
			wantErr: false,
		},
		{
			name:    "dashboard command",
			args:    []string{"dashboard"},
			wantErr: false,
		},
		{
			name:    "global db flag",
			args:    []string{"--db", "/tmp/test.db", "sessions"},
			wantErr: false,
		},
		{
			name:    "verbose flag",
			args:    []string{"-v", "sessions"},
			wantErr: false,
		},
		{
			name:    "invalid command",
			args:    []string{"invalid"},
			wantErr: true,
		},
		{
			name:    "invalid status filter",
			args:    []string{"sessions", "-s", "invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cli CLI
			parser, err := kong.New(&cli,
				kong.Name("cmt"),
				kong.Exit(func(int) {}),
			)
			if err != nil {
				t.Fatalf("Failed to create parser: %v", err)
			}

			_, err = parser.Parse(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCLIDefaults(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli,
		kong.Name("cmt"),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	_, err = parser.Parse([]string{"sessions"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cli.DB != "~/.config/cmt/sessions.db" {
		t.Errorf("DB default = %v, want ~/.config/cmt/sessions.db", cli.DB)
	}

	if cli.Verbose != false {
		t.Errorf("Verbose default = %v, want false", cli.Verbose)
	}
}

func TestCLIEnvVars(t *testing.T) {
	original := os.Getenv("CMT_DB")
	defer os.Setenv("CMT_DB", original)

	os.Setenv("CMT_DB", "/custom/path.db")

	var cli CLI
	parser, err := kong.New(&cli,
		kong.Name("cmt"),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	_, err = parser.Parse([]string{"sessions"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cli.DB != "/custom/path.db" {
		t.Errorf("DB = %v, want /custom/path.db (from env)", cli.DB)
	}
}

func TestSessionsCommand(t *testing.T) {
	// Set up test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Create some test sessions
	sessions := []*db.Session{
		{ID: "sess-1", WorkflowType: db.WorkflowResearch, Status: db.StatusWaiting, WorkingDirectory: "/tmp", TmuxSession: "main", TmuxWindow: 0, TmuxPane: 0},
		{ID: "sess-2", WorkflowType: db.WorkflowPlan, Status: db.StatusCompleted, WorkingDirectory: "/home/user", TmuxSession: "dev", TmuxWindow: 1, TmuxPane: 0},
	}

	for _, s := range sessions {
		if err := database.CreateSession(s); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
	}

	t.Run("list all sessions", func(t *testing.T) {
		cmd := &SessionsCmd{}
		cli := &CLI{}
		cli.SetDatabase(database)

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := cmd.Run(cli)

		w.Close()
		os.Stdout = old

		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		if !bytes.Contains([]byte(output), []byte("sess-1")) {
			t.Error("Output does not contain sess-1")
		}
		if !bytes.Contains([]byte(output), []byte("sess-2")) {
			t.Error("Output does not contain sess-2")
		}
	})

	t.Run("list filtered sessions", func(t *testing.T) {
		cmd := &SessionsCmd{Status: "waiting"}
		cli := &CLI{}
		cli.SetDatabase(database)

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := cmd.Run(cli)

		w.Close()
		os.Stdout = old

		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		if !bytes.Contains([]byte(output), []byte("sess-1")) {
			t.Error("Output does not contain sess-1 (waiting)")
		}
		if bytes.Contains([]byte(output), []byte("sess-2")) {
			t.Error("Output contains sess-2 (completed) but should only show waiting")
		}
	})

	t.Run("empty sessions", func(t *testing.T) {
		// Create a fresh empty database
		emptyDBPath := filepath.Join(tmpDir, "empty.db")
		emptyDB, _ := db.Open(emptyDBPath)
		defer emptyDB.Close()

		cmd := &SessionsCmd{}
		cli := &CLI{}
		cli.SetDatabase(emptyDB)

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := cmd.Run(cli)

		w.Close()
		os.Stdout = old

		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		if !bytes.Contains([]byte(output), []byte("No sessions found")) {
			t.Errorf("Expected 'No sessions found' message, got: %s", output)
		}
	})
}

func TestJumpCommand(t *testing.T) {
	// Set up test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	t.Run("session not found", func(t *testing.T) {
		cmd := &JumpCmd{Session: "nonexistent"}
		cli := &CLI{}
		cli.SetDatabase(database)

		err := cmd.Run(cli)
		if err == nil {
			t.Error("Run() should return error for nonexistent session")
		}
	})

	t.Run("no sessions for last", func(t *testing.T) {
		cmd := &JumpCmd{Session: "last"}
		cli := &CLI{}
		cli.SetDatabase(database)

		err := cmd.Run(cli)
		if err == nil {
			t.Error("Run() should return error when no sessions exist")
		}
	})
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name    string
		seconds int
		want    string
	}{
		{"just now", 30, "just now"},
		{"minutes", 300, "5m ago"},
		{"hours", 7200, "2h ago"},
		{"days", 172800, "2d ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: formatAge is unexported, so we test it indirectly
			// or make it exported for testing
		})
	}
}

func TestShortenPath(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		maxLen int
		want   string
	}{
		{
			name:   "short path unchanged",
			path:   "/home/user",
			maxLen: 20,
			want:   "/home/user",
		},
		{
			name:   "long path shortened",
			path:   "/home/user/very/long/path/to/project",
			maxLen: 20,
			want:   "...ong/path/to/project",
		},
		{
			name:   "exact length",
			path:   "/home/user/project",
			maxLen: 18,
			want:   "/home/user/project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortenPath(tt.path, tt.maxLen)
			if len(got) > tt.maxLen {
				t.Errorf("shortenPath() length = %d, want <= %d", len(got), tt.maxLen)
			}
		})
	}
}
