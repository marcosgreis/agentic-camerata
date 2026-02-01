package cli

import (
	"context"

	"github.com/agentic-camerata/cmt/internal/claude"
	"github.com/agentic-camerata/cmt/internal/db"
)

// NewCmd starts a new Claude session
type NewCmd struct {
	FileFlags
	Task string `arg:"" optional:"" help:"Initial task or prompt for Claude"`
}

// Run executes the new command
func (c *NewCmd) Run(cli *CLI) error {
	// Resolve file flags
	files, err := c.FileFlags.ResolveFiles()
	if err != nil {
		return err
	}

	// Prepend files to task
	task := PrependFilesToTask(files, c.Task)

	runner, err := claude.NewRunner(cli.Database())
	if err != nil {
		return err
	}

	return runner.Run(context.Background(), claude.RunOptions{
		Command:         claude.CommandNew,
		WorkflowType:    db.WorkflowGeneral,
		TaskDescription: task,
		AutonomousMode:  cli.Autonomous,
	})
}
