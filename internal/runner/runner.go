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

// claudeSessionIDRe matches the Claude session ID in PTY output (e.g. "session_01ABC...")
var claudeSessionIDRe = regexp.MustCompile(`session_([A-Za-z0-9]{10,})`)

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
	var err error

	// Best-effort tmux location (empty values when not in tmux)
	var tmuxSession string
	var tmuxWindow, tmuxPane int
	if tmux.InTmux() {
		if loc, err := tmux.CurrentLocation(); err == nil {
			tmuxSession = loc.Session
			tmuxWindow = loc.Window
			tmuxPane = loc.Pane
		}
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
		return b.runWithPTY(ctx, cmd, nil, opts, nil)
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
		TmuxSession:      tmuxSession,
		TmuxWindow:       tmuxWindow,
		TmuxPane:         tmuxPane,
		OutputFile:       outputFile,
		LoopInterval:     opts.LoopInterval,
		ParentID:         opts.ParentID,
	}

	if opts.ResumeSessionID != "" && opts.ResumeSessionID != "*" {
		session.ClaudeSessionID = opts.ResumeSessionID
	}

	if err := b.db.CreateSession(session); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	// Run with PTY capture
	var autoTerminated bool
	err = b.runWithPTY(ctx, cmd, session, opts, &autoTerminated)

	// When auto-terminate kills the process, cmd.Wait() returns a "signal: killed" error
	// which is expected and should be treated as successful completion
	if err != nil && !(opts.AutoTerminate && isKilledError(err)) {
		b.db.UpdateSessionStatus(sessionID, db.StatusAbandoned)
		return err
	}

	// If auto-terminate was enabled but the process exited on its own (e.g. user pressed Ctrl+C),
	// signal this as an interruption so the caller (e.g. play loop) can stop.
	if opts.AutoTerminate && !autoTerminated && opts.Interrupted != nil {
		*opts.Interrupted = true
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

// runWithPTY runs the command with a pseudo-terminal for interactive use.
// If autoTerminated is non-nil, it is set to true when the activity monitor killed the process.
func (b *Base) runWithPTY(ctx context.Context, cmd *exec.Cmd, session *db.Session, opts agent.RunOptions, autoTerminated *bool) error {
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
				if opts.CapturedSessionID != nil && *opts.CapturedSessionID == "" && session != nil {
					if m := claudeSessionIDRe.FindStringSubmatch(string(buf[:n])); len(m) > 1 {
						sid := m[1]
						*opts.CapturedSessionID = sid
						b.db.UpdateClaudeSessionID(session.ID, sid) //nolint:errcheck
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

	waitErr := cmd.Wait()

	if autoTerminated != nil && monitor != nil {
		monitor.mu.Lock()
		*autoTerminated = monitor.terminated
		monitor.mu.Unlock()
	}

	return waitErr
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
