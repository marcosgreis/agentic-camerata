package agent

import "fmt"

// promptPrefixes maps command types to the prompt prefix prepended to task descriptions.
var promptPrefixes = map[CommandType]string{
	CommandNew:              "",
	CommandResearch:         "/research_codebase",
	CommandPlan:             "/create_plan",
	CommandImplement:        "/implement_plan implement all phases ignoring any manual verification steps",
	CommandFixTest:          "Analyze and fix the failing test at:",
	CommandFixLocalComments: "Take a look at this repo and search for comments tagged with %s and propose how to solve them. If a class name or filename is provided as a parameter, focus the search on that specific file or class.",
	CommandFixPRBuild:       "Fix the build of the PR I will share and commit with the message 'Fix' and push. Do nothing if the build is not failing.",
	CommandFixPRComments:    "Read the unresolved comments from the PR and propose how to fix them",
	CommandQuick:            "",
	CommandReview:           "/review_code",
}

// GetPromptPrefix returns the prompt prefix for a command type.
// For fix-local-comments, commentTag specifies the tag to search for (defaults to "CMT").
func GetPromptPrefix(cmd CommandType, commentTag string) string {
	prefix, ok := promptPrefixes[cmd]
	if !ok {
		return ""
	}
	if cmd == CommandFixLocalComments {
		if commentTag == "" {
			commentTag = "CMT"
		}
		return fmt.Sprintf(prefix, commentTag)
	}
	return prefix
}

// ApplyPromptPrefix returns taskDescription with any command-specific prompt prefix prepended.
func ApplyPromptPrefix(cmd CommandType, taskDescription, commentTag string) string {
	prefix := GetPromptPrefix(cmd, commentTag)
	if prefix == "" {
		return taskDescription
	}
	if taskDescription == "" {
		return prefix
	}
	return prefix + " " + taskDescription
}
