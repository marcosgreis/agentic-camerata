package cli

import (
	"context"

	"github.com/agentic-camerata/cmt/internal/claude"
	"github.com/agentic-camerata/cmt/internal/db"
)

// NewCmd starts a new Claude session
type NewCmd struct {
	FileFlags
	Resume   bool   `short:"r" help:"Resume a previous Claude session (interactive picker)"`
	ResumeID string `help:"Resume a specific Claude session by ID" name:"resume-id"`
	Task     string `arg:"" optional:"" help:"Initial task or prompt for Claude"`
}

// Run executes the new command
func (c *NewCmd) Run(cli *CLI) error {
	// Resolve file flags
	files, err := c.FileFlags.ResolveFiles()
	if err != nil {
		return err
	}

	// Prepend files to task
	task := PrependFilesToTask(files, c.Task)

	runner, err := claude.NewRunner(cli.Database())
	if err != nil {
		return err
	}

	// Determine resume session ID
	var resumeID string
	if c.ResumeID != "" {
		resumeID = c.ResumeID
	} else if c.Resume {
		resumeID = "*"
	}

	return runner.Run(context.Background(), claude.RunOptions{
		Command:         claude.CommandNew,
		WorkflowType:    db.WorkflowGeneral,
		TaskDescription: task,
		AutonomousMode:  cli.Autonomous,
		ResumeSessionID: resumeID,
	})
}
