package agent

import "testing"

func TestGetPromptPrefix(t *testing.T) {
	tests := []struct {
		name       string
		command    CommandType
		commentTag string
		wantPrefix string
	}{
		{
			name:       "new command has no prefix",
			command:    CommandNew,
			wantPrefix: "",
		},
		{
			name:       "research command has skill prefix",
			command:    CommandResearch,
			wantPrefix: "/research_codebase",
		},
		{
			name:       "plan command has skill prefix",
			command:    CommandPlan,
			wantPrefix: "/create_plan",
		},
		{
			name:       "implement command has skill prefix",
			command:    CommandImplement,
			wantPrefix: "/implement_plan implement all phases ignoring any manual verification steps",
		},
		{
			name:       "fix-test command has instruction prefix",
			command:    CommandFixTest,
			wantPrefix: "Analyze and fix the failing test at:",
		},
		{
			name:       "fix-local-comments command has instruction prefix with default tag",
			command:    CommandFixLocalComments,
			wantPrefix: "Take a look at this repo and search for comments tagged with CMT and propose how to solve them. If a class name or filename is provided as a parameter, focus the search on that specific file or class.",
		},
		{
			name:       "quick command has no prefix",
			command:    CommandQuick,
			wantPrefix: "",
		},
		{
			name:       "unknown command has no prefix",
			command:    CommandType("unknown"),
			wantPrefix: "",
		},
		{
			name:       "fix-local-comments with custom tag",
			command:    CommandFixLocalComments,
			commentTag: "TODO",
			wantPrefix: "Take a look at this repo and search for comments tagged with TODO and propose how to solve them. If a class name or filename is provided as a parameter, focus the search on that specific file or class.",
		},
		{
			name:       "fix-local-comments with empty tag defaults to CMT",
			command:    CommandFixLocalComments,
			commentTag: "",
			wantPrefix: "Take a look at this repo and search for comments tagged with CMT and propose how to solve them. If a class name or filename is provided as a parameter, focus the search on that specific file or class.",
		},
		{
			name:       "non-fix-local-comments ignores commentTag",
			command:    CommandResearch,
			commentTag: "SOMETHING",
			wantPrefix: "/research_codebase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPromptPrefix(tt.command, tt.commentTag)
			if got != tt.wantPrefix {
				t.Errorf("GetPromptPrefix() = %q, want %q", got, tt.wantPrefix)
			}
		})
	}
}

func TestApplyPromptPrefix(t *testing.T) {
	tests := []struct {
		name            string
		command         CommandType
		taskDescription string
		commentTag      string
		wantTask        string
	}{
		{
			name:            "research adds prefix",
			command:         CommandResearch,
			taskDescription: "auth system",
			wantTask:        "/research_codebase auth system",
		},
		{
			name:            "new has no prefix",
			command:         CommandNew,
			taskDescription: "fix bug",
			wantTask:        "fix bug",
		},
		{
			name:            "prefix only when task empty",
			command:         CommandResearch,
			taskDescription: "",
			wantTask:        "/research_codebase",
		},
		{
			name:            "fix-local-comments with tag",
			command:         CommandFixLocalComments,
			taskDescription: "MyClass",
			commentTag:      "FIXME",
			wantTask:        "Take a look at this repo and search for comments tagged with FIXME and propose how to solve them. If a class name or filename is provided as a parameter, focus the search on that specific file or class. MyClass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyPromptPrefix(tt.command, tt.taskDescription, tt.commentTag)
			if got != tt.wantTask {
				t.Errorf("ApplyPromptPrefix() = %q, want %q", got, tt.wantTask)
			}
		})
	}
}
