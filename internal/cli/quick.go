package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
)

// QuickCmd runs a quick single-response query
type QuickCmd struct {
	Prompt string `arg:"" help:"Prompt for the agent"`
}

// Run executes the quick command
func (c *QuickCmd) Run(cli *CLI) error {
	prompt := c.Prompt
	if prompt == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
		prompt = string(data)
	}

	ag, err := newAgent(cli.Agent, cli.Database())
	if err != nil {
		return err
	}

	return ag.Run(context.Background(), agent.RunOptions{
		Command:         agent.CommandQuick,
		WorkflowType:    db.WorkflowGeneral,
		TaskDescription: prompt,
		Model:           cli.Model,
		PrintMode:       true,
		AutonomousMode:  cli.Autonomous,
		SkipTracking:    true,
	})
}
