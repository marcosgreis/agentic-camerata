package cli

import (
	"context"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
)

// FixPRBuildCmd starts a session focused on fixing a PR's CI build
type FixPRBuildCmd struct {
	FileFlags
	LoopFlags
	PRLink string `arg:"" help:"Link to the pull request"`
}

// Run executes the fix-pr-build command
func (c *FixPRBuildCmd) Run(cli *CLI) error {
	files, err := c.FileFlags.ResolveFiles()
	if err != nil {
		return err
	}

	prLink := PrependFilesToTask(files, c.PRLink)

	ag, err := newAgent(cli.Agent, cli.Database())
	if err != nil {
		return err
	}

	ctx := context.Background()
	return RunWithLoop(ctx, c.Interval, c.Limit, func(interrupted *bool) error {
		return ag.Run(ctx, agent.RunOptions{
			Command:         agent.CommandFixPRBuild,
			WorkflowType:    db.WorkflowFix,
			TaskDescription: prLink,
			Model:           cli.Model,
			AutonomousMode:  cli.Autonomous,
			LoopInterval:    c.Interval,
			AutoTerminate:   c.Interval != "",
			Interrupted:     interrupted,
		})
	})
}
