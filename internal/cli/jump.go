package cli

import (
	"fmt"

	"github.com/agentic-camerata/cmt/internal/tmux"
)

// JumpCmd jumps to the tmux location where a session was started
type JumpCmd struct {
	Session string `arg:"" help:"Session ID to jump to (or 'last' for most recent)"`
}

// Run executes the jump command
func (c *JumpCmd) Run(cli *CLI) error {
	var sessionID string

	if c.Session == "last" {
		session, err := cli.Database().GetLastSession()
		if err != nil {
			return fmt.Errorf("get last session: %w", err)
		}
		if session == nil {
			return fmt.Errorf("no sessions found")
		}
		sessionID = session.ID
	} else {
		sessionID = c.Session
	}

	session, err := cli.Database().GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	loc := tmux.Location{
		Session: session.TmuxSession,
		Window:  session.TmuxWindow,
		Pane:    session.TmuxPane,
	}

	if err := tmux.JumpTo(loc); err != nil {
		return fmt.Errorf("jump to location: %w", err)
	}

	fmt.Printf("Jumped to %s (session %s)\n", loc.String(), session.ID)
	return nil
}
