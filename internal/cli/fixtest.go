package cli

import (
	"context"

	"github.com/agentic-camerata/cmt/internal/claude"
	"github.com/agentic-camerata/cmt/internal/db"
)

// FixTestCmd starts a session focused on fixing a failing test
type FixTestCmd struct {
	FileFlags
	Test string `arg:"" help:"Test name or description of the failing test"`
}

// Run executes the fix-test command
func (c *FixTestCmd) Run(cli *CLI) error {
	// Resolve file flags
	files, err := c.FileFlags.ResolveFiles()
	if err != nil {
		return err
	}

	// Prepend files to test
	test := PrependFilesToTask(files, c.Test)

	runner, err := claude.NewRunner(cli.Database())
	if err != nil {
		return err
	}

	return runner.Run(context.Background(), claude.RunOptions{
		Command:         claude.CommandFixTest,
		WorkflowType:    db.WorkflowFix,
		TaskDescription: test,
		AutonomousMode:  cli.Autonomous,
	})
}
