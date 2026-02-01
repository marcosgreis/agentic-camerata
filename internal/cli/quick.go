package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/agentic-camerata/cmt/internal/claude"
	"github.com/agentic-camerata/cmt/internal/db"
)

// QuickCmd runs a quick single-response query using Sonnet
type QuickCmd struct {
	Prompt string `arg:"" help:"Prompt for Claude"`
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

	runner, err := claude.NewRunner(cli.Database())
	if err != nil {
		return err
	}

	return runner.Run(context.Background(), claude.RunOptions{
		Command:         claude.CommandQuick,
		WorkflowType:    db.WorkflowGeneral,
		TaskDescription: prompt,
		Model:           "haiku",
		PrintMode:       true,
		AutonomousMode:  cli.Autonomous,
	})
}
