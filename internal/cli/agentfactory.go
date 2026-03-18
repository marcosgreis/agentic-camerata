package cli

import (
	"fmt"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/claude"
	"github.com/agentic-camerata/cmt/internal/codex"
	"github.com/agentic-camerata/cmt/internal/db"
)

// newAgent creates an Agent implementation based on the agentType string.
// Valid values are "claude" (default) and "codex".
func newAgent(agentType string, database *db.DB) (agent.Agent, error) {
	switch agentType {
	case "claude", "":
		return claude.NewRunner(database)
	case "codex":
		return codex.NewRunner(database)
	default:
		return nil, fmt.Errorf("unknown agent %q (valid: claude, codex)", agentType)
	}
}
