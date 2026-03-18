// Package codex implements the Agent interface for the OpenAI Codex CLI.
package codex

import (
	"context"
	"os/exec"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
	"github.com/agentic-camerata/cmt/internal/runner"
)

// Runner manages Codex CLI execution.
type Runner struct {
	base *runner.Base
}

// Ensure Runner implements agent.Agent at compile time.
var _ agent.Agent = (*Runner)(nil)

// NewRunner creates a new Codex runner.
func NewRunner(database *db.DB) (*Runner, error) {
	base, err := runner.NewBase(database)
	if err != nil {
		return nil, err
	}
	return &Runner{base: base}, nil
}

// Run starts a Codex session.
func (r *Runner) Run(ctx context.Context, opts agent.RunOptions) error {
	cmd := r.buildCommand(opts)
	return r.base.Execute(ctx, cmd, opts)
}

// DefaultModel returns the Codex-specific default model for a command type.
// Returns "" to let the codex CLI use its own built-in default.
func (r *Runner) DefaultModel(cmd agent.CommandType) string {
	return ""
}

// buildCommand constructs the codex CLI command from the given options.
func (r *Runner) buildCommand(opts agent.RunOptions) *exec.Cmd {
	args := []string{}

	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}

	// Codex uses --full-auto for low-friction sandboxed automatic execution.
	if opts.AutonomousMode {
		args = append(args, "--full-auto")
	}

	// Codex uses -q (quiet) for non-interactive single-response mode.
	if opts.PrintMode {
		args = append(args, "-q")
	}

	if opts.TaskDescription != "" {
		args = append(args, opts.TaskDescription)
	}

	return exec.Command("codex", args...)
}
