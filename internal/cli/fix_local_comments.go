package cli

import (
	"context"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
)

// FixLocalCommentsCmd starts a session to look at an issue and fix it
type FixLocalCommentsCmd struct {
	FileFlags
	LoopFlags
	CommentTag string `help:"Comment tag to search for" env:"CMT_COMMENT_TAG" optional:""`
	Issue      string `arg:"" help:"Issue or problem to investigate and fix"`
}

// Run executes the fix-local-comments command
func (c *FixLocalCommentsCmd) Run(cli *CLI) error {
	files, err := c.FileFlags.ResolveFiles()
	if err != nil {
		return err
	}

	issue := PrependFilesToTask(files, c.Issue)

	ag, err := newAgent(cli.Agent, cli.Database())
	if err != nil {
		return err
	}

	ctx := context.Background()
	return RunWithLoop(ctx, c.Interval, c.Limit, func() error {
		return ag.Run(ctx, agent.RunOptions{
			Command:         agent.CommandFixLocalComments,
			WorkflowType:    db.WorkflowFix,
			TaskDescription: issue,
			Model:           cli.Model,
			AutonomousMode:  cli.Autonomous,
			CommentTag:      c.CommentTag,
			LoopInterval:    c.Interval,
		})
	})
}
