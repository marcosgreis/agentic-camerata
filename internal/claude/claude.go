// Package claude implements the Agent interface for the Claude CLI.
package claude

import (
	"context"
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
	cmd := r.buildCommand(opts)
	return r.base.Execute(ctx, cmd, opts)
}

var opusVersioned = "claude-opus-4-5-20251101"

// defaultModels maps command types to the default model for Claude.
var defaultModels = map[agent.CommandType]string{
	agent.CommandNew:              opusVersioned,
	agent.CommandResearch:         opusVersioned,
	agent.CommandPlan:             opusVersioned,
	agent.CommandImplement:        "sonnet",
	agent.CommandFixTest:          opusVersioned,
	agent.CommandFixLocalComments: opusVersioned,
	agent.CommandFixPRBuild:       opusVersioned,
	agent.CommandFixPRComments:    opusVersioned,
	agent.CommandQuick:            "haiku",
	agent.CommandReview:           opusVersioned,
}

// DefaultModel returns the Claude-specific default model for a command type.
func (r *Runner) DefaultModel(cmd agent.CommandType) string {
	if m, ok := defaultModels[cmd]; ok {
		return m
	}
	return opusVersioned
}

// buildCommand constructs the claude CLI command from the given options.
func (r *Runner) buildCommand(opts agent.RunOptions) *exec.Cmd {
	args := []string{}
	// hardcoded max effort to make sure it respects this option
	args = append(args, "--effort", "max")

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

	taskDescription := agent.ApplyPromptPrefix(opts.Command, opts.TaskDescription, opts.CommentTag)
	if taskDescription != "" {
		args = append(args, taskDescription)
	}

	return exec.Command("claude", args...)
}
