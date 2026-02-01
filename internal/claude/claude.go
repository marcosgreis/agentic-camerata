package claude

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/google/uuid"

	"github.com/agentic-camerata/cmt/internal/db"
	"github.com/agentic-camerata/cmt/internal/tmux"
)

const (
	// idleThreshold is how long without output before transitioning back to waiting
	idleThreshold = 1 * time.Second
)

// Runner manages Claude CLI execution
type Runner struct {
	db        *db.DB
	outputDir string
}

// NewRunner creates a new Claude runner
func NewRunner(database *db.DB) (*Runner, error) {
	// Create output directory
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	outputDir := filepath.Join(home, ".config", "cmt", "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}

	return &Runner{
		db:        database,
		outputDir: outputDir,
	}, nil
}

// RunOptions configures a Claude session
type RunOptions struct {
	Command         CommandType
	WorkflowType    db.WorkflowType
	TaskDescription string
	WorkingDir      string // Override working directory
	Model           string // Model to use (e.g., "sonnet", "opus")
	PrintMode       bool   // If true, print response and exit (non-interactive)
	AutonomousMode  bool   // If true, skip permission prompts
	CommentTag      string // Comment tag for look-and-fix (from CMT_COMMENT_TAG env var)
	ResumeSessionID string // If non-empty, pass --resume to claude. "*" means no specific ID (interactive picker)
}

// activityMonitor tracks PTY output to detect working/waiting states
type activityMonitor struct {
	sessionID  string
	db         *db.DB
	lastOutput time.Time
	isWorking  bool
	mu         sync.Mutex
	done       chan struct{}
}

func newActivityMonitor(sessionID string, database *db.DB) *activityMonitor {
	return &activityMonitor{
		sessionID:  sessionID,
		db:         database,
		lastOutput: time.Now(),
		isWorking:  false,
		done:       make(chan struct{}),
	}
}

// onOutput is called when PTY output is detected
func (m *activityMonitor) onOutput() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastOutput = time.Now()
	if !m.isWorking {
		m.isWorking = true
		m.db.UpdateSessionStatus(m.sessionID, db.StatusWorking)
	}
}

// start begins the idle detection loop
func (m *activityMonitor) start() {
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-m.done:
				return
			case <-ticker.C:
				m.mu.Lock()
				if m.isWorking && time.Since(m.lastOutput) > idleThreshold {
					m.isWorking = false
					m.db.UpdateSessionStatus(m.sessionID, db.StatusWaiting)
				}
				m.mu.Unlock()
			}
		}
	}()
}

// stop stops the activity monitor
func (m *activityMonitor) stop() {
	close(m.done)
}

// Run starts a Claude session with tracking
func (r *Runner) Run(ctx context.Context, opts RunOptions) error {
	// Require tmux
	if err := tmux.RequireTmux(); err != nil {
		return fmt.Errorf("cmt requires tmux: %w", err)
	}

	// Get current tmux location
	loc, err := tmux.CurrentLocation()
	if err != nil {
		return fmt.Errorf("get tmux location: %w", err)
	}

	// Determine working directory
	workDir := opts.WorkingDir
	if workDir == "" {
		workDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
	}

	// Create session record
	sessionID := uuid.New().String()[:8]
	outputFile := filepath.Join(r.outputDir, sessionID+".log")

	// Capture CMT_PREFIX environment variable
	prefix := os.Getenv("CMT_PREFIX")

	session := &db.Session{
		ID:               sessionID,
		WorkflowType:     opts.WorkflowType,
		Status:           db.StatusWaiting,
		WorkingDirectory: workDir,
		TaskDescription:  opts.TaskDescription,
		Prefix:           prefix,
		TmuxSession:      loc.Session,
		TmuxWindow:       loc.Window,
		TmuxPane:         loc.Pane,
		OutputFile:       outputFile,
	}

	// Store resumed Claude session ID if resuming a specific session
	if opts.ResumeSessionID != "" && opts.ResumeSessionID != "*" {
		session.ClaudeSessionID = opts.ResumeSessionID
	}

	if err := r.db.CreateSession(session); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	// Set comment tag from environment if not already set
	if opts.CommentTag == "" {
		opts.CommentTag = os.Getenv("CMT_COMMENT_TAG")
	}

	// Build command
	cmd := r.buildCommand(opts)
	cmd.Dir = workDir

	// Run with PTY capture
	err = r.runWithPTY(ctx, cmd, session)

	// Update session status based on result
	if err != nil {
		r.db.UpdateSessionStatus(sessionID, db.StatusAbandoned)
		return err
	}

	r.db.UpdateSessionStatus(sessionID, db.StatusCompleted)
	return nil
}

// buildCommand constructs the claude CLI command
func (r *Runner) buildCommand(opts RunOptions) *exec.Cmd {
	args := []string{}

	// Model selection
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}

	// Print mode (non-interactive, single response)
	if opts.PrintMode {
		args = append(args, "-p")
	}

	// Autonomous mode (skip permission prompts)
	if opts.AutonomousMode {
		args = append(args, "--dangerously-skip-permissions")
	}

	// Resume mode
	if opts.ResumeSessionID != "" {
		if opts.ResumeSessionID == "*" {
			args = append(args, "--resume")
		} else {
			args = append(args, "--resume", opts.ResumeSessionID)
		}
	}

	// Add task as positional argument (prompt) if provided
	if opts.TaskDescription != "" {
		task := opts.TaskDescription
		if prefix := GetPromptPrefix(opts.Command, opts.CommentTag); prefix != "" {
			task = prefix + " " + task
		}
		args = append(args, task)
	}

	return exec.Command("claude", args...)
}

// suspendState holds state needed for suspend/resume coordination
type suspendState struct {
	mu       sync.Mutex
	oldState interface{}
	ptmx     *os.File
	childPID int
}

func (s *suspendState) suspend() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Restore terminal to cooked mode
	restoreTerminal(os.Stdin.Fd(), s.oldState)
	s.oldState = nil

	// 2. Stop the child process group
	if s.childPID > 0 {
		syscall.Kill(-s.childPID, syscall.SIGSTOP)
	}

	// 3. Stop ourselves (execution pauses here until SIGCONT)
	syscall.Kill(0, syscall.SIGSTOP)

	// 4. Resumed — re-enter raw mode
	s.resume()
}

func (s *suspendState) resume() {
	// Re-enter raw mode
	newState, err := makeRaw(os.Stdin.Fd())
	if err == nil {
		s.oldState = newState
	}

	// Resume child process group
	if s.childPID > 0 {
		syscall.Kill(-s.childPID, syscall.SIGCONT)
	}

	// Re-sync PTY size
	if s.ptmx != nil {
		pty.InheritSize(os.Stdin, s.ptmx)
	}
}

func (s *suspendState) getOldState() interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.oldState
}

func (s *suspendState) setOldState(state interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.oldState = state
}

// runWithPTY runs the command with a pseudo-terminal for interactive use
func (r *Runner) runWithPTY(ctx context.Context, cmd *exec.Cmd, session *db.Session) error {
	// Start with PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("start pty: %w", err)
	}
	defer ptmx.Close()

	// Update session with PID
	if cmd.Process != nil {
		r.db.UpdateSessionPID(session.ID, cmd.Process.Pid)
	}

	// Create activity monitor
	monitor := newActivityMonitor(session.ID, r.db)
	monitor.start()
	defer monitor.stop()

	// Open output file for logging
	outFile, err := os.Create(session.OutputFile)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer outFile.Close()

	// Initialize suspend state
	ss := &suspendState{
		ptmx:     ptmx,
		childPID: cmd.Process.Pid,
	}

	// Handle terminal resize
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				// Ignore resize errors
			}
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize

	// Handle SIGCONT (resume after external suspension)
	sigCont := make(chan os.Signal, 1)
	signal.Notify(sigCont, syscall.SIGCONT)
	go func() {
		for range sigCont {
			ss.mu.Lock()
			if ss.oldState == nil && isTerminal(os.Stdin.Fd()) {
				// We were suspended without going through our suspend path
				// (e.g., kill -STOP / kill -CONT from another terminal)
				ss.resume()
			}
			ss.mu.Unlock()
		}
	}()
	defer signal.Stop(sigCont)

	// Set stdin to raw mode (only if stdin is a terminal)
	if isTerminal(os.Stdin.Fd()) {
		state, err := makeRaw(os.Stdin.Fd())
		if err != nil {
			return fmt.Errorf("set raw mode: %w", err)
		}
		ss.setOldState(state)
		defer func() {
			restoreTerminal(os.Stdin.Fd(), ss.getOldState())
		}()
	}

	// Copy I/O with activity monitoring
	// PTY output -> stdout + file, with activity detection
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				monitor.onOutput()
				os.Stdout.Write(buf[:n])
				outFile.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()

	// stdin -> PTY (with Ctrl+Z interception)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}
			// Scan for Ctrl+Z (0x1a) bytes
			start := 0
			for i := 0; i < n; i++ {
				if buf[i] == 0x1a {
					// Write bytes before Ctrl+Z to PTY
					if i > start {
						ptmx.Write(buf[start:i])
					}
					// Suspend
					ss.suspend()
					// Skip the 0x1a byte, continue after it
					start = i + 1
				}
			}
			// Write remaining bytes after last Ctrl+Z (or all bytes if no Ctrl+Z)
			if start < n {
				ptmx.Write(buf[start:n])
			}
		}
	}()

	// Wait for command to complete
	return cmd.Wait()
}

