package agent

import (
	"context"
	"regexp"

	"github.com/agentic-camerata/cmt/internal/db"
)

// CommandType represents a cmt command that starts an agent session
type CommandType string

const (
	CommandNew        CommandType = "new"
	CommandResearch   CommandType = "research"
	CommandPlan       CommandType = "plan"
	CommandImplement  CommandType = "implement"
	CommandFixTest         CommandType = "fix-test"
	CommandFixLocalComments CommandType = "fix-local-comments"
	CommandFixPRBuild       CommandType = "fix-pr-build"
	CommandFixPRComments    CommandType = "fix-pr-comments"
	CommandQuick       CommandType = "quick"
	CommandReview     CommandType = "review"
)

// RunOptions configures an agent session
type RunOptions struct {
	Command         CommandType
	WorkflowType    db.WorkflowType
	TaskDescription string
	WorkingDir      string         // Override working directory
	Model           string         // Model to use (e.g., "sonnet", "opus")
	PrintMode       bool           // If true, print response and exit (non-interactive)
	AutonomousMode  bool           // If true, skip permission prompts
	CommentTag      string         // Comment tag for fix-local-comments (from CMT_COMMENT_TAG env var)
	ResumeSessionID string         // If non-empty, pass --resume to agent. "*" means interactive picker
	SkipTracking    bool           // If true, skip DB session creation and activity monitoring
	AutoTerminate   bool           // If true, send kill when session goes idle after working
	CapturedFiles     *[]string      // If non-nil, collect thoughts/shared/*.md paths from output
	CapturePattern    *regexp.Regexp // If non-nil, override default file capture regex
	CapturedSessionID *string        // If non-nil, capture Claude session ID from PTY output into this string
	ParentID          string         // Parent session ID (for play command phases)
	Interrupted     *bool          // If non-nil, set to true when the child exits without auto-terminate firing
}

// Agent defines the interface for AI coding agents (Claude, Codex, etc.)
type Agent interface {
	Run(ctx context.Context, opts RunOptions) error
	// DefaultModel returns the runner-specific default model for a command type.
	// Returns "" if the runner should use its CLI's built-in default.
	DefaultModel(cmd CommandType) string
}
