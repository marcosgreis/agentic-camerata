package cli

import (
	"fmt"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/amp"
	"github.com/agentic-camerata/cmt/internal/claude"
	"github.com/agentic-camerata/cmt/internal/codex"
	"github.com/agentic-camerata/cmt/internal/db"
	"github.com/agentic-camerata/cmt/internal/pi"
)

// newAgent creates an Agent implementation based on the agentType string.
// Valid values are "pi" (default), "claude", "codex", and "amp".
func newAgent(agentType string, database *db.DB) (agent.Agent, error) {
	switch agentType {
	case "pi", "":
		return pi.NewRunner(database)
	case "claude":
		return claude.NewRunner(database)
	case "codex":
		return codex.NewRunner(database)
	case "amp":
		return amp.NewRunner(database)
	default:
		return nil, fmt.Errorf("unknown agent %q (valid: claude, codex, amp, pi)", agentType)
	}
}
