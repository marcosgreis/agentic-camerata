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

const maxVenueBoxWidth = 30

// Venue represents an aggregated working directory with session counts
type Venue struct {
	Directory     string
	RunningCount  int
	HistoryCount  int
	TotalCount    int
	PlanCount     int
	ResearchCount int
	OtherCount    int  // Count of .md files in thoughts/ excluding plans/research
	Pinned        bool // true if this venue was explicitly pinned
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
	DocTypeOther
)

// VenueItem represents a single item in the expanded venue list
type VenueItem struct {
	Type      VenueItemType
	Session   *db.Session  // non-nil when Type == VenueItemSession
	Depth     int          // indent level for sessions (0 = top-level)
	InRunning bool         // which section the session belongs to
	DocPath   string       // full path when Type == VenueItemDocument
	DocType   DocumentType // plan or research
}

// buildVenues aggregates sessions by working directory and merges pinned venues
func buildVenues(sessions []*db.Session, pinnedDirs []string) []Venue {
	venueMap := make(map[string]*Venue)

	// Initialize pinned venues first so they appear even with 0 sessions
	for _, dir := range pinnedDirs {
		venueMap[dir] = &Venue{Directory: dir, Pinned: true}
	}

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

	// Count plan, research, and other files for each venue
	for i := range venues {
		venues[i].PlanCount = countMdFiles(filepath.Join(venues[i].Directory, "thoughts", "shared", "plans"))
		venues[i].ResearchCount = countMdFiles(filepath.Join(venues[i].Directory, "thoughts", "shared", "research"))
		venues[i].OtherCount = countOtherMdFiles(venues[i].Directory)
	}

	// Sort: pinned first, then running count desc, then total count desc, then directory asc
	sort.Slice(venues, func(i, j int) bool {
		if venues[i].Pinned != venues[j].Pinned {
			return venues[i].Pinned
		}
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
	var venueSessions []*db.Session
	for _, s := range sessions {
		dir := s.WorkingDirectory
		if dir == "" {
			dir = "(unknown)"
		}
		if dir != venue.Directory {
			continue
		}
		venueSessions = append(venueSessions, s)
	}

	// Add other documents first (thoughts/ excluding plans/ and research/)
	for _, path := range listOtherMdFiles(venue.Directory) {
		items = append(items, VenueItem{Type: VenueItemDocument, DocPath: path, DocType: DocTypeOther})
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

	// Build tree-ordered nodes for hierarchy display (sessions at the end)
	nodes := buildSessionTree(venueSessions)
	for _, n := range nodes {
		items = append(items, VenueItem{
			Type:      VenueItemSession,
			Session:   n.session,
			Depth:     n.depth,
			InRunning: n.inRunning,
		})
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
	otherCount := 0
	for _, item := range d.expandedItems {
		switch {
		case item.Type == VenueItemSession && item.InRunning:
			runningCount++
		case item.Type == VenueItemSession:
			historyCount++
		case item.Type == VenueItemDocument && item.DocType == DocTypePlan:
			planCount++
		case item.Type == VenueItemDocument && item.DocType == DocTypeResearch:
			researchCount++
		case item.Type == VenueItemDocument && item.DocType == DocTypeOther:
			otherCount++
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
		// Order: Other (0), Plans (1), Research (2), History (3)
		newSection := -1
		switch {
		case item.Type == VenueItemDocument && item.DocType == DocTypeOther:
			newSection = 0
		case item.Type == VenueItemDocument && item.DocType == DocTypePlan:
			newSection = 1
		case item.Type == VenueItemDocument && item.DocType == DocTypeResearch:
			newSection = 2
		case item.Type == VenueItemSession:
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
				headerText = fmt.Sprintf("📄 OTHER (%d)", otherCount)
				headerStyle = sectionVenuesHeader
			case 1:
				headerText = fmt.Sprintf("📝 PLANS (%d)", planCount)
				headerStyle = sectionVenuesHeader
			case 2:
				headerText = fmt.Sprintf("📜 RESEARCH (%d)", researchCount)
				headerStyle = sectionVenuesHeader
			case 3:
				sessionCount := runningCount + historyCount
				headerText = fmt.Sprintf("○ HISTORY (%d)", sessionCount)
				headerStyle = sectionHistoryHeader
			}
			content.WriteString(headerStyle.Width(listWidth - 4).Render(headerText))
			content.WriteString("\n")
			lineCount++
			lastSection = newSection
		}

		isSelected := itemIdx == d.expandedSelected

		if item.Type == VenueItemSession {
			inHistory := !isRunning(item.Session)
			line := d.formatSessionLine(item.Session, cols, isSelected, inHistory, item.Depth)
			if isSelected {
				indicator := selectedRowStyle.Render("> ")
				content.WriteString(indicator + line + "\n")
			} else {
				content.WriteString("  " + line + "\n")
			}
		} else {
			// Document item — show filename without .md
			name := filepath.Base(item.DocPath)
			name = strings.TrimSuffix(name, ".md")
			if isSelected {
				indicator := selectedRowStyle.Render("> ")
				content.WriteString(indicator + selectedRowStyle.Render(name) + "\n")
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
	venues := buildVenues(d.sessions, d.pinnedVenues)
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

	// Each rendered row takes boxHeight + 2 (border) + 3 (gap newlines) lines
	// But the last row doesn't need the trailing gap
	rowRenderHeight := boxHeight + 2 + 2
	// Available content height inside the panel (minus panel border top/bottom)
	innerHeight := height - 2
	if innerHeight < rowRenderHeight {
		innerHeight = rowRenderHeight
	}

	// How many rows fit on screen
	visibleRows := innerHeight / rowRenderHeight
	if visibleRows < 1 {
		visibleRows = 1
	}
	if visibleRows > numRows {
		visibleRows = numRows
	}
	d.venueVisibleRows = visibleRows

	// Clamp scroll offset
	maxScroll := numRows - visibleRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if d.venueScrollRow > maxScroll {
		d.venueScrollRow = maxScroll
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

	content := strings.Join(rows, "\n\n\n")

	// Scroll indicator
	canScrollUp := d.venueScrollRow > 0
	canScrollDown := d.venueScrollRow+visibleRows < numRows
	scrollHint := ""
	if canScrollUp && canScrollDown {
		scrollHint = fmt.Sprintf(" ↑↓ %d/%d", d.selected+1, numRows)
	} else if canScrollUp {
		scrollHint = fmt.Sprintf(" ↑ %d/%d", d.selected+1, numRows)
	} else if canScrollDown {
		scrollHint = fmt.Sprintf(" ↓ %d/%d", d.selected+1, numRows)
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

// venueGridDimensions calculates cols, boxWidth, boxHeight for a list layout.
// Always returns 1 column (one venue per row).
func venueGridDimensions(count, availWidth, availHeight int) (cols, boxWidth, boxHeight int) {
	const minBoxHeight = 5 // border(2) + name(1) + counts(1) + padding(1)

	cols = 1

	boxWidth = availWidth - 4
	if boxWidth > maxVenueBoxWidth {
		boxWidth = maxVenueBoxWidth
	}
	if boxWidth < 10 {
		boxWidth = 10
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

// listOtherMdFiles returns .md files under thoughts/ excluding plans/ and research/
func listOtherMdFiles(venueDir string) []string {
	thoughtsDir := filepath.Join(venueDir, "thoughts")
	plansDir := filepath.Join(venueDir, "thoughts", "shared", "plans")
	researchDir := filepath.Join(venueDir, "thoughts", "shared", "research")

	var files []string
	filepath.Walk(thoughtsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if path == plansDir || path == researchDir {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), ".md") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files
}

// countOtherMdFiles counts .md files under thoughts/ excluding plans/ and research/
func countOtherMdFiles(venueDir string) int {
	return len(listOtherMdFiles(venueDir))
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

	// Add pin marker for pinned venues (📌 is 2 display cols + 1 space = 3)
	if v.Pinned && len(name)+3 <= innerWidth {
		name = "📌 " + name
	}

	if v.RunningCount > 0 {
		// Truncate name if needed to fit with " R" suffix
		maxName := innerWidth - 2 // space + "R"
		if maxName < 1 {
			maxName = 1
		}
		if displayWidth(name) > maxName {
			name = name[:maxName-1] + "…"
		}
		name = name + " " + statusWorking.Render("R")
	} else {
		if displayWidth(name) > innerWidth {
			name = name[:innerWidth-1] + "…"
		}
	}

	// Build counts line if any exist and there's vertical space
	if hasCountsSpace && (v.PlanCount > 0 || v.ResearchCount > 0 || v.OtherCount > 0) {
		countsLine := venueCountsLine(v.PlanCount, v.ResearchCount, v.OtherCount, innerWidth)
		if countsLine != "" {
			return style.Render(name + "\n" + dimStyle.Render(countsLine))
		}
	}

	return style.Render(name)
}

// venueCountsLine builds a counts string that fits within maxWidth.
// Emoji widths: each emoji occupies ~2 columns in most terminals.
// Format attempts in order: "📝 3 📜 2 📄 1", "📝3 📜2 📄1", etc.
func venueCountsLine(plans, research, other, maxWidth int) string {
	if plans == 0 && research == 0 && other == 0 {
		return ""
	}

	type candidate struct {
		text string
	}

	var candidates []candidate

	if plans > 0 && research > 0 && other > 0 {
		candidates = append(candidates,
			candidate{fmt.Sprintf("📝 %d 📜 %d 📄 %d", plans, research, other)},
			candidate{fmt.Sprintf("📝%d 📜%d 📄%d", plans, research, other)},
			candidate{fmt.Sprintf("📝%d📜%d📄%d", plans, research, other)},
			candidate{fmt.Sprintf("%dp %dr %do", plans, research, other)},
			candidate{fmt.Sprintf("%dp%dr%do", plans, research, other)},
		)
	} else if plans > 0 && research > 0 {
		candidates = append(candidates,
			candidate{fmt.Sprintf("📝 %d 📜 %d", plans, research)},
			candidate{fmt.Sprintf("📝%d 📜%d", plans, research)},
			candidate{fmt.Sprintf("📝%d📜%d", plans, research)},
			candidate{fmt.Sprintf("%dp %dr", plans, research)},
			candidate{fmt.Sprintf("%dp%dr", plans, research)},
		)
	} else if plans > 0 && other > 0 {
		candidates = append(candidates,
			candidate{fmt.Sprintf("📝 %d 📄 %d", plans, other)},
			candidate{fmt.Sprintf("📝%d 📄%d", plans, other)},
			candidate{fmt.Sprintf("📝%d📄%d", plans, other)},
			candidate{fmt.Sprintf("%dp %do", plans, other)},
			candidate{fmt.Sprintf("%dp%do", plans, other)},
		)
	} else if research > 0 && other > 0 {
		candidates = append(candidates,
			candidate{fmt.Sprintf("📜 %d 📄 %d", research, other)},
			candidate{fmt.Sprintf("📜%d 📄%d", research, other)},
			candidate{fmt.Sprintf("📜%d📄%d", research, other)},
			candidate{fmt.Sprintf("%dr %do", research, other)},
			candidate{fmt.Sprintf("%dr%do", research, other)},
		)
	} else if plans > 0 {
		candidates = append(candidates,
			candidate{fmt.Sprintf("📝 %d", plans)},
			candidate{fmt.Sprintf("📝%d", plans)},
			candidate{fmt.Sprintf("%dp", plans)},
		)
	} else if research > 0 {
		candidates = append(candidates,
			candidate{fmt.Sprintf("📜 %d", research)},
			candidate{fmt.Sprintf("📜%d", research)},
			candidate{fmt.Sprintf("%dr", research)},
		)
	} else {
		candidates = append(candidates,
			candidate{fmt.Sprintf("📄 %d", other)},
			candidate{fmt.Sprintf("📄%d", other)},
			candidate{fmt.Sprintf("%do", other)},
		)
	}

	for _, c := range candidates {
		w := displayWidth(c.text)
		if w <= maxWidth {
			return c.text
		}
	}

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
