package cli

import (
	"context"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
)

// NewCmd starts a new general agent session
type NewCmd struct {
	FileFlags
	Resume   bool   `short:"r" help:"Resume a previous Claude session (interactive picker)"`
	ResumeID string `help:"Resume a specific Claude session by ID" name:"resume-id"`
	Task     string `arg:"" optional:"" help:"Initial task or prompt for Claude"`
}

// Run executes the new command
func (c *NewCmd) Run(cli *CLI) error {
	files, err := c.FileFlags.ResolveFiles()
	if err != nil {
		return err
	}

	task := PrependFilesToTask(files, c.Task)

	ag, err := newAgent(cli.Agent, cli.Database())
	if err != nil {
		return err
	}

	var resumeID string
	if c.ResumeID != "" {
		resumeID = c.ResumeID
	} else if c.Resume {
		resumeID = "*"
	}

	return ag.Run(context.Background(), agent.RunOptions{
		Command:         agent.CommandNew,
		WorkflowType:    db.WorkflowGeneral,
		TaskDescription: task,
		Model:           cli.Model,
		AutonomousMode:  cli.Autonomous,
		ResumeSessionID: resumeID,
	})
}
