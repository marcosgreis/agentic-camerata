// Package codex implements the Agent interface for the OpenAI Codex CLI.
package codex

import (
	"context"
	"os/exec"
	"strings"

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

	// Codex uses -a never to skip approval prompts.
	if opts.AutonomousMode {
		args = append(args, "-a", "never")
	}

	// Codex uses -q (quiet) for non-interactive single-response mode.
	if opts.PrintMode {
		args = append(args, "-q")
	}

	taskDescription := applyPromptPrefix(opts.Command, opts.TaskDescription, opts.CommentTag)
	if taskDescription != "" {
		args = append(args, taskDescription)
	}

	return exec.Command("codex", args...)
}

func applyPromptPrefix(cmd agent.CommandType, taskDescription, commentTag string) string {
	prefix := agent.GetPromptPrefix(cmd, commentTag)
	if prefix == "" {
		return taskDescription
	}

	prefix = hyphenateSlashCommand(prefix)
	if taskDescription == "" {
		return prefix
	}
	return prefix + " " + taskDescription
}

func hyphenateSlashCommand(prefix string) string {
	if !strings.HasPrefix(prefix, "/") {
		return prefix
	}

	command, rest, found := strings.Cut(prefix, " ")
	command = strings.ReplaceAll(command, "_", "-")
	if !found {
		return command
	}
	return command + " " + rest
}
