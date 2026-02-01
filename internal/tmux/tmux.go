package tmux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var (
	ErrNotInTmux = errors.New("not running inside tmux")
)

// Location represents a tmux session/window/pane location
type Location struct {
	Session string
	Window  int
	Pane    int
}

// String returns a string representation of the location
func (l Location) String() string {
	return fmt.Sprintf("%s:%d.%d", l.Session, l.Window, l.Pane)
}

// InTmux checks if we're running inside a tmux session
func InTmux() bool {
	return os.Getenv("TMUX") != ""
}

// RequireTmux returns an error if not running inside tmux
func RequireTmux() error {
	if !InTmux() {
		return ErrNotInTmux
	}
	return nil
}

// CurrentLocation returns the current tmux session, window, and pane
func CurrentLocation() (*Location, error) {
	if err := RequireTmux(); err != nil {
		return nil, err
	}

	// Get session:window.pane format
	out, err := exec.Command("tmux", "display-message", "-p", "#{session_name}:#{window_index}:#{pane_index}").Output()
	if err != nil {
		return nil, fmt.Errorf("get tmux location: %w", err)
	}

	parts := strings.Split(strings.TrimSpace(string(out)), ":")
	if len(parts) != 3 {
		return nil, fmt.Errorf("unexpected tmux output format: %s", string(out))
	}

	window, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("parse window index: %w", err)
	}

	pane, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("parse pane index: %w", err)
	}

	return &Location{
		Session: parts[0],
		Window:  window,
		Pane:    pane,
	}, nil
}

// JumpTo switches to the specified tmux location
func JumpTo(loc Location) error {
	if err := RequireTmux(); err != nil {
		return err
	}

	// Switch to session if different
	currentSession, err := exec.Command("tmux", "display-message", "-p", "#{session_name}").Output()
	if err != nil {
		return fmt.Errorf("get current session: %w", err)
	}

	if strings.TrimSpace(string(currentSession)) != loc.Session {
		if err := exec.Command("tmux", "switch-client", "-t", loc.Session).Run(); err != nil {
			return fmt.Errorf("switch session: %w", err)
		}
	}

	// Select window
	target := fmt.Sprintf("%s:%d", loc.Session, loc.Window)
	if err := exec.Command("tmux", "select-window", "-t", target).Run(); err != nil {
		return fmt.Errorf("select window: %w", err)
	}

	// Select pane
	paneTarget := fmt.Sprintf("%s:%d.%d", loc.Session, loc.Window, loc.Pane)
	if err := exec.Command("tmux", "select-pane", "-t", paneTarget).Run(); err != nil {
		return fmt.Errorf("select pane: %w", err)
	}

	return nil
}

// ListSessions returns a list of tmux session names
func ListSessions() ([]string, error) {
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		return nil, fmt.Errorf("list tmux sessions: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var sessions []string
	for _, line := range lines {
		if line != "" {
			sessions = append(sessions, line)
		}
	}
	return sessions, nil
}

// GetPaneWorkingDirectory returns the working directory of a specific pane
func GetPaneWorkingDirectory(loc Location) (string, error) {
	target := fmt.Sprintf("%s:%d.%d", loc.Session, loc.Window, loc.Pane)
	out, err := exec.Command("tmux", "display-message", "-t", target, "-p", "#{pane_current_path}").Output()
	if err != nil {
		return "", fmt.Errorf("get pane working directory: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
