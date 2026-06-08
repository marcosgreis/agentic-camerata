// Package pi implements the Agent interface for the Pi coding agent.
package pi

import (
	"context"
	"os/exec"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
	"github.com/agentic-camerata/cmt/internal/runner"
)

// Runner manages Pi CLI execution.
type Runner struct {
	base *runner.Base
}

// Ensure Runner implements agent.Agent at compile time.
var _ agent.Agent = (*Runner)(nil)

// NewRunner creates a new Pi runner.
func NewRunner(database *db.DB) (*Runner, error) {
	base, err := runner.NewBase(database)
	if err != nil {
		return nil, err
	}
	return &Runner{base: base}, nil
}

// Run starts a Pi session.
func (r *Runner) Run(ctx context.Context, opts agent.RunOptions) error {
	cmd := r.buildCommand(opts)
	return r.base.Execute(ctx, cmd, opts)
}

var opusVersioned = "claude-opus-4-6"

var defaultEfforts = map[agent.CommandType]string{
	agent.CommandNew:              "xhigh",
	agent.CommandResearch:         "xhigh",
	agent.CommandPlan:             "xhigh",
	agent.CommandImplement:        "high",
	agent.CommandFixTest:          "high",
	agent.CommandFixLocalComments: "high",
	agent.CommandFixPRBuild:       "high",
	agent.CommandFixPRComments:    "high",
	agent.CommandQuick:            "normal",
	agent.CommandReview:           "xhigh",
}

// DefaultEffort returns the Pi-specific default effort for a command type.
func (r *Runner) DefaultEffort(cmd agent.CommandType) string {
	if e, ok := defaultEfforts[cmd]; ok {
		return e
	}
	return "xhigh"
}

// defaultModels maps command types to the default model for Pi.
// Pi is provider-agnostic; these defaults use Anthropic models via Pi's model pattern syntax.
var defaultModels = map[agent.CommandType]string{
	agent.CommandNew:              opusVersioned,
	agent.CommandResearch:         opusVersioned,
	agent.CommandPlan:             opusVersioned,
	agent.CommandImplement:        "claude-sonnet-4-6",
	agent.CommandFixTest:          opusVersioned,
	agent.CommandFixLocalComments: opusVersioned,
	agent.CommandFixPRBuild:       opusVersioned,
	agent.CommandFixPRComments:    opusVersioned,
	agent.CommandQuick:            "claude-haiku-4-5",
	agent.CommandReview:           opusVersioned,
}

// DefaultModel returns the Pi-specific default model for a command type.
func (r *Runner) DefaultModel(cmd agent.CommandType) string {
	if m, ok := defaultModels[cmd]; ok {
		return m
	}
	return opusVersioned
}

// buildCommand constructs the pi CLI command from the given options.
func (r *Runner) buildCommand(opts agent.RunOptions) *exec.Cmd {
	args := []string{}

	effort := opts.Effort
	if effort == "" {
		effort = r.DefaultEffort(opts.Command)
	}
	if effort != "" {
		args = append(args, "--thinking", effort)
	}

	model := opts.Model
	if model == "" {
		model = r.DefaultModel(opts.Command)
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	// Pi has no permission popups, so no autonomous flag needed.

	if opts.PrintMode {
		args = append(args, "-p")
	}

	// Session resume support
	if opts.ResumeSessionID != "" {
		if opts.ResumeSessionID == "*" {
			args = append(args, "--resume")
		} else {
			args = append(args, "--session", opts.ResumeSessionID)
		}
	}

	// Skip session persistence for ephemeral commands
	if opts.SkipTracking {
		args = append(args, "--no-session")
	}

	// Build task with prompt prefix (uses underscore slash commands, same as Claude)
	taskDescription := agent.ApplyPromptPrefix(opts.Command, opts.TaskDescription, opts.CommentTag)
	if taskDescription != "" {
		args = append(args, taskDescription)
	}

	return exec.Command("pi", args...)
}
