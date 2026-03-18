package cli

import (
	"context"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
)

// LookAndFixCmd starts a session to look at an issue and fix it
type LookAndFixCmd struct {
	FileFlags
	CommentTag string `help:"Comment tag to search for" env:"CMT_COMMENT_TAG" optional:""`
	Issue      string `arg:"" help:"Issue or problem to investigate and fix"`
}

// Run executes the look-and-fix command
func (c *LookAndFixCmd) Run(cli *CLI) error {
	files, err := c.FileFlags.ResolveFiles()
	if err != nil {
		return err
	}

	issue := PrependFilesToTask(files, c.Issue)

	ag, err := newAgent(cli.Agent, cli.Database())
	if err != nil {
		return err
	}

	return ag.Run(context.Background(), agent.RunOptions{
		Command:         agent.CommandLookAndFix,
		WorkflowType:    db.WorkflowFix,
		TaskDescription: issue,
		Model:           cli.Model,
		AutonomousMode:  cli.Autonomous,
		CommentTag:      c.CommentTag,
	})
}
