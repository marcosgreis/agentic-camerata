package cli

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/agentic-camerata/cmt/internal/tui"
)

// DashboardCmd opens the TUI dashboard
type DashboardCmd struct{}

// Run executes the dashboard command
func (c *DashboardCmd) Run(cli *CLI) error {
	model := tui.NewDashboard(cli.Database())

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run dashboard: %w", err)
	}

	return nil
}
