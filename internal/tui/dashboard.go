package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/agentic-camerata/cmt/internal/db"
	"github.com/agentic-camerata/cmt/internal/tmux"
)

const (
	focusList = iota
	focusInfo
)

const (
	viewNormal = iota
	viewTrash
	viewVenues
	viewVenueExpanded
	viewTodos
)

// ViewVenues is the exported constant for venues view mode (for CLI flag)
const ViewVenues = viewVenues

// ViewTodos is the exported constant for todos view mode (for CLI flag)
const ViewTodos = viewTodos

// Dashboard is the main TUI model
type Dashboard struct {
	db       *db.DB
	sessions []*db.Session
	todos    []*db.Todo
	selected int
	focus    int

	// Components
	infoViewport viewport.Model

	// Dimensions
	width  int
	height int

	// State
	err        error
	loading    bool
	jumpTarget *db.Session // Session to jump to after quit
	showInfo      bool // Toggle info pane visibility
	viewMode      int  // viewNormal or viewTrash
	venueGridCols   int // Current venue grid column count (set during render)
	venueGridRows   int // Total venue grid rows (set during render)
	venueVisibleRows int // How many rows fit on screen (set during render)
	venueScrollRow  int // First visible row in venue grid (for scrolling)

	// Expanded venue state
	expandedVenue     *Venue        // The venue being viewed
	expandedItems     []VenueItem   // Unified list of sessions + documents
	expandedSelected  int           // Selected index in the unified list
	expandedScrollOff int           // Scroll offset for the list
	showDocViewer     bool          // Whether the document viewer is visible
	docViewport       viewport.Model // Viewport for document content
}

// NewDashboard creates a new dashboard model
func NewDashboard(database *db.DB) *Dashboard {
	return &Dashboard{
		db:       database,
		focus:    focusList,
		loading:  true,
		showInfo: false,
	}
}

// DebugRender loads data synchronously and returns the rendered view for debugging.
func (d *Dashboard) DebugRender(width, height int) string {
	d.width = width
	d.height = height

	// Load data synchronously
	if msg, ok := d.loadSessions().(sessionsLoadedMsg); ok {
		d.sessions = sortSessions(msg.sessions)
		d.err = msg.err
	}
	if msg, ok := d.loadTodos().(todosLoadedMsg); ok {
		d.todos = msg.todos
	}

	d.loading = false
	d.infoViewport = viewport.New(d.infoWidth(), d.infoHeight())
	d.updateInfoContent()

	return d.View()
}

// JumpTarget returns the session to jump to (if any) after the dashboard exits
func (d *Dashboard) JumpTarget() *db.Session {
	return d.jumpTarget
}

// SetViewMode sets the initial view mode
func (d *Dashboard) SetViewMode(mode int) {
	d.viewMode = mode
}

// sessionsLoadedMsg is sent when sessions are loaded
type sessionsLoadedMsg struct {
	sessions []*db.Session
	err      error
}

// todosLoadedMsg is sent when todos are loaded
type todosLoadedMsg struct {
	todos []*db.Todo
	err   error
}

// tickMsg triggers periodic updates
type tickMsg time.Time

// pruneCompletedMsg is sent when pruning is done
type pruneCompletedMsg struct {
	count int64
	err   error
}

// Init initializes the dashboard
func (d *Dashboard) Init() tea.Cmd {
	return tea.Batch(
		d.pruneDeletedSessions,
		d.loadSessions,
		d.loadTodos,
		d.tick(),
	)
}

// pruneDeletedSessions removes old deleted sessions
func (d *Dashboard) pruneDeletedSessions() tea.Msg {
	count, err := d.db.PruneDeletedSessions()
	return pruneCompletedMsg{count: count, err: err}
}

// loadTodos fetches all todos from the database
func (d *Dashboard) loadTodos() tea.Msg {
	todos, err := d.db.ListTodos("") // all statuses
	return todosLoadedMsg{todos: todos, err: err}
}

// loadSessions fetches sessions from the database based on view mode
func (d *Dashboard) loadSessions() tea.Msg {
	var sessions []*db.Session
	var err error

	switch d.viewMode {
	case viewTrash:
		sessions, err = d.db.ListDeletedSessions()
	default:
		sessions, err = d.db.ListSessions("")
	}
	return sessionsLoadedMsg{sessions: sessions, err: err}
}

// tick returns a command that ticks periodically
func (d *Dashboard) tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update handles messages
func (d *Dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return d, tea.Quit

		case "tab":
			if d.showInfo || (d.viewMode == viewVenueExpanded && d.showDocViewer) {
				d.focus = (d.focus + 1) % 2
			}

		case "j", "down":
			if d.focus == focusList {
				if d.viewMode == viewVenueExpanded {
					if d.expandedSelected < len(d.expandedItems)-1 {
						d.expandedSelected++
						if d.showDocViewer {
							d.updateDocViewerContent()
						}
						if d.showInfo {
							d.updateInfoContent()
						}
					}
				} else if d.viewMode == viewVenues && d.venueGridCols > 0 {
					newSel := d.selected + d.venueGridCols
					if newSel < d.listLen() {
						d.selected = newSel
						d.ensureVenueSelectionVisible()
					}
				} else if d.selected < d.listLen()-1 {
					d.selected++
					d.updateInfoContent()
				}
			} else if d.focus == focusInfo {
				if d.viewMode == viewVenueExpanded {
					var cmd tea.Cmd
					d.docViewport, cmd = d.docViewport.Update(msg)
					cmds = append(cmds, cmd)
				} else {
					var cmd tea.Cmd
					d.infoViewport, cmd = d.infoViewport.Update(msg)
					cmds = append(cmds, cmd)
				}
			}

		case "k", "up":
			if d.focus == focusList {
				if d.viewMode == viewVenueExpanded {
					if d.expandedSelected > 0 {
						d.expandedSelected--
						if d.showDocViewer {
							d.updateDocViewerContent()
						}
						if d.showInfo {
							d.updateInfoContent()
						}
					}
				} else if d.viewMode == viewVenues && d.venueGridCols > 0 {
					newSel := d.selected - d.venueGridCols
					if newSel >= 0 {
						d.selected = newSel
						d.ensureVenueSelectionVisible()
					}
				} else if d.selected > 0 {
					d.selected--
					d.updateInfoContent()
				}
			} else if d.focus == focusInfo {
				if d.viewMode == viewVenueExpanded {
					var cmd tea.Cmd
					d.docViewport, cmd = d.docViewport.Update(msg)
					cmds = append(cmds, cmd)
				} else {
					var cmd tea.Cmd
					d.infoViewport, cmd = d.infoViewport.Update(msg)
					cmds = append(cmds, cmd)
				}
			}

		case "h", "left":
			if d.viewMode == viewVenues && d.focus == focusList && d.selected > 0 && d.venueGridCols > 0 {
				if d.selected%d.venueGridCols > 0 {
					d.selected--
					d.ensureVenueSelectionVisible()
				}
			}

		case "l", "right":
			if d.viewMode == viewVenues && d.focus == focusList && d.selected < d.listLen()-1 && d.venueGridCols > 0 {
				if d.selected%d.venueGridCols < d.venueGridCols-1 {
					d.selected++
					d.ensureVenueSelectionVisible()
				}
			}

		case "enter":
			if d.viewMode == viewVenues && d.focus == focusList {
				d.enterVenueExpanded()
			} else if d.viewMode == viewVenueExpanded && d.focus == focusList {
				// Jump to session if selected item is a session
				if d.expandedSelected < len(d.expandedItems) {
					item := d.expandedItems[d.expandedSelected]
					if item.Type == VenueItemSession && item.Session != nil {
						loc := tmux.Location{
							Session: item.Session.TmuxSession,
							Window:  item.Session.TmuxWindow,
							Pane:    item.Session.TmuxPane,
						}
						tmux.JumpTo(loc)
					}
				}
			} else if d.viewMode == viewNormal && d.focus == focusList && len(d.sessions) > 0 && d.selected < len(d.sessions) {
				// Jump to session - only in normal view (can't jump to deleted sessions)
				session := d.sessions[d.selected]
				loc := tmux.Location{
					Session: session.TmuxSession,
					Window:  session.TmuxWindow,
					Pane:    session.TmuxPane,
				}
				// Jump to the pane - dashboard stays running in its pane
				tmux.JumpTo(loc)
			} else if d.viewMode == viewTodos && d.focus == focusList {
				items := sortedTodos(d.todos)
				if d.selected < len(items) {
					item := items[d.selected]
					if item.Status == db.TodoStatusTodo {
						item.Status = db.TodoStatusDone
					} else {
						item.Status = db.TodoStatusTodo
					}
					d.db.UpdateTodo(item)
					cmds = append(cmds, d.loadTodos)
				}
			}

		case "esc", "backspace":
			if d.viewMode == viewVenueExpanded {
				d.viewMode = viewVenues
				d.showDocViewer = false
				d.focus = focusList
				// d.selected is still at the venue that was expanded
			} else if d.viewMode == viewTodos {
				d.viewMode = viewNormal
				d.showInfo = false
				d.focus = focusList
			}

		case "o":
			if d.viewMode == viewTodos && d.focus == focusList {
				items := sortedTodos(d.todos)
				if d.selected < len(items) {
					item := items[d.selected]
					if item.URL != nil {
						openURL(*item.URL)
					}
				}
			} else if d.viewMode == viewVenueExpanded {
				if d.expandedSelected < len(d.expandedItems) {
					item := d.expandedItems[d.expandedSelected]
					if item.Type == VenueItemDocument {
						d.showDocViewer = !d.showDocViewer
						if d.showDocViewer {
							d.docViewport = viewport.New(d.infoWidth(), d.infoHeight())
							d.updateDocViewerContent()
							d.focus = focusInfo
						} else {
							d.focus = focusList
						}
					}
				}
			}

		case "r":
			// Refresh sessions
			cmds = append(cmds, d.loadSessions)

		case "i":
			if d.viewMode == viewVenues {
				break // No info panel for venues
			}
			// Toggle info pane
			d.showInfo = !d.showInfo
			if !d.showInfo && d.focus == focusInfo {
				d.focus = focusList
			}
			// Resize viewport when toggling
			d.infoViewport = viewport.New(d.infoWidth(), d.infoHeight())
			d.updateInfoContent()

		case "s":
			// Stop (kill) selected session - only in normal view
			if d.viewMode == viewNormal && d.focus == focusList && len(d.sessions) > 0 && d.selected < len(d.sessions) {
				session := d.sessions[d.selected]
				// Only kill if running
				if session.Status == db.StatusWaiting || session.Status == db.StatusWorking {
					if session.PID > 0 {
						syscall.Kill(session.PID, syscall.SIGKILL)
					}
					d.db.UpdateSessionStatus(session.ID, db.StatusKilled)
					cmds = append(cmds, d.loadSessions)
				}
			}

		case "D":
			// Delete (soft-delete) selected session - only in normal view
			if d.viewMode == viewNormal && d.focus == focusList && len(d.sessions) > 0 && d.selected < len(d.sessions) {
				session := d.sessions[d.selected]
				// Stop process if running before deleting
				if session.PID > 0 && (session.Status == db.StatusWaiting || session.Status == db.StatusWorking) {
					syscall.Kill(session.PID, syscall.SIGKILL)
				}
				d.db.SoftDeleteSession(session.ID)
				cmds = append(cmds, d.loadSessions)
			} else if d.viewMode == viewTodos && d.focus == focusList {
				items := sortedTodos(d.todos)
				if d.selected < len(items) {
					d.db.DeleteTodo(items[d.selected].ID)
					cmds = append(cmds, d.loadTodos)
				}
			}

		case "T":
			// Toggle trash view
			if d.viewMode == viewNormal {
				d.viewMode = viewTrash
			} else {
				d.viewMode = viewNormal
			}
			d.selected = 0 // Reset selection when switching views
			cmds = append(cmds, d.loadSessions)

		case "R":
			// Restore selected session - only in trash view
			if d.viewMode == viewTrash && d.focus == focusList && len(d.sessions) > 0 && d.selected < len(d.sessions) {
				session := d.sessions[d.selected]
				d.db.RestoreSession(session.ID)
				cmds = append(cmds, d.loadSessions)
			}

		case "V":
			// Toggle venues view
			if d.viewMode == viewVenues {
				d.viewMode = viewNormal
			} else {
				d.viewMode = viewVenues
			}
			d.selected = 0
			d.venueScrollRow = 0
			cmds = append(cmds, d.loadSessions)

		case "t":
			d.viewMode = viewTodos
			d.selected = 0
			d.showInfo = false
			d.focus = focusList
		}

	case tea.WindowSizeMsg:
		d.width = msg.Width
		d.height = msg.Height
		d.infoViewport = viewport.New(d.infoWidth(), d.infoHeight())
		d.docViewport = viewport.New(d.infoWidth(), d.infoHeight())
		d.updateInfoContent()
		if d.showDocViewer {
			d.updateDocViewerContent()
		}

	case pruneCompletedMsg:
		// Pruning is silent - we don't show errors for this background task
		// Could optionally log msg.count if needed

	case sessionsLoadedMsg:
		d.loading = false
		d.err = msg.err
		// Sort sessions: active first, then by created_at desc
		d.sessions = sortSessions(msg.sessions)
		// Clamp selection if list shrunk
		if d.selected >= len(d.sessions) && len(d.sessions) > 0 {
			d.selected = len(d.sessions) - 1
		}
		d.updateInfoContent()
		// Rebuild expanded items if viewing a venue
		if d.viewMode == viewVenueExpanded && d.expandedVenue != nil {
			d.expandedItems = buildVenueItems(d.expandedVenue, d.sessions)
			if d.expandedSelected >= len(d.expandedItems) && len(d.expandedItems) > 0 {
				d.expandedSelected = len(d.expandedItems) - 1
			}
		}

	case todosLoadedMsg:
		if msg.err == nil {
			d.todos = msg.todos
			if d.viewMode == viewTodos {
				sortedLen := len(sortedTodos(d.todos))
				if d.selected >= sortedLen && sortedLen > 0 {
					d.selected = sortedLen - 1
				}
				d.updateInfoContent()
			}
		}

	case tickMsg:
		// Periodic refresh of sessions and todos
		cmds = append(cmds, d.loadSessions)
		cmds = append(cmds, d.loadTodos)
		cmds = append(cmds, d.tick())
	}

	return d, tea.Batch(cmds...)
}

// updateInfoContent updates the info panel content based on selected session
func (d *Dashboard) updateInfoContent() {
	// In expanded venue view, show info for the selected expanded item
	if d.viewMode == viewVenueExpanded {
		d.updateExpandedInfoContent()
		return
	}

	if d.viewMode == viewTodos {
		items := sortedTodos(d.todos)
		if d.selected < len(items) {
			d.infoViewport.SetContent(d.formatTodoInfo(items[d.selected]))
		} else {
			d.infoViewport.SetContent("No item selected")
		}
		return
	}

	if len(d.sessions) == 0 || d.selected >= len(d.sessions) {
		d.infoViewport.SetContent("No session selected")
		return
	}

	session := d.sessions[d.selected]
	d.infoViewport.SetContent(d.formatSessionInfo(session))
}

// updateExpandedInfoContent updates the info panel for the expanded venue view
func (d *Dashboard) updateExpandedInfoContent() {
	if d.expandedSelected >= len(d.expandedItems) {
		d.infoViewport.SetContent("No item selected")
		return
	}

	item := d.expandedItems[d.expandedSelected]
	switch item.Type {
	case VenueItemSession:
		if item.Session != nil {
			d.infoViewport.SetContent(d.formatSessionInfo(item.Session))
		} else {
			d.infoViewport.SetContent("No session data")
		}
	case VenueItemDocument:
		var content strings.Builder
		content.WriteString(fmt.Sprintf("Path: %s\n", item.DocPath))
		content.WriteString(fmt.Sprintf("Name: %s\n", filepath.Base(item.DocPath)))
		docType := "Plan"
		if item.DocType == DocTypeResearch {
			docType = "Research"
		}
		content.WriteString(fmt.Sprintf("Type: %s\n", docType))
		content.WriteString("\n")
		content.WriteString("─── Content ──────────────────────────────\n")
		content.WriteString("\n")
		data, err := os.ReadFile(item.DocPath)
		if err != nil {
			content.WriteString(fmt.Sprintf("Error reading file: %v", err))
		} else {
			content.Write(data)
		}
		d.infoViewport.SetContent(content.String())
	}
}

// formatSessionInfo formats session details for the info panel
func (d *Dashboard) formatSessionInfo(session *db.Session) string {
	var content strings.Builder
	content.WriteString(fmt.Sprintf("ID:                %s\n", session.ID))
	content.WriteString(fmt.Sprintf("Status:            %s\n", session.Status))
	content.WriteString(fmt.Sprintf("Workflow:          %s\n", session.WorkflowType))
	content.WriteString(fmt.Sprintf("Working Directory: %s\n", session.WorkingDirectory))
	content.WriteString(fmt.Sprintf("Prefix:            %s\n", session.Prefix))
	content.WriteString(fmt.Sprintf("Created:           %s\n", session.CreatedAt.Format(time.RFC3339)))
	content.WriteString(fmt.Sprintf("Updated:           %s\n", session.UpdatedAt.Format(time.RFC3339)))
	content.WriteString(fmt.Sprintf("Claude Session ID: %s\n", session.ClaudeSessionID))
	content.WriteString(fmt.Sprintf("Tmux Location:     %s:%d.%d\n", session.TmuxSession, session.TmuxWindow, session.TmuxPane))
	content.WriteString(fmt.Sprintf("Output File:       %s\n", session.OutputFile))
	content.WriteString(fmt.Sprintf("PID:               %d\n", session.PID))

	// Show parent info if this is a child session
	if session.ParentID != "" {
		content.WriteString(fmt.Sprintf("Parent ID:         %s\n", session.ParentID))
		content.WriteString("\n")
		content.WriteString("─── Parent (Play Session) ────────────────\n")
		content.WriteString("\n")
		if parent, err := d.db.GetSession(session.ParentID); err == nil && parent != nil {
			content.WriteString(fmt.Sprintf("Parent Status:     %s\n", parent.Status))
			content.WriteString(fmt.Sprintf("Playbook:          %s\n", parent.TaskDescription))
			content.WriteString(fmt.Sprintf("Parent Created:    %s\n", parent.CreatedAt.Format(time.RFC3339)))
			content.WriteString(fmt.Sprintf("Parent Tmux:       %s:%d.%d\n", parent.TmuxSession, parent.TmuxWindow, parent.TmuxPane))
		} else {
			content.WriteString("(Parent session not found)\n")
		}
	}

	content.WriteString("\n")
	content.WriteString("─── Prompt ───────────────────────────────\n")
	content.WriteString("\n")
	if session.TaskDescription != "" {
		content.WriteString(session.TaskDescription)
	} else {
		content.WriteString("(No prompt)")
	}
	return content.String()
}

// View renders the dashboard
func (d *Dashboard) View() string {
	if d.width == 0 {
		return "Loading..."
	}

	// Header
	header := titleStyle.Render("Agentic Camerata")

	// Error display
	if d.err != nil {
		return lipgloss.JoinVertical(lipgloss.Left,
			header,
			errorStyle.Render(fmt.Sprintf("Error: %v", d.err)),
		)
	}

	// Session list panel
	listPanel := d.renderSessionList()

	// Help bar
	help := d.renderHelp()

	// Doc viewer panel (expanded venue view)
	if d.viewMode == viewVenueExpanded && d.showDocViewer {
		docPanel := d.renderDocViewer()
		return lipgloss.JoinVertical(lipgloss.Left, header, listPanel, docPanel, help)
	}

	// Info panel at the bottom (when visible)
	if d.showInfo {
		infoPanel := d.renderInfoPanel()
		return lipgloss.JoinVertical(lipgloss.Left, header, listPanel, infoPanel, help)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, listPanel, help)
}

// Column widths for session table
const (
	colIDWidth       = 10
	colStatusWidth   = 11
	colWorkflowWidth = 10
	colAgeWidth      = 6
	colPrefixWidth   = 20
)

// visibleColumns determines which columns to show based on available width
type visibleColumns struct {
	id          bool
	status      bool
	workflow    bool
	age         bool
	prefix      bool
	prompt      bool
	promptWidth int // dynamic width for prompt column
}

// getVisibleColumns returns which columns fit in the given width
func getVisibleColumns(width int) visibleColumns {
	// Account for padding, borders, indentation
	available := width - 6

	cols := visibleColumns{id: true} // ID always visible

	// Add columns from left to right if they fit
	used := colIDWidth

	if used+1+colStatusWidth <= available {
		cols.status = true
		used += 1 + colStatusWidth
	}

	if used+1+colWorkflowWidth <= available {
		cols.workflow = true
		used += 1 + colWorkflowWidth
	}

	if used+1+colAgeWidth+2 <= available { // +2 for spacing before prefix
		cols.age = true
		used += 1 + colAgeWidth + 2
	}

	if used+1+colPrefixWidth <= available {
		cols.prefix = true
		used += 1 + colPrefixWidth
	}

	// Prompt column gets remaining space (minimum 10 chars)
	remaining := available - used - 1 // -1 for separator
	if remaining >= 10 {
		cols.prompt = true
		cols.promptWidth = remaining
	}

	return cols
}

// renderSessionList renders the session list panel
func (d *Dashboard) renderSessionList() string {
	width := d.listWidth()
	height := d.contentHeight()

	var content strings.Builder

	// Venues view has its own rendering
	if d.viewMode == viewVenues {
		return d.renderVenuesGrid(VenueDisplayBox)
	}

	// Expanded venue view
	if d.viewMode == viewVenueExpanded {
		return d.renderVenueExpanded()
	}

	// Todos view
	if d.viewMode == viewTodos {
		return d.renderTodosList()
	}

	// Determine visible columns based on width
	cols := getVisibleColumns(width)

	// Determine content based on view mode
	if d.viewMode == viewTrash {
		// Trash view - single section with all deleted sessions
		if len(d.sessions) == 0 {
			content.WriteString(dimStyle.Render("  Trash is empty") + "\n")
		} else {
			// Column header
			headerLine := d.formatHeaderLine(cols)
			content.WriteString(columnHeaderStyle.Render(headerLine))
			content.WriteString("\n")

			lineCount := 1 // Header already rendered
			maxLines := height - 4

			sectionHeader := sectionTrashHeader.Width(width - 4).Render(fmt.Sprintf("🗑 TRASH (%d)", len(d.sessions)))
			content.WriteString(sectionHeader)
			content.WriteString("\n")
			lineCount++

			for i, s := range d.sessions {
				if lineCount >= maxLines {
					content.WriteString(fmt.Sprintf("  ... and %d more\n", len(d.sessions)-i))
					break
				}

				isSelected := i == d.selected
				line := d.formatSessionLine(s, cols, isSelected, true) // Always dim in trash

				if isSelected {
					content.WriteString(selectionIndicatorStyle.Render(">") + " " + line + "\n")
				} else {
					content.WriteString("  " + line + "\n")
				}
				lineCount++
			}
		}
	} else {
		// Normal view - split into running and history sections
		if len(d.sessions) == 0 {
			content.WriteString("No sessions yet.\n")
			content.WriteString("Start one with: cmt new")
		} else {
			// Column header
			headerLine := d.formatHeaderLine(cols)
			content.WriteString(columnHeaderStyle.Render(headerLine))
			content.WriteString("\n")

			// Split sessions into running (waiting/working) and history
			var runningSessions, historySessions []*db.Session
			for _, s := range d.sessions {
				if s.Status == db.StatusWaiting || s.Status == db.StatusWorking {
					runningSessions = append(runningSessions, s)
				} else {
					historySessions = append(historySessions, s)
				}
			}

			lineCount := 1 // Header already rendered
			maxLines := height - 4

			// Render running sessions section
			sectionHeader := sectionActiveHeader.Width(width - 4).Render(fmt.Sprintf("● RUNNING (%d)", len(runningSessions)))
			content.WriteString(sectionHeader)
			content.WriteString("\n")
			lineCount++

			if len(runningSessions) > 0 {
				for i, s := range runningSessions {
					if lineCount >= maxLines {
						content.WriteString(fmt.Sprintf("  ... and %d more\n", len(runningSessions)-i))
						lineCount++
						break
					}

					globalIdx := i
					isSelected := globalIdx == d.selected
					line := d.formatSessionLine(s, cols, isSelected, false)

					if isSelected {
						content.WriteString(selectionIndicatorStyle.Render(">") + " " + line + "\n")
					} else {
						content.WriteString("  " + line + "\n")
					}
					lineCount++
				}
			} else {
				content.WriteString(dimStyle.Render("  No running sessions") + "\n")
				lineCount++
			}

			// Render history section
			if lineCount < maxLines {
				sectionHeader := sectionHistoryHeader.Width(width - 4).Render(fmt.Sprintf("○ HISTORY (%d)", len(historySessions)))
				content.WriteString(sectionHeader)
				content.WriteString("\n")
				lineCount++

				if len(historySessions) > 0 {
					for i, s := range historySessions {
						if lineCount >= maxLines {
							content.WriteString(fmt.Sprintf("  ... and %d more\n", len(historySessions)-i))
							break
						}

						globalIdx := len(runningSessions) + i
						isSelected := globalIdx == d.selected
						line := d.formatSessionLine(s, cols, isSelected, true)

						if isSelected {
							content.WriteString(selectionIndicatorStyle.Render(">") + " " + line + "\n")
						} else {
							content.WriteString("  " + line + "\n")
						}
						lineCount++
					}
				} else {
					content.WriteString(dimStyle.Render("  No completed sessions") + "\n")
				}
			}
		}
	}

	style := panelStyle.Width(width).Height(height)
	if d.focus == focusList {
		style = focusedPanelStyle.Width(width).Height(height)
	}

	var title string
	if d.viewMode == viewTrash {
		title = titleStyle.Render("Trash")
	} else {
		title = titleStyle.Render("Sessions")
	}
	return lipgloss.JoinVertical(lipgloss.Left, title, style.Render(content.String()))
}

// formatHeaderLine formats the column header based on visible columns
func (d *Dashboard) formatHeaderLine(cols visibleColumns) string {
	var parts []string

	if cols.id {
		parts = append(parts, fmt.Sprintf("%-*s", colIDWidth, "ID"))
	}
	if cols.status {
		parts = append(parts, fmt.Sprintf("%-*s", colStatusWidth, "STATUS"))
	}
	if cols.workflow {
		parts = append(parts, fmt.Sprintf("%-*s", colWorkflowWidth, "WORKFLOW"))
	}
	if cols.age {
		parts = append(parts, fmt.Sprintf("%*s  ", colAgeWidth, "AGE"))
	}
	if cols.prefix {
		parts = append(parts, fmt.Sprintf("%-*s", colPrefixWidth, "PREFIX"))
	}
	if cols.prompt {
		parts = append(parts, fmt.Sprintf("%-*s", cols.promptWidth, "     PROMPT"))
	}

	return strings.Join(parts, " ")
}

// formatSessionLine formats a single session for display with aligned columns
// When selected is true, colors are kept but without full-row background change
// When inHistory is true, dimmed color variants are used
func (d *Dashboard) formatSessionLine(s *db.Session, cols visibleColumns, selected bool, inHistory bool) string {
	// Truncate or pad ID
	id := s.ID
	if len(id) > colIDWidth {
		id = id[:colIDWidth-1] + "…"
	}

	statusStr := string(s.Status)
	workflowStr := string(s.WorkflowType)
	age := formatAge(s.CreatedAt)

	// Truncate prefix to fit column width
	prefix := s.Prefix
	if cols.prefix && len(prefix) > colPrefixWidth {
		prefix = prefix[:colPrefixWidth-1] + "…"
	}

	// Truncate prompt to fit column width (account for 5-char indent)
	prompt := s.TaskDescription
	if cols.prompt {
		maxPrompt := cols.promptWidth - 5
		if maxPrompt > 0 && len(prompt) > maxPrompt {
			prompt = prompt[:maxPrompt-1] + "…"
		}
	}
	// Replace newlines with spaces for single-line display
	prompt = strings.ReplaceAll(prompt, "\n", " ")

	// Format with colors - use dimmed variants for history section
	var parts []string
	if cols.id {
		if inHistory {
			parts = append(parts, dimIDStyle.Render(fmt.Sprintf("%-*s", colIDWidth, id)))
		} else {
			parts = append(parts, fmt.Sprintf("%-*s", colIDWidth, id))
		}
	}
	if cols.status {
		var status string
		if inHistory {
			status = StatusStyleDim(statusStr).Render(fmt.Sprintf("%-*s", colStatusWidth, statusStr))
		} else {
			status = StatusStyle(statusStr).Render(fmt.Sprintf("%-*s", colStatusWidth, statusStr))
		}
		parts = append(parts, status)
	}
	if cols.workflow {
		var workflow string
		if inHistory {
			workflow = WorkflowStyleDim(workflowStr).Render(fmt.Sprintf("%-*s", colWorkflowWidth, workflowStr))
		} else {
			workflow = WorkflowStyle(workflowStr).Render(fmt.Sprintf("%-*s", colWorkflowWidth, workflowStr))
		}
		parts = append(parts, workflow)
	}
	if cols.age {
		if inHistory {
			parts = append(parts, dimAgeStyle.Render(fmt.Sprintf("%*s", colAgeWidth, age))+"  ")
		} else {
			parts = append(parts, fmt.Sprintf("%*s  ", colAgeWidth, age))
		}
	}
	if cols.prefix {
		if inHistory {
			parts = append(parts, dimStyle.Render(fmt.Sprintf("%-*s", colPrefixWidth, prefix)))
		} else {
			parts = append(parts, fmt.Sprintf("%-*s", colPrefixWidth, prefix))
		}
	}
	if cols.prompt {
		parts = append(parts, dimStyle.Render("     "+prompt))
	}

	return strings.Join(parts, " ")
}

// renderInfoPanel renders the additional info panel
func (d *Dashboard) renderInfoPanel() string {
	width := d.infoWidth()
	height := d.infoHeight()

	style := panelStyle.Width(width).Height(height)
	if d.focus == focusInfo {
		style = focusedPanelStyle.Width(width).Height(height)
	}

	title := titleStyle.Render("Additional Info")
	return lipgloss.JoinVertical(lipgloss.Left, title, style.Render(d.infoViewport.View()))
}

// updateDocViewerContent reads the selected document and sets the viewport content
func (d *Dashboard) updateDocViewerContent() {
	if d.expandedSelected >= len(d.expandedItems) {
		d.docViewport.SetContent("No document selected")
		return
	}

	item := d.expandedItems[d.expandedSelected]
	if item.Type != VenueItemDocument {
		d.docViewport.SetContent("Select a document to view")
		return
	}

	data, err := os.ReadFile(item.DocPath)
	if err != nil {
		d.docViewport.SetContent(fmt.Sprintf("Error reading file: %v", err))
		return
	}

	d.docViewport.SetContent(string(data))
}

// renderDocViewer renders the document viewer panel at the bottom
func (d *Dashboard) renderDocViewer() string {
	width := d.infoWidth()
	height := d.infoHeight()

	style := panelStyle.Width(width).Height(height)
	if d.focus == focusInfo {
		style = focusedPanelStyle.Width(width).Height(height)
	}

	titleText := "Document Viewer"
	if d.expandedSelected < len(d.expandedItems) {
		item := d.expandedItems[d.expandedSelected]
		if item.Type == VenueItemDocument {
			titleText = filepath.Base(item.DocPath)
		}
	}

	title := titleStyle.Render(titleText)
	return lipgloss.JoinVertical(lipgloss.Left, title, style.Render(d.docViewport.View()))
}

// renderHelp renders the help bar
func (d *Dashboard) renderHelp() string {
	var help string
	switch d.viewMode {
	case viewTrash:
		help = "j/k: navigate • R: restore • T: back to sessions • i: toggle info • r: refresh • q: quit"
	case viewVenues:
		help = "h/j/k/l: navigate • enter: expand • V: back to sessions • r: refresh • q: quit"
	case viewTodos:
		help = "j/k: navigate • enter: toggle done • o: open url • D: delete • i: toggle info • esc: back • r: refresh • q: quit"
	case viewVenueExpanded:
		if d.showDocViewer {
			help = "j/k: navigate • tab: switch focus • o: close viewer • enter: jump • esc: back • q: quit"
		} else {
			help = "j/k: navigate • enter: jump • o: view doc • esc: back to venues • r: refresh • q: quit"
		}
	default:
		help = "j/k: navigate • enter: jump • s: stop • D: delete • T: trash • V: venues • i: toggle info • r: refresh • q: quit"
	}
	return helpStyle.Render(help)
}

// listLen returns the number of items in the current view
func (d *Dashboard) listLen() int {
	if d.viewMode == viewVenues {
		return len(buildVenues(d.sessions))
	}
	if d.viewMode == viewVenueExpanded {
		return len(d.expandedItems)
	}
	if d.viewMode == viewTodos {
		return len(sortedTodos(d.todos))
	}
	return len(d.sessions)
}

// ensureVenueSelectionVisible adjusts venueScrollRow so the selected venue's
// row is within the visible window.
func (d *Dashboard) ensureVenueSelectionVisible() {
	if d.venueGridCols <= 0 {
		return
	}
	selectedRow := d.selected / d.venueGridCols
	if selectedRow < d.venueScrollRow {
		d.venueScrollRow = selectedRow
	}
	if d.venueVisibleRows > 0 && selectedRow >= d.venueScrollRow+d.venueVisibleRows {
		d.venueScrollRow = selectedRow - d.venueVisibleRows + 1
	}
}

// enterVenueExpanded transitions from venue grid to expanded venue view
func (d *Dashboard) enterVenueExpanded() {
	venues := buildVenues(d.sessions)
	if d.selected >= len(venues) {
		return
	}
	venue := venues[d.selected]
	d.expandedVenue = &venue
	d.expandedItems = buildVenueItems(&venue, d.sessions)
	d.expandedSelected = 0
	d.expandedScrollOff = 0
	d.showDocViewer = false
	d.viewMode = viewVenueExpanded
}

// Layout calculations
func (d *Dashboard) listWidth() int {
	return d.width - 2 // Full width minus borders
}

func (d *Dashboard) infoWidth() int {
	return d.width - 4 // Full width minus borders/padding
}

func (d *Dashboard) contentHeight() int {
	// Chrome lines always present:
	//   1 - "Agentic Camerata" header
	//   1 - "Sessions" title
	//   2 - session panel border (top + bottom)
	//   1 - help bar
	// = 5 total
	if d.viewMode == viewVenueExpanded && d.showDocViewer {
		// Doc viewer gets up to 80% of available space; list gets the rest
		chrome := 8
		available := d.height - chrome
		listHeight := available - (available * 80 / 100)
		if listHeight < 5 {
			listHeight = 5
		}
		return listHeight
	}
	if d.showInfo {
		// Additional chrome when info panel is shown:
		//   1 - "Additional Info" title
		//   2 - info panel border (top + bottom)
		// = 3 more, so 8 total chrome
		chrome := 8
		minInfoHeight := 18
		listHeight := d.height - chrome - minInfoHeight
		if listHeight < 5 {
			listHeight = 5
		}
		return listHeight
	}
	return d.height - 5
}

func (d *Dashboard) infoHeight() int {
	// Total chrome = 8 (see contentHeight)
	// Info panel content height = total - chrome - listContentHeight
	height := d.height - 8 - d.contentHeight()
	if height < 3 {
		height = 3
	}
	return height
}

// formatAge returns a human-readable age string
func formatAge(t time.Time) string {
	d := time.Since(t)

	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// openURL opens a URL in the default browser
func openURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	cmd.Start()
}

// sortSessions sorts sessions with running (waiting/working) first, then by creation time descending
func sortSessions(sessions []*db.Session) []*db.Session {
	if len(sessions) == 0 {
		return sessions
	}

	sorted := make([]*db.Session, len(sessions))
	copy(sorted, sessions)

	sort.Slice(sorted, func(i, j int) bool {
		// Running sessions (waiting or working) come first
		iRunning := sorted[i].Status == db.StatusWaiting || sorted[i].Status == db.StatusWorking
		jRunning := sorted[j].Status == db.StatusWaiting || sorted[j].Status == db.StatusWorking

		if iRunning != jRunning {
			return iRunning // Running comes before non-running
		}

		// Within same status group, sort by created_at descending (newer first)
		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})

	return sorted
}
