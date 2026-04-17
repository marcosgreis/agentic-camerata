// Package amp implements the Agent interface for the Sourcegraph Amp CLI.
package amp

import (
	"context"
	"os/exec"
	"time"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
	"github.com/agentic-camerata/cmt/internal/runner"
)

// Runner manages Amp CLI execution.
type Runner struct {
	base *runner.Base
}

const initialInputDelay = 4 * time.Second

// Ensure Runner implements agent.Agent at compile time.
var _ agent.Agent = (*Runner)(nil)

// NewRunner creates a new Amp runner.
func NewRunner(database *db.DB) (*Runner, error) {
	base, err := runner.NewBase(database)
	if err != nil {
		return nil, err
	}
	return &Runner{base: base}, nil
}

// Run starts an Amp session.
func (r *Runner) Run(ctx context.Context, opts agent.RunOptions) error {
	execOpts := prepareRunOptions(opts)
	cmd := r.buildCommand(execOpts)
	return r.base.Execute(ctx, cmd, execOpts)
}

func prepareRunOptions(opts agent.RunOptions) agent.RunOptions {
	execOpts := opts
	if !opts.PrintMode {
		execOpts.InitialInput = agent.ApplyPromptPrefix(opts.Command, opts.TaskDescription, opts.CommentTag)
		execOpts.InitialInputDelay = initialInputDelay
	}
	return execOpts
}

// DefaultModel returns the Amp-specific default model for a command type.
// Returns "" because Amp manages model selection via modes (smart/rush/deep)
// in its settings rather than per-invocation CLI flags.
func (r *Runner) DefaultModel(cmd agent.CommandType) string {
	return ""
}

// buildCommand constructs the amp CLI command from the given options.
func (r *Runner) buildCommand(opts agent.RunOptions) *exec.Cmd {
	args := []string{}

	// Amp uses --dangerously-allow-all to skip all command confirmation prompts.
	if opts.AutonomousMode {
		args = append(args, "--dangerously-allow-all")
	}

	// Handle thread resume as a subcommand: amp threads continue [id]
	if opts.ResumeSessionID != "" {
		args = append(args, "threads", "continue")
		if opts.ResumeSessionID != "*" {
			args = append(args, opts.ResumeSessionID)
		}
		return exec.Command("amp", args...)
	}

	// Amp uses -x (--execute) for non-interactive single-response mode.
	taskDescription := agent.ApplyPromptPrefix(opts.Command, opts.TaskDescription, opts.CommentTag)
	if opts.PrintMode {
		if taskDescription != "" {
			args = append(args, "-x", taskDescription)
		} else {
			args = append(args, "-x")
		}
	}

	return exec.Command("amp", args...)
}
