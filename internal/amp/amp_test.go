package amp

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
			name: "print mode preserves prefixed task",
			opts: agent.RunOptions{
				Command:         agent.CommandPlan,
				WorkflowType:    db.WorkflowPlan,
				TaskDescription: "checkout flow",
				PrintMode:       true,
			},
			wantArgs: []string{"-x", "/create_plan checkout flow"},
		},
		{
			name: "interactive mode does not pass task as positional arg",
			opts: agent.RunOptions{
				Command:         agent.CommandNew,
				WorkflowType:    db.WorkflowGeneral,
				TaskDescription: "test task",
			},
			notWantArgs: []string{"test task"},
		},
		{
			name: "resume keeps autonomous flag",
			opts: agent.RunOptions{
				Command:         agent.CommandNew,
				WorkflowType:    db.WorkflowGeneral,
				ResumeSessionID: "*",
				AutonomousMode:  true,
			},
			wantArgs: []string{"--dangerously-allow-all", "threads", "continue"},
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

func TestPrepareRunOptions(t *testing.T) {
	tests := []struct {
		name             string
		opts             agent.RunOptions
		wantInitialInput string
	}{
		{
			name: "research workflow prefixes initial input",
			opts: agent.RunOptions{
				Command:         agent.CommandResearch,
				WorkflowType:    db.WorkflowResearch,
				TaskDescription: "test topic",
			},
			wantInitialInput: "/research_codebase test topic",
		},
		{
			name: "fix-local-comments includes default tag in initial input",
			opts: agent.RunOptions{
				Command:         agent.CommandFixLocalComments,
				WorkflowType:    db.WorkflowFix,
				TaskDescription: "auth bug",
			},
			wantInitialInput: "Take a look at this repo and search for comments tagged with CMT and propose how to solve them. If a class name or filename is provided as a parameter, focus the search on that specific file or class. auth bug",
		},
		{
			name: "print mode leaves initial input empty",
			opts: agent.RunOptions{
				Command:         agent.CommandPlan,
				WorkflowType:    db.WorkflowPlan,
				TaskDescription: "checkout flow",
				PrintMode:       true,
			},
			wantInitialInput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prepareRunOptions(tt.opts)
			if got.InitialInput != tt.wantInitialInput {
				t.Fatalf("prepareRunOptions(%+v).InitialInput = %q, want %q", tt.opts, got.InitialInput, tt.wantInitialInput)
			}
			if tt.wantInitialInput != "" && got.InitialInputDelay != initialInputDelay {
				t.Fatalf("prepareRunOptions(%+v).InitialInputDelay = %v, want %v", tt.opts, got.InitialInputDelay, initialInputDelay)
			}
			if tt.wantInitialInput == "" && got.InitialInputDelay != 0 {
				t.Fatalf("prepareRunOptions(%+v).InitialInputDelay = %v, want 0", tt.opts, got.InitialInputDelay)
			}
		})
	}
}
