package tui

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/agentic-camerata/cmt/internal/db"
)

var (
	// Matches link markup: <url|label> or <url>
	slackLinkRe = regexp.MustCompile(`<(https?://[^|>]+)\|([^>]+)>`)
	slackURLRe  = regexp.MustCompile(`<(https?://[^>]+)>`)
	// Matches emoji codes: :emoji_name:
	slackEmojiRe = regexp.MustCompile(`:[a-zA-Z0-9_+-]+:`)
)

// cleanMarkup strips formatting from text for terminal display.
// Converts <url|label> to label, <url> to url, and removes :emoji: codes.
func cleanMarkup(s string) string {
	// Replace <url|label> with label first (more specific pattern)
	s = slackLinkRe.ReplaceAllString(s, "$2")
	// Replace <url> with url
	s = slackURLRe.ReplaceAllString(s, "$1")
	// Remove emoji codes
	s = slackEmojiRe.ReplaceAllString(s, "")
	// Clean up any resulting double spaces
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// sortedTodos returns todos with TodoStatusTodo first, then TodoStatusDone.
// Pending items are sorted by date ascending (nulls last).
// Done items are sorted by updated_at descending (most recently completed first).
func sortedTodos(todos []*db.Todo) []*db.Todo {
	sorted := make([]*db.Todo, len(todos))
	copy(sorted, todos)
	sort.Slice(sorted, func(i, j int) bool {
		iPending := sorted[i].Status == db.TodoStatusTodo
		jPending := sorted[j].Status == db.TodoStatusTodo
		if iPending != jPending {
			return iPending
		}
		// Done items: sort by updated_at descending (most recently completed first)
		if !iPending {
			return sorted[i].UpdatedAt.After(sorted[j].UpdatedAt)
		}
		// Pending items: sort by date ascending (nulls last)
		if sorted[i].Date == nil && sorted[j].Date == nil {
			return false
		}
		if sorted[i].Date == nil {
			return false
		}
		if sorted[j].Date == nil {
			return true
		}
		return sorted[i].Date.After(*sorted[j].Date)
	})
	return sorted
}

// Column widths for todos table
const (
	todoColStatusWidth = 10 // "[ ] to do"
	todoColDateWidth   = 12 // "Mar 01 2026"
	todoColSenderWidth = 14
	todoColSourceWidth = 12
)

// renderTodosList renders the full todos view panel
func (d *Dashboard) renderTodosList() string {
	width := d.listWidth()
	height := d.contentHeight()

	items := sortedTodos(d.todos)
	var content strings.Builder

	// Header line
	// Account for panel border (2) + panel padding (2) + selection indicator (2) = 6
	promptWidth := width - 6 - todoColStatusWidth - 1 - todoColDateWidth - 1 - todoColSenderWidth - 1 - todoColSourceWidth - 1
	if promptWidth < 10 {
		promptWidth = 10
	}
	header := padRight("STATUS", todoColStatusWidth) + " " +
		padRight("SUMMARY", promptWidth) + " " +
		padRight("SENDER", todoColSenderWidth) + " " +
		padRight("DATE", todoColDateWidth) + " " +
		padRight("SOURCE", todoColSourceWidth)
	content.WriteString(columnHeaderStyle.Render(header))
	content.WriteString("\n")

	// Split into sections
	var pending, done []*db.Todo
	for _, item := range items {
		if item.Status == db.TodoStatusTodo {
			pending = append(pending, item)
		} else {
			done = append(done, item)
		}
	}

	sortedItems := sortedTodos(d.todos)
	lineCount := 1
	maxLines := height - 4

	// TO DO section
	sectionHeader := sectionTodoHeader.Width(width - 4).Render(fmt.Sprintf("[ ] TO DO (%d)", len(pending)))
	content.WriteString(sectionHeader)
	content.WriteString("\n")
	lineCount++

	globalIdx := 0
	if len(pending) == 0 {
		content.WriteString(dimStyle.Render("  Nothing to do") + "\n")
		lineCount++
	}
	for i := range sortedItems {
		if sortedItems[i].Status != db.TodoStatusTodo {
			break
		}
		if lineCount >= maxLines {
			content.WriteString(fmt.Sprintf("  ... and more\n"))
			break
		}
		isSelected := globalIdx == d.selected
		line := d.formatTodoLine(sortedItems[i], promptWidth)
		if isSelected {
			content.WriteString(selectionIndicatorStyle.Render(">") + " " + line + "\n")
		} else {
			content.WriteString("  " + line + "\n")
		}
		lineCount++
		globalIdx++
	}

	// DONE section
	if lineCount < maxLines {
		sectionHeader = sectionDoneHeader.Width(width - 4).Render(fmt.Sprintf("[x] DONE (%d)", len(done)))
		content.WriteString(sectionHeader)
		content.WriteString("\n")
		lineCount++

		if len(done) == 0 {
			content.WriteString(dimStyle.Render("  Nothing done yet") + "\n")
		}
		for i := len(pending); i < len(sortedItems); i++ {
			if lineCount >= maxLines {
				content.WriteString(fmt.Sprintf("  ... and more\n"))
				break
			}
			isSelected := globalIdx == d.selected
			line := d.formatTodoLine(sortedItems[i], promptWidth)
			if isSelected {
				content.WriteString(selectionIndicatorStyle.Render(">") + " " + line + "\n")
			} else {
				content.WriteString("  " + line + "\n")
			}
			lineCount++
			globalIdx++
		}
	}

	style := panelStyle.Width(width).Height(height)
	if d.focus == focusList {
		style = focusedPanelStyle.Width(width).Height(height)
	}
	title := titleStyle.Render("Todos")
	return lipgloss.JoinVertical(lipgloss.Left, title, style.Render(content.String()))
}

// padRight pads s with spaces to the given width, based on visible (non-ANSI) length.
func padRight(s string, width int) string {
	visible := lipgloss.Width(s)
	if visible >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visible)
}

// truncateToWidth truncates a string to fit within the given visible width,
// appending "…" if truncated. Uses rune-aware iteration for correct handling
// of multi-byte characters.
func truncateToWidth(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	// Build up rune by rune until we'd exceed width-1 (leaving room for …)
	result := []rune(s)
	for i := len(result); i > 0; i-- {
		candidate := string(result[:i]) + "…"
		if lipgloss.Width(candidate) <= width {
			return candidate
		}
	}
	return "…"
}

// formatTodoLine formats a single todo row
func (d *Dashboard) formatTodoLine(item *db.Todo, summaryWidth int) string {
	// Status checkbox with color
	var statusStyled string
	if item.Status == db.TodoStatusTodo {
		statusStyled = todoStatusTodo.Render("[ ] to do")
	} else {
		statusStyled = todoStatusDone.Render("[x] done")
	}

	// Date
	dateStr := "\u2014"
	if item.Date != nil {
		dateStr = item.Date.Format("Jan 02 2006")
	}

	// Sender
	senderStr := "\u2014"
	if item.Sender != nil {
		senderStr = truncateToWidth(*item.Sender, todoColSenderWidth)
	}

	// Source
	sourceStr := "\u2014"
	if item.Source != nil {
		sourceStr = truncateToWidth(*item.Source, todoColSourceWidth)
	}

	// Summary (show only first line in table, clean markup)
	summary, _, _ := strings.Cut(item.Summary, "\n")
	summary = cleanMarkup(summary)
	summary = truncateToWidth(summary, summaryWidth)

	if item.Status == db.TodoStatusDone {
		return padRight(statusStyled, todoColStatusWidth) + " " +
			dimStyle.Render(padRight(summary, summaryWidth)) + " " +
			dimStyle.Render(padRight(senderStr, todoColSenderWidth)) + " " +
			dimStyle.Render(padRight(dateStr, todoColDateWidth)) + " " +
			dimStyle.Render(padRight(sourceStr, todoColSourceWidth))
	}
	return padRight(statusStyled, todoColStatusWidth) + " " +
		padRight(summary, summaryWidth) + " " +
		padRight(senderStr, todoColSenderWidth) + " " +
		padRight(dateStr, todoColDateWidth) + " " +
		padRight(sourceStr, todoColSourceWidth)
}

// formatTodoInfo formats the info panel content for a selected todo item
func (d *Dashboard) formatTodoInfo(item *db.Todo) string {
	var b strings.Builder

	// Status
	if item.Status == db.TodoStatusTodo {
		b.WriteString("Status:  " + todoStatusTodo.Render("[ ] to do") + "\n")
	} else {
		b.WriteString("Status:  " + todoStatusDone.Render("[x] done") + "\n")
	}

	// Summary
	b.WriteString(fmt.Sprintf("Summary: %s\n", cleanMarkup(item.Summary)))

	// Nullable fields
	if item.Date != nil {
		b.WriteString(fmt.Sprintf("Date:    %s\n", item.Date.Format(time.RFC3339)))
	} else {
		b.WriteString("Date:    \u2014\n")
	}

	if item.Source != nil {
		b.WriteString(fmt.Sprintf("Source:  %s\n", *item.Source))
	} else {
		b.WriteString("Source:  \u2014\n")
	}

	if item.URL != nil {
		b.WriteString(fmt.Sprintf("URL:     %s\n", *item.URL))
	} else {
		b.WriteString("URL:     \u2014\n")
	}

	if item.Channel != nil {
		b.WriteString(fmt.Sprintf("Channel: %s\n", *item.Channel))
	} else {
		b.WriteString("Channel: \u2014\n")
	}

	if item.Sender != nil {
		b.WriteString(fmt.Sprintf("Sender:  %s\n", *item.Sender))
	} else {
		b.WriteString("Sender:  \u2014\n")
	}

	if item.IdempotencyKey != nil {
		b.WriteString(fmt.Sprintf("Key:     %s\n", *item.IdempotencyKey))
	} else {
		b.WriteString("Key:     \u2014\n")
	}

	if item.FullMessage != nil {
		b.WriteString(fmt.Sprintf("Message: %s\n", cleanMarkup(*item.FullMessage)))
	} else {
		b.WriteString("Message: \u2014\n")
	}

	return b.String()
}
