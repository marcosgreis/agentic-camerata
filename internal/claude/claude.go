// Package claude implements the Agent interface for the Claude CLI.
package claude

import (
	"context"
	"os"
	"os/exec"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
	"github.com/agentic-camerata/cmt/internal/runner"
)

// Runner manages Claude CLI execution.
type Runner struct {
	base *runner.Base
}

// Ensure Runner implements agent.Agent at compile time.
var _ agent.Agent = (*Runner)(nil)

// NewRunner creates a new Claude runner.
func NewRunner(database *db.DB) (*Runner, error) {
	base, err := runner.NewBase(database)
	if err != nil {
		return nil, err
	}
	return &Runner{base: base}, nil
}

// Run starts a Claude session.
func (r *Runner) Run(ctx context.Context, opts agent.RunOptions) error {
	// Set comment tag from environment if not already set
	if opts.CommentTag == "" {
		opts.CommentTag = os.Getenv("CMT_COMMENT_TAG")
	}
	cmd := r.buildCommand(opts)
	return r.base.Execute(ctx, cmd, opts)
}

// defaultModels maps command types to the default model for Claude.
var defaultModels = map[agent.CommandType]string{
	agent.CommandNew:        "opus",
	agent.CommandResearch:   "opus",
	agent.CommandPlan:       "opus",
	agent.CommandImplement:  "sonnet",
	agent.CommandFixTest:    "opus",
	agent.CommandLookAndFix: "opus",
	agent.CommandQuick:      "haiku",
	agent.CommandReview:     "opus",
}

// DefaultModel returns the Claude-specific default model for a command type.
func (r *Runner) DefaultModel(cmd agent.CommandType) string {
	if m, ok := defaultModels[cmd]; ok {
		return m
	}
	return "opus"
}

// buildCommand constructs the claude CLI command from the given options.
func (r *Runner) buildCommand(opts agent.RunOptions) *exec.Cmd {
	args := []string{}

	model := opts.Model
	if model == "" {
		model = r.DefaultModel(opts.Command)
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	if opts.PrintMode {
		args = append(args, "-p")
	}

	if opts.AutonomousMode {
		args = append(args, "--dangerously-skip-permissions")
	}

	if opts.ResumeSessionID != "" {
		if opts.ResumeSessionID == "*" {
			args = append(args, "--resume")
		} else {
			args = append(args, "--resume", opts.ResumeSessionID)
		}
	}

	task := opts.TaskDescription
	if prefix := GetPromptPrefix(opts.Command, opts.CommentTag); prefix != "" {
		if task != "" {
			task = prefix + " " + task
		} else {
			task = prefix
		}
	}
	if task != "" {
		args = append(args, task)
	}

	return exec.Command("claude", args...)
}
