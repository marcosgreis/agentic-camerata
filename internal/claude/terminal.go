package claude

import (
	"os"

	"golang.org/x/term"
)

// terminalState holds the original terminal state
type terminalState struct {
	state *term.State
}

// makeRaw puts the terminal in raw mode and returns the previous state
func makeRaw(fd uintptr) (interface{}, error) {
	state, err := term.MakeRaw(int(fd))
	if err != nil {
		return nil, err
	}
	return &terminalState{state: state}, nil
}

// restoreTerminal restores the terminal to its previous state
func restoreTerminal(fd uintptr, state interface{}) {
	if state == nil {
		return
	}
	if ts, ok := state.(*terminalState); ok && ts.state != nil {
		term.Restore(int(fd), ts.state)
	}
}

// isTerminal returns true if the file descriptor is a terminal
func isTerminal(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}

// getSize returns the terminal size
func getSize(fd uintptr) (width, height int, err error) {
	return term.GetSize(int(fd))
}

// IsInteractive returns true if stdin and stdout are terminals
func IsInteractive() bool {
	return isTerminal(os.Stdin.Fd()) && isTerminal(os.Stdout.Fd())
}
