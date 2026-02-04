package claude

import (
	"context"
	"fmt"
	"io"
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
	ContinueSession string // Session ID to continue (or "last")
	WorkingDir      string // Override working directory
	Model           string // Model to use (e.g., "sonnet", "opus")
	PrintMode       bool   // If true, print response and exit (non-interactive)
	AutonomousMode  bool   // If true, skip permission prompts
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

	if err := r.db.CreateSession(session); err != nil {
		return fmt.Errorf("create session: %w", err)
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

// Continue continues an existing session
func (r *Runner) Continue(ctx context.Context, sessionID string, autonomousMode bool) error {
	var session *db.Session
	var err error

	if sessionID == "last" {
		session, err = r.db.GetLastSession()
	} else {
		session, err = r.db.GetSession(sessionID)
	}

	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Build continue command
	return r.Run(ctx, RunOptions{
		Command:         CommandContinue,
		WorkflowType:    session.WorkflowType,
		TaskDescription: session.TaskDescription,
		ContinueSession: session.ClaudeSessionID,
		WorkingDir:      session.WorkingDirectory,
		AutonomousMode:  autonomousMode,
	})
}

// buildCommand constructs the claude CLI command
func (r *Runner) buildCommand(opts RunOptions) *exec.Cmd {
	args := []string{}

	// Continue existing session
	if opts.ContinueSession != "" {
		args = append(args, "-c")
	}

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

	// Add task as positional argument (prompt) if provided
	if opts.TaskDescription != "" && opts.ContinueSession == "" {
		task := opts.TaskDescription
		if prefix := GetPromptPrefix(opts.Command); prefix != "" {
			task = prefix + " " + task
		}
		args = append(args, task)
	}

	return exec.Command("claude", args...)
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

	// Set stdin to raw mode (only if stdin is a terminal)
	var oldState interface{}
	if isTerminal(os.Stdin.Fd()) {
		oldState, err = makeRaw(os.Stdin.Fd())
		if err != nil {
			return fmt.Errorf("set raw mode: %w", err)
		}
		defer restoreTerminal(os.Stdin.Fd(), oldState)
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

	// stdin -> PTY
	go func() {
		io.Copy(ptmx, os.Stdin)
	}()

	// Wait for command to complete
	return cmd.Wait()
}

