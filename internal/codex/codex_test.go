package codex

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
)

func TestBuildCommand(t *testing.T) {
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

	tests := []struct {
		name        string
		opts        agent.RunOptions
		wantArgs    []string
		notWantArgs []string
	}{
		{
			name: "research workflow prefixes task description",
			opts: agent.RunOptions{
				Command:         agent.CommandResearch,
				WorkflowType:    db.WorkflowResearch,
				TaskDescription: "test topic",
			},
			wantArgs: []string{"/research_codebase test topic"},
		},
		{
			name: "fix-local-comments includes custom tag",
			opts: agent.RunOptions{
				Command:         agent.CommandFixLocalComments,
				WorkflowType:    db.WorkflowFix,
				TaskDescription: "auth bug",
				CommentTag:      "FIXME",
			},
			wantArgs:    []string{"comments tagged with FIXME", "auth bug"},
			notWantArgs: []string{"comments tagged with CMT"},
		},
		{
			name: "print mode preserves prefixed task",
			opts: agent.RunOptions{
				Command:         agent.CommandReview,
				WorkflowType:    db.WorkflowReview,
				TaskDescription: "payments",
				PrintMode:       true,
			},
			wantArgs: []string{"-q", "/review_code payments"},
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
