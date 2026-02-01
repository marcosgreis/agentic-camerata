package cli

import (
	"github.com/agentic-camerata/cmt/internal/db"
)

// CLI is the root command structure for cmt
type CLI struct {
	// Commands
	New        NewCmd        `cmd:"" help:"Start a new Claude session"`
	Research   ResearchCmd   `cmd:"" help:"Start a research-focused session"`
	Plan       PlanCmd       `cmd:"" help:"Start a planning session"`
	Implement  ImplementCmd  `cmd:"" help:"Start an implementation session"`
	FixTest    FixTestCmd    `cmd:"fix-test" help:"Fix a failing test"`
	LookAndFix LookAndFixCmd `cmd:"look-and-fix" help:"Look at an issue and fix it"`
	Quick      QuickCmd      `cmd:"" help:"Quick single-response query (uses Sonnet)"`
	Sessions   SessionsCmd   `cmd:"" help:"List all sessions"`
	Jump       JumpCmd       `cmd:"" help:"Jump to a session's tmux location"`
	Dashboard  DashboardCmd  `cmd:"" help:"Open the TUI dashboard"`

	// Global flags
	DB         string `help:"Database path" default:"~/.config/cmt/sessions.db" env:"CMT_DB" optional:""`
	Verbose    bool   `short:"v" help:"Enable verbose output"`
	Autonomous bool   `short:"a" help:"Enable autonomous mode (skip permission prompts)" env:"CMT_AUTONOMOUS"`

	// Shared state (populated by Run)
	database *db.DB
}

// Database returns the database connection
func (c *CLI) Database() *db.DB {
	return c.database
}

// SetDatabase sets the database connection
func (c *CLI) SetDatabase(database *db.DB) {
	c.database = database
}
