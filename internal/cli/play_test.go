package cli

import (
	"testing"

	"github.com/agentic-camerata/cmt/internal/playbook"
)

func TestIsParallelizable(t *testing.T) {
	tests := []struct {
		name   string
		phases []playbook.Phase
		want   bool
	}{
		{
			name:   "single phase is not parallelizable",
			phases: []playbook.Phase{{Type: "research", Content: "topic A"}},
			want:   false,
		},
		{
			name: "two independent phases are parallelizable",
			phases: []playbook.Phase{
				{Type: "research", Content: "topic A", Tag: "a"},
				{Type: "research", Content: "topic B", Tag: "b"},
			},
			want: true,
		},
		{
			name: "three independent phases are parallelizable",
			phases: []playbook.Phase{
				{Type: "research", Content: "topic A"},
				{Type: "research", Content: "topic B"},
				{Type: "research", Content: "topic C"},
			},
			want: true,
		},
		{
			name: "phase using sibling tag is not parallelizable",
			phases: []playbook.Phase{
				{Type: "research", Content: "topic A", Tag: "a"},
				{Type: "research", Content: "topic B", Uses: []string{"a"}},
			},
			want: false,
		},
		{
			name: "phase using external tag is parallelizable",
			phases: []playbook.Phase{
				{Type: "research", Content: "topic A", Tag: "a", Uses: []string{"external"}},
				{Type: "research", Content: "topic B", Tag: "b"},
			},
			want: true,
		},
		{
			name: "one phase with intra-group dependency makes all non-parallelizable",
			phases: []playbook.Phase{
				{Type: "research", Content: "topic A", Tag: "a"},
				{Type: "research", Content: "topic B", Tag: "b"},
				{Type: "research", Content: "topic C", Uses: []string{"a"}},
			},
			want: false,
		},
		{
			name:   "empty slice is not parallelizable",
			phases: []playbook.Phase{},
			want:   false,
		},
		{
			name: "phases with no tags are parallelizable",
			phases: []playbook.Phase{
				{Type: "research", Content: "topic A"},
				{Type: "research", Content: "topic B"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isParallelizable(tt.phases)
			if got != tt.want {
				t.Errorf("isParallelizable() = %v, want %v", got, tt.want)
			}
		})
	}
}
