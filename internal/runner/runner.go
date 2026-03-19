// Package runner provides shared PTY-based execution infrastructure for agent implementations.
package runner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"context"

	"github.com/creack/pty"
	"github.com/google/uuid"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
	"github.com/agentic-camerata/cmt/internal/tmux"
)

var defaultCapturedFileRe = regexp.MustCompile(`(thoughts/shared/\S+\.md)`)

const (
	// idleThreshold is how long without output before transitioning back to waiting
	idleThreshold = 1 * time.Second
	// autoTerminateThreshold is how long without output before killing the process
	autoTerminateThreshold = 5 * time.Second
)

// Base provides PTY-based execution infrastructure for agent implementations.
type Base struct {
	db        *db.DB
	outputDir string
}

// NewBase creates a new Base runner, ensuring the output directory exists.
func NewBase(database *db.DB) (*Base, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	outputDir := filepath.Join(home, ".config", "cmt", "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}

	return &Base{
		db:        database,
		outputDir: outputDir,
	}, nil
}

// Execute runs a pre-built command with PTY support, session tracking, and activity monitoring.
// The caller is responsible for building cmd (including setting any agent-specific flags).
func (b *Base) Execute(ctx context.Context, cmd *exec.Cmd, opts agent.RunOptions) error {
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
	cmd.Dir = workDir

	// Skip DB tracking for quick/ephemeral commands
	if opts.SkipTracking {
		return b.runWithPTY(ctx, cmd, nil, opts)
	}

	// Create session record
	sessionID := uuid.New().String()[:8]
	outputFile := filepath.Join(b.outputDir, sessionID+".log")

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
		ParentID:         opts.ParentID,
	}

	if opts.ResumeSessionID != "" && opts.ResumeSessionID != "*" {
		session.ClaudeSessionID = opts.ResumeSessionID
	}

	if err := b.db.CreateSession(session); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	// Run with PTY capture
	err = b.runWithPTY(ctx, cmd, session, opts)

	// When auto-terminate kills the process, cmd.Wait() returns a "signal: killed" error
	// which is expected and should be treated as successful completion
	if err != nil && !(opts.AutoTerminate && isKilledError(err)) {
		b.db.UpdateSessionStatus(sessionID, db.StatusAbandoned)
		return err
	}

	b.db.UpdateSessionStatus(sessionID, db.StatusCompleted)
	return nil
}

// activityMonitor tracks PTY output to detect working/waiting states
type activityMonitor struct {
	sessionID     string
	db            *db.DB
	lastOutput    time.Time
	isWorking     bool
	hasWorked     bool
	autoTerminate bool
	terminated    bool
	process       *os.Process
	mu            sync.Mutex
	done          chan struct{}
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

func (m *activityMonitor) onOutput() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastOutput = time.Now()
	m.terminated = false
	if !m.isWorking {
		m.isWorking = true
		m.hasWorked = true
		m.db.UpdateSessionStatus(m.sessionID, db.StatusWorking)
	}
}

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
				idle := time.Since(m.lastOutput)
				if m.isWorking && idle > idleThreshold {
					m.isWorking = false
					m.db.UpdateSessionStatus(m.sessionID, db.StatusWaiting)
				}
				if m.autoTerminate && m.hasWorked && !m.terminated && idle > autoTerminateThreshold && m.process != nil {
					m.terminated = true
					m.process.Kill()
				}
				m.mu.Unlock()
			}
		}
	}()
}

func (m *activityMonitor) stop() {
	close(m.done)
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

	restoreTerminal(os.Stdin.Fd(), s.oldState)
	s.oldState = nil

	if s.childPID > 0 {
		syscall.Kill(-s.childPID, syscall.SIGSTOP)
	}

	syscall.Kill(0, syscall.SIGSTOP)

	s.resume()
}

func (s *suspendState) resume() {
	newState, err := makeRaw(os.Stdin.Fd())
	if err == nil {
		s.oldState = newState
	}

	if s.childPID > 0 {
		syscall.Kill(-s.childPID, syscall.SIGCONT)
	}

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
func (b *Base) runWithPTY(ctx context.Context, cmd *exec.Cmd, session *db.Session, opts agent.RunOptions) error {
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("start pty: %w", err)
	}
	defer ptmx.Close()

	var outFile *os.File
	var monitor *activityMonitor
	if session != nil {
		if cmd.Process != nil {
			b.db.UpdateSessionPID(session.ID, cmd.Process.Pid)
		}

		monitor = newActivityMonitor(session.ID, b.db)
		monitor.autoTerminate = opts.AutoTerminate
		monitor.process = cmd.Process
		monitor.start()
		defer monitor.stop()

		var err error
		outFile, err = os.Create(session.OutputFile)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer outFile.Close()
	}

	ss := &suspendState{
		ptmx:     ptmx,
		childPID: cmd.Process.Pid,
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				// Ignore resize errors
			}
		}
	}()
	ch <- syscall.SIGWINCH

	sigCont := make(chan os.Signal, 1)
	signal.Notify(sigCont, syscall.SIGCONT)
	go func() {
		for range sigCont {
			ss.mu.Lock()
			if ss.oldState == nil && isTerminal(os.Stdin.Fd()) {
				ss.resume()
			}
			ss.mu.Unlock()
		}
	}()
	defer signal.Stop(sigCont)

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

	done := make(chan struct{})
	defer close(done)

	capturedSeen := map[string]bool{}

	// PTY output -> stdout + file, with activity detection and file capture
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				writeAll(os.Stdout, buf[:n])
				if outFile != nil {
					outFile.Write(buf[:n])
				}
				if monitor != nil {
					monitor.onOutput()
				}

				if opts.CapturedFiles != nil {
					re := defaultCapturedFileRe
					if opts.CapturePattern != nil {
						re = opts.CapturePattern
					}
					matches := re.FindAllString(string(buf[:n]), -1)
					for _, m := range matches {
						if !capturedSeen[m] {
							capturedSeen[m] = true
							*opts.CapturedFiles = append(*opts.CapturedFiles, m)
						}
					}
				}
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
			select {
			case <-done:
				return
			default:
			}
			start := 0
			for i := 0; i < n; i++ {
				if buf[i] == 0x1a {
					if i > start {
						writeAll(ptmx, buf[start:i])
					}
					ss.suspend()
					start = i + 1
				}
			}
			if start < n {
				writeAll(ptmx, buf[start:n])
			}
		}
	}()

	return cmd.Wait()
}

func writeAll(w io.Writer, data []byte) {
	for len(data) > 0 {
		n, err := w.Write(data)
		data = data[n:]
		if err != nil {
			return
		}
	}
}

func isKilledError(err error) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return status.Signal() == syscall.SIGKILL
		}
	}
	return false
}

// shellescape wraps s in single quotes, escaping any embedded single quotes.
func shellescape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// isLinux reports whether the current platform is Linux.
func isLinux() bool {
	return runtime.GOOS == "linux"
}

// PaneSession represents an agent session running in a background tmux pane.
type PaneSession struct {
	paneID    string
	sessionID string
	logFile   string
	db        *db.DB
}

// ExecuteInPane starts a pre-built command in a new background tmux pane (non-blocking).
// Output is piped through tee so it appears in the pane and is saved to the session log.
// The pane does not steal focus. Call PaneSession.Wait to wait for completion.
func (b *Base) ExecuteInPane(ctx context.Context, cmd *exec.Cmd, opts agent.RunOptions) (*PaneSession, error) {
	if err := tmux.RequireTmux(); err != nil {
		return nil, fmt.Errorf("cmt requires tmux: %w", err)
	}

	workDir := opts.WorkingDir
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	sessionID := uuid.New().String()[:8]
	logFile := filepath.Join(b.outputDir, sessionID+".log")
	prefix := os.Getenv("CMT_PREFIX")

	// Build shell command: cd to workdir, run agent CLI with output captured to log file.
	// We use `script` to allocate a PTY so the child process (e.g. claude -p) sees an
	// interactive terminal and streams output in real time instead of buffering.
	parts := make([]string, len(cmd.Args))
	for i, arg := range cmd.Args {
		parts[i] = shellescape(arg)
	}
	agentCmd := strings.Join(parts, " ")
	var shellCmd string
	if isLinux() {
		// Linux: script -qfc <command> <logfile>
		shellCmd = fmt.Sprintf("cd %s && script -qfc %s %s",
			shellescape(workDir),
			shellescape(agentCmd),
			shellescape(logFile))
	} else {
		// macOS: script -q <logfile> <command...>
		shellCmd = fmt.Sprintf("cd %s && script -q %s %s",
			shellescape(workDir),
			shellescape(logFile),
			agentCmd)
	}

	fmt.Fprintf(os.Stderr, "[debug] shellCmd: %s\n", shellCmd)
	paneID, err := tmux.NewBackgroundPane(shellCmd)
	if err != nil {
		return nil, fmt.Errorf("create background pane: %w", err)
	}

	// Resolve the new pane's location (not the caller's) so cmt jump works correctly.
	loc, err := tmux.PaneLocation(paneID)
	if err != nil {
		tmux.KillPane(paneID) //nolint:errcheck
		return nil, fmt.Errorf("get pane location: %w", err)
	}

	session := &db.Session{
		ID:               sessionID,
		WorkflowType:     opts.WorkflowType,
		Status:           db.StatusWorking,
		WorkingDirectory: workDir,
		TaskDescription:  opts.TaskDescription,
		Prefix:           prefix,
		TmuxSession:      loc.Session,
		TmuxWindow:       loc.Window,
		TmuxPane:         loc.Pane,
		OutputFile:       logFile,
		ParentID:         opts.ParentID,
	}
	if err := b.db.CreateSession(session); err != nil {
		tmux.KillPane(paneID) //nolint:errcheck
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &PaneSession{
		paneID:    paneID,
		sessionID: sessionID,
		logFile:   logFile,
		db:        b.db,
	}, nil
}

// WaitOptions configures behavior of Wait.
type WaitOptions struct {
	KeepPane bool // If true, don't kill the pane after completion (for debugging).
}

// Wait polls until the background pane exits, collects captured files, and cleans up.
func (ps *PaneSession) Wait(ctx context.Context, capturePattern *regexp.Regexp, wopts ...WaitOptions) ([]string, error) {
	keepPane := len(wopts) > 0 && wopts[0].KeepPane

	for {
		select {
		case <-ctx.Done():
			if !keepPane {
				tmux.KillPane(ps.paneID) //nolint:errcheck
			}
			ps.db.UpdateSessionStatus(ps.sessionID, db.StatusAbandoned)
			return nil, ctx.Err()
		default:
		}
		if !tmux.IsPaneAlive(ps.paneID) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !keepPane {
		tmux.KillPane(ps.paneID) //nolint:errcheck
	}

	// Parse log file for captured file paths
	var captured []string
	if data, err := os.ReadFile(ps.logFile); err == nil {
		re := defaultCapturedFileRe
		if capturePattern != nil {
			re = capturePattern
		}
		seen := map[string]bool{}
		for _, m := range re.FindAllString(string(data), -1) {
			if !seen[m] {
				seen[m] = true
				captured = append(captured, m)
			}
		}
	}

	ps.db.UpdateSessionStatus(ps.sessionID, db.StatusCompleted)
	return captured, nil
}
