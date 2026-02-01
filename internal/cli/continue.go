package cli

import (
	"context"

	"github.com/agentic-camerata/cmt/internal/claude"
)

// ContinueCmd continues an existing Claude session
type ContinueCmd struct {
	Session string `arg:"" default:"last" help:"Session ID to continue (or 'last' for most recent)"`
}

// Run executes the continue command
func (c *ContinueCmd) Run(cli *CLI) error {
	runner, err := claude.NewRunner(cli.Database())
	if err != nil {
		return err
	}

	return runner.Continue(context.Background(), c.Session, cli.Autonomous)
}
