package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/agentic-camerata/cmt/internal/db"
)

// SessionsCmd lists all tracked sessions
type SessionsCmd struct {
	Status string `short:"s" help:"Filter by status (waiting, working, completed, abandoned)" enum:"waiting,working,completed,abandoned," default:""`
	Limit  int    `short:"n" help:"Limit number of sessions shown" default:"20"`
}

// Run executes the sessions command
func (c *SessionsCmd) Run(cli *CLI) error {
	var status db.SessionStatus
	if c.Status != "" {
		status = db.SessionStatus(c.Status)
	}

	sessions, err := cli.Database().ListSessions(status)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	// Limit results
	if c.Limit > 0 && len(sessions) > c.Limit {
		sessions = sessions[:c.Limit]
	}

	// Print as table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tWORKFLOW\tDIRECTORY\tTMUX\tAGE")

	for _, s := range sessions {
		age := formatAge(s.CreatedAt)
		dir := shortenPath(s.WorkingDirectory, 30)
		tmuxLoc := fmt.Sprintf("%s:%d.%d", s.TmuxSession, s.TmuxWindow, s.TmuxPane)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			s.ID, s.Status, s.WorkflowType, dir, tmuxLoc, age)
	}

	return w.Flush()
}

// formatAge returns a human-readable age string
func formatAge(t time.Time) string {
	d := time.Since(t)

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// shortenPath shortens a path to fit within maxLen
func shortenPath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}

	// Try to show the last meaningful part
	if len(path) > maxLen {
		return "..." + path[len(path)-maxLen+3:]
	}
	return path
}
