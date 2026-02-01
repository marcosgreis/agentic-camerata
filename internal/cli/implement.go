package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/agentic-camerata/cmt/internal/claude"
	"github.com/agentic-camerata/cmt/internal/db"
	"github.com/agentic-camerata/cmt/internal/plans"
)

// ImplementCmd starts an implementation-focused Claude session
type ImplementCmd struct {
	FileFlags
	Plan string `arg:"" optional:"" help:"Path to plan file (uses fzf selector if not provided)"`
}

// Run executes the implement command
func (c *ImplementCmd) Run(cli *CLI) error {
	// Get plan file path (this is the main plan, not from -f)
	planPath := c.Plan
	if planPath == "" {
		var err error
		planPath, err = plans.SelectPlanFile()
		if err != nil {
			return err
		}
	}

	// Verify plan file exists
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		return fmt.Errorf("plan file not found: %s", planPath)
	}

	// Resolve additional file flags
	files, err := c.FileFlags.ResolveFiles()
	if err != nil {
		return err
	}

	// Prepend additional files to the plan path
	task := PrependFilesToTask(files, planPath)

	runner, err := claude.NewRunner(cli.Database())
	if err != nil {
		return err
	}

	return runner.Run(context.Background(), claude.RunOptions{
		Command:         claude.CommandImplement,
		WorkflowType:    db.WorkflowImplement,
		TaskDescription: task,
		AutonomousMode:  cli.Autonomous,
	})
}
