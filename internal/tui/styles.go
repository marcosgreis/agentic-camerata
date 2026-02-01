package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette
var (
	colorPrimary   = lipgloss.Color("#7C3AED") // Purple
	colorSecondary = lipgloss.Color("#3B82F6") // Blue
	colorSuccess   = lipgloss.Color("#10B981") // Green
	colorWarning   = lipgloss.Color("#F59E0B") // Amber
	colorError     = lipgloss.Color("#EF4444") // Red
	colorMuted     = lipgloss.Color("#6B7280") // Gray
	colorBorder    = lipgloss.Color("#374151") // Dark gray
	colorBg        = lipgloss.Color("#1F2937") // Dark background
	colorFg        = lipgloss.Color("#F9FAFB") // Light foreground
)

// Styles
var (
	// Base styles
	baseStyle = lipgloss.NewStyle().
			Foreground(colorFg)

	// Panel styles
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	focusedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary).
				Padding(0, 1)

	// Title styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Padding(0, 1)

	// Status styles
	statusWaiting = lipgloss.NewStyle().
			Foreground(colorWarning). // Amber (#F59E0B)
			Bold(true)

	statusWorking = lipgloss.NewStyle().
			Foreground(colorSuccess). // Green (#10B981)
			Bold(true)

	statusCompleted = lipgloss.NewStyle().
			Foreground(colorMuted)

	statusAbandoned = lipgloss.NewStyle().
			Foreground(colorError)

	statusKilled = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)

	statusDeleted = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	statusRestored = lipgloss.NewStyle().
			Foreground(colorSecondary)

	// Workflow type styles
	workflowResearch = lipgloss.NewStyle().
				Foreground(colorSecondary)

	workflowPlan = lipgloss.NewStyle().
			Foreground(colorWarning)

	workflowImplement = lipgloss.NewStyle().
				Foreground(colorSuccess)

	workflowGeneral = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Selection styles
	selectedStyle = lipgloss.NewStyle().
			Background(colorPrimary).
			Foreground(colorFg).
			Bold(true)

	// Help style
	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(0, 1)

	// Error style
	errorStyle = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)

	// Section header styles
	sectionActiveHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#ECFDF5")).
				Background(lipgloss.Color("#059669")).
				Padding(0, 1).
				MarginBottom(0)

	sectionHistoryHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#F3F4F6")).
				Background(lipgloss.Color("#4B5563")).
				Padding(0, 1).
				MarginTop(1).
				MarginBottom(0)

	sectionTrashHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FEE2E2")).
				Background(lipgloss.Color("#991B1B")).
				Padding(0, 1).
				MarginBottom(0)

	// Dim style for non-active sessions
	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF"))

	// Selection indicator style (purple and bold like title)
	selectionIndicatorStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary)

	// Dimmed status styles for history section (waiting/working shouldn't appear here, but handle gracefully)
	statusWaitingDim = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#B77A08")) // Darker amber

	statusWorkingDim = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0D8F68")) // Darker green

	statusCompletedDim = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280")) // Darker gray

	statusAbandonedDim = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#B45050")) // Darker red

	statusKilledDim = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#B45050")) // Darker red

	statusDeletedDim = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280")).
				Italic(true)

	statusRestoredDim = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#2563A8"))

	// Dimmed workflow styles for history section
	workflowResearchDim = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#2563A8")) // Darker blue

	workflowPlanDim = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#B77A08")) // Darker amber

	workflowImplementDim = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#0D8F68")) // Darker green

	workflowGeneralDim = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280")) // Darker gray

	// Dim ID style for history
	dimIDStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7A8290"))

	// Dim age style for history
	dimAgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7A8290"))

	// Column header style
	columnHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#D1D5DB")).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(lipgloss.Color("#4B5563")).
				PaddingLeft(2)
)

// StatusStyle returns the appropriate style for a status
func StatusStyle(status string) lipgloss.Style {
	switch status {
	case "waiting":
		return statusWaiting
	case "working":
		return statusWorking
	case "completed":
		return statusCompleted
	case "abandoned":
		return statusAbandoned
	case "killed":
		return statusKilled
	case "deleted":
		return statusDeleted
	case "restored":
		return statusRestored
	default:
		return baseStyle
	}
}

// StatusStyleDim returns a dimmed style for history section
func StatusStyleDim(status string) lipgloss.Style {
	switch status {
	case "waiting":
		return statusWaitingDim
	case "working":
		return statusWorkingDim
	case "completed":
		return statusCompletedDim
	case "abandoned":
		return statusAbandonedDim
	case "killed":
		return statusKilledDim
	case "deleted":
		return statusDeletedDim
	case "restored":
		return statusRestoredDim
	default:
		return dimStyle
	}
}

// WorkflowStyle returns the appropriate style for a workflow type
func WorkflowStyle(workflow string) lipgloss.Style {
	switch workflow {
	case "research":
		return workflowResearch
	case "plan":
		return workflowPlan
	case "implement":
		return workflowImplement
	default:
		return workflowGeneral
	}
}

// WorkflowStyleDim returns a dimmed style for history section
func WorkflowStyleDim(workflow string) lipgloss.Style {
	switch workflow {
	case "research":
		return workflowResearchDim
	case "plan":
		return workflowPlanDim
	case "implement":
		return workflowImplementDim
	default:
		return workflowGeneralDim
	}
}

// StatusBadge returns a styled status with icon
func StatusBadge(status string) string {
	switch status {
	case "waiting":
		return statusWaiting.Render("⏸ waiting")
	case "working":
		return statusWorking.Render("● working")
	case "completed":
		return statusCompleted.Render("✓ completed")
	case "abandoned":
		return statusAbandoned.Render("✗ abandoned")
	case "killed":
		return statusKilled.Render("☠ killed")
	case "deleted":
		return statusDeleted.Render("🗑 deleted")
	case "restored":
		return statusRestored.Render("↩ restored")
	default:
		return status
	}
}
