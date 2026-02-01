package cli

import (
	"context"

	"github.com/agentic-camerata/cmt/internal/claude"
	"github.com/agentic-camerata/cmt/internal/db"
)

// PlanCmd starts a planning-focused Claude session
type PlanCmd struct {
	FileFlags
	Task string `arg:"" help:"Task or feature to plan"`
}

// Run executes the plan command
func (c *PlanCmd) Run(cli *CLI) error {
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
		Command:         claude.CommandPlan,
		WorkflowType:    db.WorkflowPlan,
		TaskDescription: task,
		AutonomousMode:  cli.Autonomous,
	})
}
