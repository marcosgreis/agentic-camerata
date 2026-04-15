package cli

import (
	"context"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
)

// ResearchCmd starts a research-focused agent session
type ResearchCmd struct {
	FileFlags
	LoopFlags
	Topic string `arg:"" help:"Topic or area to research"`
}

// Run executes the research command
func (c *ResearchCmd) Run(cli *CLI) error {
	files, err := c.FileFlags.ResolveFiles()
	if err != nil {
		return err
	}

	topic := PrependFilesToTask(files, c.Topic)

	ag, err := newAgent(cli.Agent, cli.Database())
	if err != nil {
		return err
	}

	ctx := context.Background()
	return RunWithLoop(ctx, c.Interval, c.Limit, func(interrupted *bool) error {
		return ag.Run(ctx, agent.RunOptions{
			Command:         agent.CommandResearch,
			WorkflowType:    db.WorkflowResearch,
			TaskDescription: topic,
			Model:           cli.Model,
			AutonomousMode:  cli.Autonomous,
			LoopInterval:    c.Interval,
			AutoTerminate:   c.Interval != "",
			Interrupted:     interrupted,
		})
	})
}
