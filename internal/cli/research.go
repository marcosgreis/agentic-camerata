package cli

import (
	"context"

	"github.com/agentic-camerata/cmt/internal/claude"
	"github.com/agentic-camerata/cmt/internal/db"
)

// ResearchCmd starts a research-focused Claude session
type ResearchCmd struct {
	FileFlags
	Topic string `arg:"" help:"Topic or area to research"`
}

// Run executes the research command
func (c *ResearchCmd) Run(cli *CLI) error {
	// Resolve file flags
	files, err := c.FileFlags.ResolveFiles()
	if err != nil {
		return err
	}

	// Prepend files to topic
	topic := PrependFilesToTask(files, c.Topic)

	runner, err := claude.NewRunner(cli.Database())
	if err != nil {
		return err
	}

	return runner.Run(context.Background(), claude.RunOptions{
		Command:         claude.CommandResearch,
		WorkflowType:    db.WorkflowResearch,
		TaskDescription: topic,
		AutonomousMode:  cli.Autonomous,
	})
}
