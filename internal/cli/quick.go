package cli

import (
	"context"

	"github.com/agentic-camerata/cmt/internal/claude"
	"github.com/agentic-camerata/cmt/internal/db"
)

// QuickCmd runs a quick single-response query using Sonnet
type QuickCmd struct {
	Prompt string `arg:"" help:"Prompt for Claude"`
}

// Run executes the quick command
func (c *QuickCmd) Run(cli *CLI) error {
	runner, err := claude.NewRunner(cli.Database())
	if err != nil {
		return err
	}

	return runner.Run(context.Background(), claude.RunOptions{
		Command:         claude.CommandQuick,
		WorkflowType:    db.WorkflowGeneral,
		TaskDescription: c.Prompt,
		Model:           "haiku",
		PrintMode:       true,
		AutonomousMode:  cli.Autonomous,
	})
}
