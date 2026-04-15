package cli

import (
	"context"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
)

// PlanCmd starts a planning-focused agent session
type PlanCmd struct {
	FileFlags
	LoopFlags
	Task string `arg:"" help:"Task or feature to plan"`
}

// Run executes the plan command
func (c *PlanCmd) Run(cli *CLI) error {
	files, err := c.FileFlags.ResolveFiles()
	if err != nil {
		return err
	}

	task := PrependFilesToTask(files, c.Task)

	ag, err := newAgent(cli.Agent, cli.Database())
	if err != nil {
		return err
	}

	ctx := context.Background()
	return RunWithLoop(ctx, c.Interval, c.Limit, func() error {
		return ag.Run(ctx, agent.RunOptions{
			Command:         agent.CommandPlan,
			WorkflowType:    db.WorkflowPlan,
			TaskDescription: task,
			Model:           cli.Model,
			AutonomousMode:  cli.Autonomous,
			LoopInterval:    c.Interval,
		})
	})
}
