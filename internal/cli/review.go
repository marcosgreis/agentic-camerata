package cli

import (
	"context"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
)

// ReviewCmd starts a session to review changes in the working directory
type ReviewCmd struct {
	FileFlags
	Focus string `arg:"" optional:"" help:"Optional focus area or context for the review"`
}

// Run executes the review command
func (c *ReviewCmd) Run(cli *CLI) error {
	files, err := c.FileFlags.ResolveFiles()
	if err != nil {
		return err
	}

	focus := PrependFilesToTask(files, c.Focus)

	ag, err := newAgent(cli.Agent, cli.Database())
	if err != nil {
		return err
	}

	return ag.Run(context.Background(), agent.RunOptions{
		Command:         agent.CommandReview,
		WorkflowType:    db.WorkflowReview,
		TaskDescription: focus,
		Model:           cli.Model,
		AutonomousMode:  cli.Autonomous,
	})
}
