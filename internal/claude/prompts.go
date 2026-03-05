package claude

import "fmt"

// CommandType represents a cmt command that starts a Claude session
type CommandType string

const (
	CommandNew        CommandType = "new"
	CommandResearch   CommandType = "research"
	CommandPlan       CommandType = "plan"
	CommandImplement  CommandType = "implement"
	CommandFixTest    CommandType = "fix-test"
	CommandLookAndFix CommandType = "look-and-fix"
	CommandQuick      CommandType = "quick"
	CommandReview     CommandType = "review"
)

// Prompt prefixes prepended to the task description for each command
var promptPrefixes = map[CommandType]string{
	CommandNew:        "",
	CommandResearch:   "/research_codebase",
	CommandPlan:       "/create_plan",
	CommandImplement:  "/implement_plan implement all phases ignoring any manual verification steps",
	CommandFixTest:    "Analyze and fix the failing test at: ",
	CommandLookAndFix: "Take a look at this repo and search for comments tagged with %s and propose how to solve them. If a class name or filename is provided as a parameter, focus the search on that specific file or class.",
	CommandQuick:      "",
	CommandReview:     "Review the changes in this working directory. Examine git diff (unstaged changes), git diff --staged (staged changes), and recent git log to understand what has changed. Provide a structured code review covering: correctness, potential bugs, code quality, naming, and any suggestions for improvement. If additional context is provided, focus the review on that area. Create a markdown file in thoughts/shared/reviews/ with the suggestions. DO NOT make any changes",
}

// GetPromptPrefix returns the prompt prefix for a command type.
// For look-and-fix, commentTag specifies the tag to search for (defaults to "CMT").
func GetPromptPrefix(cmd CommandType, commentTag string) string {
	prefix, ok := promptPrefixes[cmd]
	if !ok {
		return ""
	}
	if cmd == CommandLookAndFix {
		if commentTag == "" {
			commentTag = "CMT"
		}
		return fmt.Sprintf(prefix, commentTag)
	}
	return prefix
}
