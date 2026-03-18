package cli

import (
	"context"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
)

// FixTestCmd starts a session focused on fixing a failing test
type FixTestCmd struct {
	FileFlags
	Test string `arg:"" help:"Test name or description of the failing test"`
}

// Run executes the fix-test command
func (c *FixTestCmd) Run(cli *CLI) error {
	files, err := c.FileFlags.ResolveFiles()
	if err != nil {
		return err
	}

	test := PrependFilesToTask(files, c.Test)

	ag, err := newAgent(cli.Agent, cli.Database())
	if err != nil {
		return err
	}

	return ag.Run(context.Background(), agent.RunOptions{
		Command:         agent.CommandFixTest,
		WorkflowType:    db.WorkflowFix,
		TaskDescription: test,
		Model:           cli.Model,
		AutonomousMode:  cli.Autonomous,
	})
}
