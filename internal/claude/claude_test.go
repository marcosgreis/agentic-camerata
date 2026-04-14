package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
)

func TestGetPromptPrefix(t *testing.T) {
	tests := []struct {
		name       string
		command    agent.CommandType
		commentTag string
		wantPrefix string
	}{
		{
			name:       "new command has no prefix",
			command:    agent.CommandNew,
			wantPrefix: "",
		},
		{
			name:       "research command has skill prefix",
			command:    agent.CommandResearch,
			wantPrefix: "/research_codebase",
		},
		{
			name:       "plan command has skill prefix",
			command:    agent.CommandPlan,
			wantPrefix: "/create_plan",
		},
		{
			name:       "implement command has skill prefix",
			command:    agent.CommandImplement,
			wantPrefix: "/implement_plan implement all phases ignoring any manual verification steps",
		},
		{
			name:       "fix-test command has instruction prefix",
			command:    agent.CommandFixTest,
			wantPrefix: "Analyze and fix the failing test at:",
		},
		{
			name:       "fix-local-comments command has instruction prefix",
			command:    agent.CommandFixLocalComments,
			wantPrefix: "Take a look at this repo and search for comments tagged with CMT and propose how to solve them. If a class name or filename is provided as a parameter, focus the search on that specific file or class.",
		},
		{
			name:       "quick command has no prefix",
			command:    agent.CommandQuick,
			wantPrefix: "",
		},
		{
			name:       "unknown command has no prefix",
			command:    agent.CommandType("unknown"),
			wantPrefix: "",
		},
		{
			name:       "fix-local-comments with custom tag",
			command:    agent.CommandFixLocalComments,
			commentTag: "TODO",
			wantPrefix: "Take a look at this repo and search for comments tagged with TODO and propose how to solve them. If a class name or filename is provided as a parameter, focus the search on that specific file or class.",
		},
		{
			name:       "fix-local-comments with empty tag defaults to CMT",
			command:    agent.CommandFixLocalComments,
			commentTag: "",
			wantPrefix: "Take a look at this repo and search for comments tagged with CMT and propose how to solve them. If a class name or filename is provided as a parameter, focus the search on that specific file or class.",
		},
		{
			name:       "non-fix-local-comments ignores commentTag",
			command:    agent.CommandResearch,
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
		opts        agent.RunOptions
		wantArgs    []string
		notWantArgs []string
	}{
		{
			name: "general workflow no prompt",
			opts: agent.RunOptions{
				Command:      agent.CommandNew,
				WorkflowType: db.WorkflowGeneral,
			},
			notWantArgs: []string{"--system-prompt"},
		},
		{
			name: "research workflow",
			opts: agent.RunOptions{
				Command:         agent.CommandResearch,
				WorkflowType:    db.WorkflowResearch,
				TaskDescription: "test topic",
			},
			wantArgs:    []string{"/research_codebase test topic"},
			notWantArgs: []string{"--system-prompt"},
		},
		{
			name: "with task description",
			opts: agent.RunOptions{
				Command:         agent.CommandNew,
				WorkflowType:    db.WorkflowGeneral,
				TaskDescription: "test task",
			},
			wantArgs:    []string{"test task"},
			notWantArgs: []string{"--prompt", "-p"},
		},
		{
			name: "autonomous mode enabled",
			opts: agent.RunOptions{
				Command:        agent.CommandNew,
				WorkflowType:   db.WorkflowGeneral,
				AutonomousMode: true,
			},
			wantArgs: []string{"--dangerously-skip-permissions"},
		},
		{
			name: "autonomous mode with task",
			opts: agent.RunOptions{
				Command:         agent.CommandNew,
				WorkflowType:    db.WorkflowGeneral,
				TaskDescription: "test task",
				AutonomousMode:  true,
			},
			wantArgs: []string{"--dangerously-skip-permissions", "test task"},
		},
		{
			name: "autonomous mode disabled by default",
			opts: agent.RunOptions{
				Command:         agent.CommandNew,
				WorkflowType:    db.WorkflowGeneral,
				TaskDescription: "test task",
			},
			notWantArgs: []string{"--dangerously-skip-permissions"},
		},
		{
			name: "fix-local-comments with custom comment tag",
			opts: agent.RunOptions{
				Command:         agent.CommandFixLocalComments,
				WorkflowType:    db.WorkflowFix,
				TaskDescription: "auth bug",
				CommentTag:      "FIXME",
			},
			wantArgs:    []string{"comments tagged with FIXME"},
			notWantArgs: []string{"comments tagged with CMT"},
		},
		{
			name: "fix-local-comments defaults to CMT tag",
			opts: agent.RunOptions{
				Command:         agent.CommandFixLocalComments,
				WorkflowType:    db.WorkflowFix,
				TaskDescription: "auth bug",
			},
			wantArgs: []string{"comments tagged with CMT"},
		},
		{
			name: "resume interactive picker",
			opts: agent.RunOptions{
				Command:         agent.CommandNew,
				WorkflowType:    db.WorkflowGeneral,
				ResumeSessionID: "*",
			},
			wantArgs:    []string{"--resume"},
			notWantArgs: []string{"--resume *"},
		},
		{
			name: "resume specific session",
			opts: agent.RunOptions{
				Command:         agent.CommandNew,
				WorkflowType:    db.WorkflowGeneral,
				ResumeSessionID: "abc123",
			},
			wantArgs: []string{"--resume", "abc123"},
		},
		{
			name: "no resume by default",
			opts: agent.RunOptions{
				Command:      agent.CommandNew,
				WorkflowType: db.WorkflowGeneral,
			},
			notWantArgs: []string{"--resume"},
		},
		{
			name: "resume with task description",
			opts: agent.RunOptions{
				Command:         agent.CommandNew,
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

