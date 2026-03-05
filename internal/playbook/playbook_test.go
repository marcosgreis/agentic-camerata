package playbook

import (
	"slices"
	"testing"
)

func TestParseContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []Phase
		wantErr bool
	}{
		{
			name: "basic playbook",
			content: `# My Playbook

## Research
Explore the codebase.

## Plan
Design a solution.

## Implement
`,
			want: []Phase{
				{Type: "research", Content: "Explore the codebase."},
				{Type: "plan", Content: "Design a solution."},
				{Type: "implement", Content: ""},
			},
		},
		{
			name: "case insensitive headings",
			content: `## RESEARCH
topic

## Plan
design

## implement
build
`,
			want: []Phase{
				{Type: "research", Content: "topic"},
				{Type: "plan", Content: "design"},
				{Type: "implement", Content: "build"},
			},
		},
		{
			name: "multiline content",
			content: `## Research
Line one.
Line two.
Line three.
`,
			want: []Phase{
				{Type: "research", Content: "Line one.\nLine two.\nLine three."},
			},
		},
		{
			name:    "no phases",
			content: "# Just a title\nSome text",
			wantErr: true,
		},
		{
			name:    "unknown phase type",
			content: "## Deploy\nDeploy stuff",
			wantErr: true,
		},
		{
			name: "phase with tag and uses",
			content: `## Research
tag: auth-research
Investigate auth patterns

## Plan
tag: auth-plan
uses: auth-research
Design auth implementation
`,
			want: []Phase{
				{Type: "research", Content: "Investigate auth patterns", Tag: "auth-research"},
				{Type: "plan", Content: "Design auth implementation", Tag: "auth-plan", Uses: []string{"auth-research"}},
			},
		},
		{
			name: "uses with multiple tags",
			content: `## Research
tag: r1
Topic 1

## Research
tag: r2
Topic 2

## Plan
uses: r1, r2
Design both
`,
			want: []Phase{
				{Type: "research", Content: "Topic 1", Tag: "r1"},
				{Type: "research", Content: "Topic 2", Tag: "r2"},
				{Type: "plan", Content: "Design both", Uses: []string{"r1", "r2"}},
			},
		},
		{
			name: "no metadata — backward compatible",
			content: `## Research
Explore the codebase.
`,
			want: []Phase{
				{Type: "research", Content: "Explore the codebase."},
			},
		},
		{
			name:    "duplicate tags error",
			content: "## Research\ntag: same\nTopic 1\n\n## Research\ntag: same\nTopic 2\n",
			wantErr: true,
		},
		{
			name: "phase with include",
			content: `## Research
include: playbook_test.go
Explore the codebase.
`,
			want: []Phase{
				{Type: "research", Content: "Explore the codebase.", Include: []string{"playbook_test.go"}},
			},
		},
		{
			name: "phase with multiple includes",
			content: `## Research
include: playbook_test.go, playbook.go
Explore the codebase.
`,
			want: []Phase{
				{Type: "research", Content: "Explore the codebase.", Include: []string{"playbook_test.go", "playbook.go"}},
			},
		},
		{
			name:    "include nonexistent file",
			content: "## Research\ninclude: nonexistent_file.go\nExplore\n",
			wantErr: true,
		},
		{
			name: "all phase types",
			content: `## New
general

## Research
research

## Plan
plan

## Implement
implement

## Fix
fix

## Look-and-Fix
look and fix

## Review
review changes
`,
			want: []Phase{
				{Type: "new", Content: "general"},
				{Type: "research", Content: "research"},
				{Type: "plan", Content: "plan"},
				{Type: "implement", Content: "implement"},
				{Type: "fix", Content: "fix"},
				{Type: "look-and-fix", Content: "look and fix"},
				{Type: "review", Content: "review changes"},
			},
		},
		{
			name: "exit phase terminates playbook",
			content: `## Research
Explore the codebase.

## Exit

## Implement
This should not run.
`,
			want: []Phase{
				{Type: "research", Content: "Explore the codebase."},
				{Type: "exit", Content: ""},
				{Type: "implement", Content: "This should not run."},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb, err := ParseContent(tt.content)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(pb.Phases) != len(tt.want) {
				t.Fatalf("got %d phases, want %d", len(pb.Phases), len(tt.want))
			}

			for i, phase := range pb.Phases {
				if phase.Type != tt.want[i].Type {
					t.Errorf("phase %d: type = %q, want %q", i, phase.Type, tt.want[i].Type)
				}
				if phase.Content != tt.want[i].Content {
					t.Errorf("phase %d: content = %q, want %q", i, phase.Content, tt.want[i].Content)
				}
				if phase.Tag != tt.want[i].Tag {
					t.Errorf("phase %d: tag = %q, want %q", i, phase.Tag, tt.want[i].Tag)
				}
				if !slices.Equal(phase.Uses, tt.want[i].Uses) {
					t.Errorf("phase %d: uses = %v, want %v", i, phase.Uses, tt.want[i].Uses)
				}
				if !slices.Equal(phase.Include, tt.want[i].Include) {
					t.Errorf("phase %d: include = %v, want %v", i, phase.Include, tt.want[i].Include)
				}
			}
		})
	}
}
