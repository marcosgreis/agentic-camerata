package cli

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/agentic-camerata/cmt/internal/tui"
)

// DashboardCmd opens the TUI dashboard
type DashboardCmd struct {
	Venues bool `help:"Open directly to venues view" default:"false"`
	Todos  bool `help:"Open directly to todos view" default:"false"`
	Debug  bool `help:"Render dashboard to stdout and exit (for debugging)" default:"false"`
}

// Run executes the dashboard command
func (c *DashboardCmd) Run(cli *CLI) error {
	model := tui.NewDashboard(cli.Database())
	if c.Venues {
		model.SetViewMode(tui.ViewVenues)
	} else if c.Todos {
		model.SetViewMode(tui.ViewTodos)
	}

	if c.Debug {
		width, height := 120, 40
		if w, h, err := term.GetSize(0); err == nil {
			width, height = w, h
		}
		fmt.Println(model.DebugRender(width, height))
		return nil
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run dashboard: %w", err)
	}

	return nil
}
