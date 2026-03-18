package claude

import "github.com/agentic-camerata/cmt/internal/runner"

// IsInteractive returns true if stdin and stdout are terminals.
func IsInteractive() bool {
	return runner.IsInteractive()
}
