package claude

// CommandType represents a cmt command that starts a Claude session
type CommandType string

const (
	CommandNew        CommandType = "new"
	CommandContinue   CommandType = "continue"
	CommandResearch   CommandType = "research"
	CommandPlan       CommandType = "plan"
	CommandImplement  CommandType = "implement"
	CommandFixTest    CommandType = "fix-test"
	CommandLookAndFix CommandType = "look-and-fix"
	CommandQuick      CommandType = "quick"
)

// Prompt prefixes prepended to the task description for each command
var promptPrefixes = map[CommandType]string{
	CommandNew:        "",
	CommandContinue:   "",
	CommandResearch:   "/research_codebase",
	CommandPlan:       "/create_plan",
	CommandImplement:  "/implement_plan implement all phases ignoring any manual verification steps",
	CommandFixTest:    "Analyze and fix the failing test at:",
	CommandLookAndFix: "Take a look at this repo and search for comments tagged with CMT and propose how to solve them. If a class name or filename is provided as a parameter, focus the search on that specific file or class.",
	CommandQuick:      "",
}

// GetPromptPrefix returns the prompt prefix for a command type
func GetPromptPrefix(cmd CommandType) string {
	if prefix, ok := promptPrefixes[cmd]; ok {
		return prefix
	}
	return ""
}
