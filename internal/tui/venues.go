package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/agentic-camerata/cmt/internal/db"
)

// VenueDisplay represents the display style for venues
type VenueDisplay int

const (
	VenueDisplayBox VenueDisplay = iota
	// Future: VenueDisplayList, VenueDisplayCompact, etc.
)

const (
	maxVenueBoxWidth  = 30
	maxVenueBoxHeight = 25
)

// Venue represents an aggregated working directory with session counts
type Venue struct {
	Directory     string
	RunningCount  int
	HistoryCount  int
	TotalCount    int
	PlanCount     int
	ResearchCount int
}

// VenueItemType distinguishes sessions from documents in the expanded view
type VenueItemType int

const (
	VenueItemSession VenueItemType = iota
	VenueItemDocument
)

// DocumentType distinguishes plans from research documents
type DocumentType int

const (
	DocTypePlan DocumentType = iota
	DocTypeResearch
)

// VenueItem represents a single item in the expanded venue list
type VenueItem struct {
	Type    VenueItemType
	Session *db.Session  // non-nil when Type == VenueItemSession
	DocPath string       // full path when Type == VenueItemDocument
	DocType DocumentType // plan or research
}

// buildVenues aggregates sessions by working directory
func buildVenues(sessions []*db.Session) []Venue {
	venueMap := make(map[string]*Venue)

	for _, s := range sessions {
		dir := s.WorkingDirectory
		if dir == "" {
			dir = "(unknown)"
		}

		v, ok := venueMap[dir]
		if !ok {
			v = &Venue{Directory: dir}
			venueMap[dir] = v
		}

		v.TotalCount++
		if s.Status == db.StatusWaiting || s.Status == db.StatusWorking {
			v.RunningCount++
		} else {
			v.HistoryCount++
		}
	}

	venues := make([]Venue, 0, len(venueMap))
	for _, v := range venueMap {
		venues = append(venues, *v)
	}

	// Count plan and research files for each venue
	for i := range venues {
		venues[i].PlanCount = countMdFiles(filepath.Join(venues[i].Directory, "thoughts", "shared", "plans"))
		venues[i].ResearchCount = countMdFiles(filepath.Join(venues[i].Directory, "thoughts", "shared", "research"))
	}

	// Sort: running count desc, then total count desc, then directory asc
	sort.Slice(venues, func(i, j int) bool {
		if venues[i].RunningCount != venues[j].RunningCount {
			return venues[i].RunningCount > venues[j].RunningCount
		}
		if venues[i].TotalCount != venues[j].TotalCount {
			return venues[i].TotalCount > venues[j].TotalCount
		}
		return venues[i].Directory < venues[j].Directory
	})

	return venues
}

// buildVenueItems creates the unified list of sessions and documents for a venue
func buildVenueItems(venue *Venue, sessions []*db.Session) []VenueItem {
	var items []VenueItem

	// Filter sessions for this venue's directory
	var runningSessions, historySessions []*db.Session
	for _, s := range sessions {
		dir := s.WorkingDirectory
		if dir == "" {
			dir = "(unknown)"
		}
		if dir != venue.Directory {
			continue
		}
		if s.Status == db.StatusWaiting || s.Status == db.StatusWorking {
			runningSessions = append(runningSessions, s)
		} else {
			historySessions = append(historySessions, s)
		}
	}

	// Add running sessions
	for _, s := range runningSessions {
		items = append(items, VenueItem{Type: VenueItemSession, Session: s})
	}

	// Add history sessions
	for _, s := range historySessions {
		items = append(items, VenueItem{Type: VenueItemSession, Session: s})
	}

	// Add plan documents
	planDir := filepath.Join(venue.Directory, "thoughts", "shared", "plans")
	for _, path := range listMdFiles(planDir) {
		items = append(items, VenueItem{Type: VenueItemDocument, DocPath: path, DocType: DocTypePlan})
	}

	// Add research documents
	researchDir := filepath.Join(venue.Directory, "thoughts", "shared", "research")
	for _, path := range listMdFiles(researchDir) {
		items = append(items, VenueItem{Type: VenueItemDocument, DocPath: path, DocType: DocTypeResearch})
	}

	return items
}

// renderVenueExpanded renders the expanded venue view with venue box top-right
// and a unified list of sessions + documents
func (d *Dashboard) renderVenueExpanded() string {
	if d.expandedVenue == nil {
		return ""
	}

	width := d.listWidth()
	height := d.contentHeight()

	// Render the venue box (top-right, fixed size)
	venueBoxWidth := 28
	venueBoxHeight := 5
	venueBox := renderVenueBox(*d.expandedVenue, venueBoxWidth, venueBoxHeight, true)

	// List panel takes the remaining width
	listWidth := width - venueBoxWidth - 3 // 3 for gap + borders
	if listWidth < 30 {
		listWidth = 30
	}

	// Build the list content with section headers
	var content strings.Builder
	cols := getVisibleColumns(listWidth)

	// Pre-count for section headers
	runningCount := 0
	historyCount := 0
	planCount := 0
	researchCount := 0
	for _, item := range d.expandedItems {
		switch {
		case item.Type == VenueItemSession && (item.Session.Status == db.StatusWaiting || item.Session.Status == db.StatusWorking):
			runningCount++
		case item.Type == VenueItemSession:
			historyCount++
		case item.Type == VenueItemDocument && item.DocType == DocTypePlan:
			planCount++
		case item.Type == VenueItemDocument && item.DocType == DocTypeResearch:
			researchCount++
		}
	}

	// Column header
	headerLine := d.formatHeaderLine(cols)
	content.WriteString(columnHeaderStyle.Render(headerLine))
	content.WriteString("\n")

	maxLines := height - 4
	lineCount := 1 // Header already rendered
	itemIdx := 0
	lastSection := -1

	for _, item := range d.expandedItems {
		if lineCount >= maxLines {
			break
		}

		// Determine which section this item belongs to
		newSection := -1
		switch {
		case item.Type == VenueItemSession && (item.Session.Status == db.StatusWaiting || item.Session.Status == db.StatusWorking):
			newSection = 0
		case item.Type == VenueItemSession:
			newSection = 1
		case item.Type == VenueItemDocument && item.DocType == DocTypePlan:
			newSection = 2
		case item.Type == VenueItemDocument && item.DocType == DocTypeResearch:
			newSection = 3
		}

		// Emit section header when entering a new section
		if newSection != lastSection {
			// Add blank line between sections (not before the first)
			if lastSection != -1 && lineCount < maxLines {
				content.WriteString("\n")
				lineCount++
			}

			var headerText string
			var headerStyle lipgloss.Style
			switch newSection {
			case 0:
				headerText = fmt.Sprintf("● RUNNING (%d)", runningCount)
				headerStyle = sectionActiveHeader
			case 1:
				headerText = fmt.Sprintf("○ HISTORY (%d)", historyCount)
				headerStyle = sectionHistoryHeader
			case 2:
				headerText = fmt.Sprintf("📝 PLANS (%d)", planCount)
				headerStyle = sectionVenuesHeader
			case 3:
				headerText = fmt.Sprintf("📜 RESEARCH (%d)", researchCount)
				headerStyle = sectionVenuesHeader
			}
			content.WriteString(headerStyle.Width(listWidth - 4).Render(headerText))
			content.WriteString("\n")
			lineCount++
			lastSection = newSection
		}

		isSelected := itemIdx == d.expandedSelected

		if item.Type == VenueItemSession {
			inHistory := !(item.Session.Status == db.StatusWaiting || item.Session.Status == db.StatusWorking)
			line := d.formatSessionLine(item.Session, cols, isSelected, inHistory)
			if isSelected {
				content.WriteString(selectionIndicatorStyle.Render(">") + " " + line + "\n")
			} else {
				content.WriteString("  " + line + "\n")
			}
		} else {
			// Document item — show filename without .md
			name := filepath.Base(item.DocPath)
			name = strings.TrimSuffix(name, ".md")
			if isSelected {
				content.WriteString(selectionIndicatorStyle.Render(">") + " " + name + "\n")
			} else {
				content.WriteString("  " + dimStyle.Render(name) + "\n")
			}
		}

		lineCount++
		itemIdx++
	}

	// Empty state
	if len(d.expandedItems) == 0 {
		content.WriteString(dimStyle.Render("  No sessions or documents") + "\n")
	}

	listStyle := panelStyle.Width(listWidth).Height(height)
	if d.focus == focusList {
		listStyle = focusedPanelStyle.Width(listWidth).Height(height)
	}

	listPanel := listStyle.Render(content.String())

	// Compose: list on left, venue box top-right
	rightPanel := lipgloss.NewStyle().Width(venueBoxWidth + 2).Render(venueBox)
	combined := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, " ", rightPanel)

	venueName := lastPathSegment(d.expandedVenue.Directory)
	title := titleStyle.Render(venueName)
	return lipgloss.JoinVertical(lipgloss.Left, title, combined)
}

// renderVenuesGrid renders venues in the specified display style
func (d *Dashboard) renderVenuesGrid(display VenueDisplay) string {
	switch display {
	case VenueDisplayBox:
		return d.renderVenuesBoxGrid()
	default:
		return d.renderVenuesBoxGrid()
	}
}

// renderVenuesBoxGrid renders venues as a grid of boxes
func (d *Dashboard) renderVenuesBoxGrid() string {
	venues := buildVenues(d.sessions)
	width := d.listWidth()
	height := d.contentHeight()

	if len(venues) == 0 {
		content := dimStyle.Render("  No venues yet")
		style := panelStyle.Width(width).Height(height)
		if d.focus == focusList {
			style = focusedPanelStyle.Width(width).Height(height)
		}
		title := titleStyle.Render("Venues")
		return lipgloss.JoinVertical(lipgloss.Left, title, style.Render(content))
	}

	// Calculate grid dimensions
	cols, boxWidth, boxHeight := venueGridDimensions(len(venues), width, height)
	d.venueGridCols = cols
	numRows := (len(venues) + cols - 1) / cols
	d.venueGridRows = numRows

	// Each rendered row takes boxHeight + 2 (border) lines, plus 1 line gap between rows
	rowRenderHeight := boxHeight + 2
	// Available content height inside the panel (minus panel border top/bottom)
	innerHeight := height - 2
	if innerHeight < rowRenderHeight {
		innerHeight = rowRenderHeight
	}

	// How many rows fit on screen
	// First row: rowRenderHeight. Each additional: 1 (gap) + rowRenderHeight.
	visibleRows := 1
	remaining := innerHeight - rowRenderHeight
	if remaining > 0 {
		visibleRows += remaining / (rowRenderHeight + 1)
	}
	if visibleRows > numRows {
		visibleRows = numRows
	}
	d.venueVisibleRows = visibleRows

	// Clamp scroll offset
	if d.venueScrollRow > numRows-visibleRows {
		d.venueScrollRow = numRows - visibleRows
	}
	if d.venueScrollRow < 0 {
		d.venueScrollRow = 0
	}

	// Build only the visible rows
	var rows []string
	for r := d.venueScrollRow; r < d.venueScrollRow+visibleRows && r < numRows; r++ {
		var rowBoxes []string
		for c := 0; c < cols; c++ {
			idx := r*cols + c
			if idx >= len(venues) {
				break
			}
			if c > 0 {
				hSpacer := lipgloss.NewStyle().Width(1).Render("")
				rowBoxes = append(rowBoxes, hSpacer)
			}
			isSelected := idx == d.selected
			box := renderVenueBox(venues[idx], boxWidth, boxHeight, isSelected)
			rowBoxes = append(rowBoxes, box)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, rowBoxes...))
	}

	content := strings.Join(rows, "\n")

	// Scroll indicator
	canScrollUp := d.venueScrollRow > 0
	canScrollDown := d.venueScrollRow+visibleRows < numRows
	scrollHint := ""
	if canScrollUp && canScrollDown {
		scrollHint = fmt.Sprintf(" ↑↓ %d/%d", d.venueScrollRow+1, numRows)
	} else if canScrollUp {
		scrollHint = fmt.Sprintf(" ↑ %d/%d", d.venueScrollRow+1, numRows)
	} else if canScrollDown {
		scrollHint = fmt.Sprintf(" ↓ %d/%d", d.venueScrollRow+1, numRows)
	}

	style := panelStyle.Width(width).Height(height)
	if d.focus == focusList {
		style = focusedPanelStyle.Width(width).Height(height)
	}
	titleText := fmt.Sprintf("Venues (%d)", len(venues))
	if scrollHint != "" {
		titleText += dimStyle.Render(scrollHint)
	}
	title := titleStyle.Render(titleText)
	return lipgloss.JoinVertical(lipgloss.Left, title, style.Render(content))
}

// venueGridDimensions calculates cols, boxWidth, boxHeight for a grid layout.
// Box height is fixed (minVenueBoxHeight) so content is always readable.
// If the grid doesn't fit vertically, the caller handles scrolling.
func venueGridDimensions(count, availWidth, availHeight int) (cols, boxWidth, boxHeight int) {
	const minBoxWidth = 10
	const minBoxHeight = 5 // border(2) + name(1) + counts(1) + padding(1)

	if count == 0 {
		return 1, maxVenueBoxWidth, minBoxHeight
	}

	// Max columns that fit: each column needs minBoxWidth + 1 gap (except first)
	maxCols := 1
	if availWidth-4 >= minBoxWidth {
		maxCols = (availWidth - 4 + 1) / (minBoxWidth + 1)
	}
	if maxCols < 1 {
		maxCols = 1
	}
	if maxCols > count {
		maxCols = count
	}

	// How many rows fit on screen with fixed box height
	rowRenderHeight := minBoxHeight + 2 // box content height + border
	maxVisibleRows := 1
	rem := (availHeight - 2) - rowRenderHeight // first row
	if rem > 0 {
		maxVisibleRows += rem / (rowRenderHeight + 1) // subsequent rows + 1-line gap
	}
	if maxVisibleRows < 1 {
		maxVisibleRows = 1
	}

	// Find column count that is roughly square but biased toward wider layouts.
	// Among similarly-square options, prefer more columns over more rows.
	bestCols := 1
	bestScore := count * count * 100 // worst case
	for c := 1; c <= maxCols; c++ {
		r := (count + c - 1) / c
		diff := c - r
		if diff < 0 {
			diff = -diff
		}
		// Base score: how far from square (lower is better)
		score := diff * 10
		// Bias toward wider: penalise extra rows more than extra columns.
		// When cols < rows, add extra penalty to push toward wider.
		if r > c {
			score += (r - c) * 5
		}
		// Tie-break: fewer empty cells in last row
		emptyCells := c*r - count
		score += emptyCells
		// Heavily penalise layouts that require scrolling
		if r > maxVisibleRows {
			score += (r - maxVisibleRows) * 100
		}
		if score < bestScore {
			bestScore = score
			bestCols = c
		}
	}
	cols = bestCols

	// Box width: divide available width among columns + 1-char gaps
	// c columns with (c-1) gaps of 1 char
	usableWidth := availWidth - 4
	boxWidth = (usableWidth - (cols - 1)) / cols
	if boxWidth > maxVenueBoxWidth {
		boxWidth = maxVenueBoxWidth
	}
	if boxWidth < minBoxWidth {
		boxWidth = minBoxWidth
	}

	boxHeight = minBoxHeight

	return cols, boxWidth, boxHeight
}

// listMdFiles returns the paths of .md files under a directory
func listMdFiles(dir string) []string {
	var files []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files
}

// countMdFiles counts .md files recursively under a directory
func countMdFiles(dir string) int {
	count := 0
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			count++
		}
		return nil
	})
	return count
}

// renderVenueBox renders a single venue as a styled box
func renderVenueBox(v Venue, width, height int, selected bool) string {
	name := lastPathSegment(v.Directory)

	style := venueBoxStyle.Width(width).Height(height)
	if selected {
		style = venueBoxSelectedStyle.Width(width).Height(height)
	}

	// Inner content width is box width minus border (1 char each side)
	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}

	// Inner content height is box height minus border (1 line each side)
	innerHeight := height - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	// Whether we have room for a second line (counts)
	hasCountsSpace := innerHeight >= 2

	if v.RunningCount > 0 {
		// Truncate name if needed to fit with " R" suffix
		maxName := innerWidth - 2 // space + "R"
		if maxName < 1 {
			maxName = 1
		}
		if len(name) > maxName {
			name = name[:maxName-1] + "…"
		}
		name = name + " " + statusWorking.Render("R")
	} else {
		if len(name) > innerWidth {
			name = name[:innerWidth-1] + "…"
		}
	}

	// Build counts line if any exist and there's vertical space
	if hasCountsSpace && (v.PlanCount > 0 || v.ResearchCount > 0) {
		countsLine := venueCountsLine(v.PlanCount, v.ResearchCount, innerWidth)
		if countsLine != "" {
			return style.Render(name + "\n" + dimStyle.Render(countsLine))
		}
	}

	return style.Render(name)
}

// venueCountsLine builds a counts string that fits within maxWidth.
// Emoji widths: each emoji occupies ~2 columns in most terminals.
// Format attempts in order: "📝 3 📜 2", "📝3 📜2", "📝3📜2", "3p 2r", "3p2r"
func venueCountsLine(plans, research, maxWidth int) string {
	if plans == 0 && research == 0 {
		return ""
	}

	// Build parts and try progressively more compact formats
	type candidate struct {
		text string
	}

	var candidates []candidate

	if plans > 0 && research > 0 {
		candidates = append(candidates,
			candidate{fmt.Sprintf("📝 %d 📜 %d", plans, research)},
			candidate{fmt.Sprintf("📝%d 📜%d", plans, research)},
			candidate{fmt.Sprintf("📝%d📜%d", plans, research)},
			candidate{fmt.Sprintf("%dp %dr", plans, research)},
			candidate{fmt.Sprintf("%dp%dr", plans, research)},
		)
	} else if plans > 0 {
		candidates = append(candidates,
			candidate{fmt.Sprintf("📝 %d", plans)},
			candidate{fmt.Sprintf("📝%d", plans)},
			candidate{fmt.Sprintf("%dp", plans)},
		)
	} else {
		candidates = append(candidates,
			candidate{fmt.Sprintf("📜 %d", research)},
			candidate{fmt.Sprintf("📜%d", research)},
			candidate{fmt.Sprintf("%dr", research)},
		)
	}

	for _, c := range candidates {
		// Estimate display width: each emoji is ~2 columns, rest is 1 per byte
		w := displayWidth(c.text)
		if w <= maxWidth {
			return c.text
		}
	}

	// Nothing fits
	return ""
}

// displayWidth estimates the terminal display width of a string.
// Emoji runes (>= 0x1F000) count as 2 columns; ASCII as 1.
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if r >= 0x1F000 {
			w += 2
		} else {
			w++
		}
	}
	return w
}

// lastPathSegment returns the last component of a file path
func lastPathSegment(path string) string {
	if path == "" || path == "(unknown)" {
		return path
	}
	path = strings.TrimRight(path, "/")
	idx := strings.LastIndex(path, "/")
	if idx >= 0 {
		return path[idx+1:]
	}
	return path
}

// shortenPath shortens a directory path to fit within maxWidth
func shortenPath(path string, maxWidth int) string {
	// Replace home directory with ~
	if strings.HasPrefix(path, "/Users/") {
		parts := strings.SplitN(path, "/", 4)
		if len(parts) >= 4 {
			path = "~/" + parts[3]
		}
	} else if strings.HasPrefix(path, "/home/") {
		parts := strings.SplitN(path, "/", 4)
		if len(parts) >= 4 {
			path = "~/" + parts[3]
		}
	}

	if len(path) <= maxWidth {
		return path
	}

	// Truncate from the left, showing the end of the path
	return "…" + path[len(path)-maxWidth+1:]
}
