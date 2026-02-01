package cli

import (
	"context"

	"github.com/agentic-camerata/cmt/internal/claude"
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
	// Resolve file flags
	files, err := c.FileFlags.ResolveFiles()
	if err != nil {
		return err
	}

	// Prepend files to issue
	issue := PrependFilesToTask(files, c.Issue)

	runner, err := claude.NewRunner(cli.Database())
	if err != nil {
		return err
	}

	return runner.Run(context.Background(), claude.RunOptions{
		Command:         claude.CommandLookAndFix,
		WorkflowType:    db.WorkflowFix,
		TaskDescription: issue,
		AutonomousMode:  cli.Autonomous,
		CommentTag:      c.CommentTag,
	})
}
