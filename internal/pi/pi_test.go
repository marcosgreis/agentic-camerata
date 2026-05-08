package pi

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
			name: "general workflow with default model",
			opts: agent.RunOptions{
				Command:         agent.CommandNew,
				WorkflowType:    db.WorkflowGeneral,
				TaskDescription: "test task",
			},
			wantArgs:    []string{"--model", "claude-opus-4-6", "test task"},
			notWantArgs: []string{"-p", "--resume", "--no-session"},
		},
		{
			name: "research workflow with prompt prefix",
			opts: agent.RunOptions{
				Command:         agent.CommandResearch,
				WorkflowType:    db.WorkflowResearch,
				TaskDescription: "test topic",
			},
			wantArgs: []string{"/research_codebase test topic"},
		},
		{
			name: "quick command uses flash model",
			opts: agent.RunOptions{
				Command:         agent.CommandQuick,
				WorkflowType:    db.WorkflowGeneral,
				TaskDescription: "what time is it",
				PrintMode:       true,
				SkipTracking:    true,
			},
			wantArgs: []string{"--model", "claude-haiku-4-5", "-p", "--no-session", "what time is it"},
		},
		{
			name: "print mode adds -p flag",
			opts: agent.RunOptions{
				Command:         agent.CommandPlan,
				WorkflowType:    db.WorkflowPlan,
				TaskDescription: "checkout flow",
				PrintMode:       true,
			},
			wantArgs: []string{"-p", "/create_plan checkout flow"},
		},
		{
			name: "model override",
			opts: agent.RunOptions{
				Command:         agent.CommandNew,
				WorkflowType:    db.WorkflowGeneral,
				TaskDescription: "test",
				Model:           "anthropic/claude-opus-4-20250514",
			},
			wantArgs:    []string{"--model", "anthropic/claude-opus-4-20250514"},
			notWantArgs: []string{"claude-opus-4-6"},
		},
		{
			name: "no autonomous flag even when requested",
			opts: agent.RunOptions{
				Command:        agent.CommandNew,
				WorkflowType:   db.WorkflowGeneral,
				AutonomousMode: true,
			},
			notWantArgs: []string{"--dangerously", "-a"},
		},
		{
			name: "resume interactive picker",
			opts: agent.RunOptions{
				Command:         agent.CommandNew,
				WorkflowType:    db.WorkflowGeneral,
				ResumeSessionID: "*",
			},
			wantArgs:    []string{"--resume"},
			notWantArgs: []string{"--session", "--resume *"},
		},
		{
			name: "resume specific session",
			opts: agent.RunOptions{
				Command:         agent.CommandNew,
				WorkflowType:    db.WorkflowGeneral,
				ResumeSessionID: "abc123",
			},
			wantArgs:    []string{"--session", "abc123"},
			notWantArgs: []string{"--resume"},
		},
		{
			name: "fix-local-comments with custom tag",
			opts: agent.RunOptions{
				Command:         agent.CommandFixLocalComments,
				WorkflowType:    db.WorkflowFix,
				TaskDescription: "auth bug",
				CommentTag:      "FIXME",
			},
			wantArgs: []string{"comments tagged with FIXME", "auth bug"},
		},
		{
			name: "fix-local-comments defaults to CMT tag",
			opts: agent.RunOptions{
				Command:         agent.CommandFixLocalComments,
				WorkflowType:    db.WorkflowFix,
				TaskDescription: "auth bug",
			},
			wantArgs: []string{"comments tagged with CMT", "auth bug"},
		},
		{
			name: "skip tracking adds --no-session",
			opts: agent.RunOptions{
				Command:      agent.CommandNew,
				WorkflowType: db.WorkflowGeneral,
				SkipTracking: true,
			},
			wantArgs: []string{"--no-session"},
		},
		{
			name: "implement workflow",
			opts: agent.RunOptions{
				Command:         agent.CommandImplement,
				WorkflowType:    db.WorkflowImplement,
				TaskDescription: "build the feature",
			},
			wantArgs: []string{"/implement_plan implement all phases ignoring any manual verification steps build the feature"},
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

func TestDefaultModel(t *testing.T) {
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
		cmd  agent.CommandType
		want string
	}{
		{agent.CommandNew, "claude-opus-4-6"},
		{agent.CommandQuick, "claude-haiku-4-5"},
		{agent.CommandResearch, "claude-opus-4-6"},
		{agent.CommandImplement, "claude-sonnet-4-6"},
	}

	for _, tt := range tests {
		t.Run(string(tt.cmd), func(t *testing.T) {
			got := runner.DefaultModel(tt.cmd)
			if got != tt.want {
				t.Errorf("DefaultModel(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}
