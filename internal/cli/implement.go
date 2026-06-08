package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
	"github.com/agentic-camerata/cmt/internal/plans"
)

// ImplementCmd starts an implementation-focused agent session
type ImplementCmd struct {
	LoopFlags
	Dir  string `short:"d" name:"dir" default:"thoughts/shared/plans" help:"Directory to list plans from in the fzf selector"`
	Plan string `arg:"" optional:"" help:"Path to plan file (uses fzf selector if not provided)"`
}

// Run executes the implement command
func (c *ImplementCmd) Run(cli *CLI) error {
	planPath := c.Plan
	if planPath == "" {
		var err error
		planPath, err = plans.SelectMarkdownFile(c.Dir)
		if err != nil {
			return err
		}
	}

	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		return fmt.Errorf("plan file not found: %s", planPath)
	}

	task := planPath

	ag, err := newAgent(cli.Agent, cli.Database())
	if err != nil {
		return err
	}

	ctx := context.Background()
	return RunWithLoop(ctx, c.Interval, c.Limit, func(interrupted *bool) error {
		return ag.Run(ctx, agent.RunOptions{
			Command:         agent.CommandImplement,
			WorkflowType:    db.WorkflowImplement,
			TaskDescription: task,
			Model:           cli.Model,
			Effort:          cli.Effort,
			AutonomousMode:  cli.Autonomous,
			LoopInterval:    c.Interval,
			AutoTerminate:   c.Interval != "",
			Interrupted:     interrupted,
		})
	})
}
