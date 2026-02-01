package tui

import (
	"strings"
	"testing"
)

func TestStatusStyle(t *testing.T) {
	tests := []struct {
		name   string
		status string
	}{
		{"waiting", "waiting"},
		{"working", "working"},
		{"completed", "completed"},
		{"abandoned", "abandoned"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := StatusStyle(tt.status)

			// Just verify it doesn't panic and returns a valid style
			rendered := style.Render("test")
			if rendered == "" {
				t.Error("StatusStyle() rendered empty string")
			}
		})
	}
}

func TestWorkflowStyle(t *testing.T) {
	tests := []struct {
		name     string
		workflow string
	}{
		{"research", "research"},
		{"plan", "plan"},
		{"implement", "implement"},
		{"general", "general"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := WorkflowStyle(tt.workflow)

			// Just verify it doesn't panic and returns a valid style
			rendered := style.Render("test")
			if rendered == "" {
				t.Error("WorkflowStyle() rendered empty string")
			}
		})
	}
}

func TestStylesNotNil(t *testing.T) {
	// Verify all exported styles are usable
	styles := []struct {
		name  string
		style func() string
	}{
		{"baseStyle", func() string { return baseStyle.Render("test") }},
		{"panelStyle", func() string { return panelStyle.Render("test") }},
		{"focusedPanelStyle", func() string { return focusedPanelStyle.Render("test") }},
		{"titleStyle", func() string { return titleStyle.Render("test") }},
		{"statusWaiting", func() string { return statusWaiting.Render("test") }},
		{"statusWorking", func() string { return statusWorking.Render("test") }},
		{"statusCompleted", func() string { return statusCompleted.Render("test") }},
		{"statusAbandoned", func() string { return statusAbandoned.Render("test") }},
		{"workflowResearch", func() string { return workflowResearch.Render("test") }},
		{"workflowPlan", func() string { return workflowPlan.Render("test") }},
		{"workflowImplement", func() string { return workflowImplement.Render("test") }},
		{"workflowGeneral", func() string { return workflowGeneral.Render("test") }},
		{"selectedStyle", func() string { return selectedStyle.Render("test") }},
		{"helpStyle", func() string { return helpStyle.Render("test") }},
		{"errorStyle", func() string { return errorStyle.Render("test") }},
	}

	for _, tt := range styles {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.style()
			if result == "" {
				t.Errorf("%s rendered empty string", tt.name)
			}
		})
	}
}

func TestStatusBadge(t *testing.T) {
	tests := []struct {
		status   string
		contains string
	}{
		{"waiting", "⏸"},
		{"working", "●"},
		{"completed", "✓"},
		{"abandoned", "✗"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := StatusBadge(tt.status)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("StatusBadge(%q) = %q, want to contain %q", tt.status, result, tt.contains)
			}
		})
	}
}

func TestStatusStyleDim(t *testing.T) {
	tests := []struct {
		name   string
		status string
	}{
		{"waiting", "waiting"},
		{"working", "working"},
		{"completed", "completed"},
		{"abandoned", "abandoned"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := StatusStyleDim(tt.status)

			// Just verify it doesn't panic and returns a valid style
			rendered := style.Render("test")
			if rendered == "" {
				t.Error("StatusStyleDim() rendered empty string")
			}
		})
	}
}
