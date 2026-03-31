package claude

import (
	"fmt"

	"github.com/agentic-camerata/cmt/internal/agent"
)

// promptPrefixes maps command types to the prompt prefix prepended to task descriptions.
var promptPrefixes = map[agent.CommandType]string{
	agent.CommandNew:        "",
	agent.CommandResearch:   "/research_codebase",
	agent.CommandPlan:       "/create_plan",
	agent.CommandImplement:  "/implement_plan implement all phases ignoring any manual verification steps",
	agent.CommandFixTest:    "Analyze and fix the failing test at:",
	agent.CommandLookAndFix: "Take a look at this repo and search for comments tagged with %s and propose how to solve them. If a class name or filename is provided as a parameter, focus the search on that specific file or class.",
	agent.CommandQuick:      "",
	agent.CommandReview:     "/review_code",
}

// GetPromptPrefix returns the prompt prefix for a command type.
// For look-and-fix, commentTag specifies the tag to search for (defaults to "CMT").
func GetPromptPrefix(cmd agent.CommandType, commentTag string) string {
	prefix, ok := promptPrefixes[cmd]
	if !ok {
		return ""
	}
	if cmd == agent.CommandLookAndFix {
		if commentTag == "" {
			commentTag = "CMT"
		}
		return fmt.Sprintf(prefix, commentTag)
	}
	return prefix
}
