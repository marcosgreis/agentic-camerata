package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
	"github.com/agentic-camerata/cmt/internal/plans"
	"github.com/agentic-camerata/cmt/internal/playbook"
	"github.com/agentic-camerata/cmt/internal/tmux"
)

// PlayState holds the state persisted between phases so a play session can be resumed.
type PlayState struct {
	NextPhase       int                 `json:"next_phase"`
	Phases          []playbook.Phase    `json:"phases"`
	ResearchFiles   []string            `json:"research_files"`
	PlanFile        string              `json:"plan_file"`
	TaggedFiles     map[string][]string `json:"tagged_files"`
	PhaseSessionIDs map[int]string      `json:"phase_session_ids"` // phase index → Claude session ID
}

// phaseCapturePatterns maps phase types to their file capture regex.
var phaseCapturePatterns = map[string]*regexp.Regexp{
	"research": regexp.MustCompile(`(thoughts/shared/research/\S+\.md)`),
	"plan":     regexp.MustCompile(`(thoughts/shared/plans/\S+\.md)`),
	"review":   regexp.MustCompile(`(thoughts/shared/reviews/\S+\.md)`),
}

// PlayCmd runs a multi-phase playbook workflow
type PlayCmd struct {
	Playbook string `arg:"" optional:"" help:"Path to playbook markdown file"`
	Resume   string `name:"resume" short:"r" optional:"*" help:"Resume an abandoned play session; use --resume for interactive selection or --resume SESSION_ID for a specific session"`
}

// phaseMapping maps playbook phase types to command/workflow types
var phaseMapping = map[string]struct {
	Command  agent.CommandType
	Workflow db.WorkflowType
}{
	"research":     {agent.CommandResearch, db.WorkflowResearch},
	"plan":         {agent.CommandPlan, db.WorkflowPlan},
	"implement":    {agent.CommandImplement, db.WorkflowImplement},
	"new":          {agent.CommandNew, db.WorkflowGeneral},
	"fix":          {agent.CommandFixTest, db.WorkflowFix},
	"fix-local-comments": {agent.CommandFixLocalComments, db.WorkflowFix},
	"review":       {agent.CommandReview, db.WorkflowReview},
}

// Run executes the play command
func (c *PlayCmd) Run(cli *CLI) (retErr error) {
	database := cli.Database()

	if c.Resume != "" {
		return c.doResume(cli, database)
	}

	if c.Playbook == "" {
		return fmt.Errorf("either a playbook file or --resume is required")
	}

	pb, err := playbook.Parse(c.Playbook)
	if err != nil {
		return err
	}

	playbookPath, err := filepath.Abs(c.Playbook)
	if err != nil {
		return fmt.Errorf("resolve playbook path: %w", err)
	}

	sessionID := uuid.New().String()[:8]
	var tmuxSession string
	var tmuxWindow, tmuxPane int
	if tmux.InTmux() {
		if loc, err := tmux.CurrentLocation(); err == nil {
			tmuxSession = loc.Session
			tmuxWindow = loc.Window
			tmuxPane = loc.Pane
		}
	}
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	outputDir := filepath.Join(homeDir, ".config", "cmt", "output")
	os.MkdirAll(outputDir, 0755) //nolint:errcheck

	// Copy playbook to config dir for future reference
	playbooksDir := os.Getenv("CMT_PLAYBOOKS_DIR")
	if playbooksDir == "" {
		playbooksDir = filepath.Join(homeDir, ".agentic-camerata", "playbooks")
	}
	os.MkdirAll(playbooksDir, 0755) //nolint:errcheck

	savedName := sessionID + "-" + filepath.Base(playbookPath)
	savedPath := filepath.Join(playbooksDir, savedName)
	pbData, err := os.ReadFile(playbookPath)
	if err != nil {
		return fmt.Errorf("read playbook for copy: %w", err)
	}
	if err := os.WriteFile(savedPath, pbData, 0644); err != nil {
		return fmt.Errorf("save playbook copy: %w", err)
	}

	session := &db.Session{
		ID:               sessionID,
		WorkflowType:     db.WorkflowPlay,
		Status:           db.StatusWorking,
		WorkingDirectory: workDir,
		TaskDescription:  playbookPath,
		Prefix:           os.Getenv("CMT_PREFIX"),
		TmuxSession:      tmuxSession,
		TmuxWindow:       tmuxWindow,
		TmuxPane:         tmuxPane,
		OutputFile:       filepath.Join(outputDir, sessionID+".log"),
		PlaybookFile:     savedPath,
		PID:              os.Getpid(),
	}
	if err := database.CreateSession(session); err != nil {
		return fmt.Errorf("create play session: %w", err)
	}

	defer func() {
		if retErr != nil {
			database.UpdateSessionStatus(sessionID, db.StatusAbandoned)
			retErr = fmt.Errorf("%w\n\nto resume this session run: cmt play --resume %s", retErr, sessionID)
		} else {
			database.UpdateSessionStatus(sessionID, db.StatusCompleted)
		}
	}()

	return runPlaybook(cli, database, sessionID, pb, 0, PlayState{})
}

// doResume resumes an abandoned play session.
func (c *PlayCmd) doResume(cli *CLI, database *db.DB) (retErr error) {
	var session *db.Session
	var err error

	if c.Resume == "*" {
		// Interactive selection
		session, err = selectAbandonedPlaySession(database)
		if err != nil {
			return err
		}
	} else {
		// Resume by specific session ID
		session, err = database.GetSession(c.Resume)
		if err != nil {
			return fmt.Errorf("get session: %w", err)
		}
		if session == nil {
			return fmt.Errorf("session %s not found", c.Resume)
		}
		if session.WorkflowType != db.WorkflowPlay {
			return fmt.Errorf("session %s is not a play session", c.Resume)
		}
		if session.Status != db.StatusAbandoned {
			return fmt.Errorf("session %s is not abandoned (status: %s)", c.Resume, session.Status)
		}
	}

	// Parse play state
	var state PlayState
	if session.PlayState != "" {
		if err := json.Unmarshal([]byte(session.PlayState), &state); err != nil {
			return fmt.Errorf("parse play state: %w", err)
		}
	}

	// Build the playbook: use pre-expanded phases from state, or re-parse from file
	var pb *playbook.Playbook
	if len(state.Phases) > 0 {
		pb = &playbook.Playbook{Phases: state.Phases}
	} else {
		if session.PlaybookFile == "" {
			return fmt.Errorf("session %s has no playbook file", session.ID)
		}
		pb, err = playbook.Parse(session.PlaybookFile)
		if err != nil {
			return fmt.Errorf("parse playbook: %w", err)
		}
	}

	fmt.Printf("Resuming play session %s from phase %d/%d\n", session.ID, state.NextPhase+1, len(pb.Phases))

	// Re-activate the session
	if err := database.UpdateSessionStatus(session.ID, db.StatusWorking); err != nil {
		return fmt.Errorf("update session status: %w", err)
	}
	database.UpdateSessionPID(session.ID, os.Getpid()) //nolint:errcheck

	defer func() {
		if retErr != nil {
			database.UpdateSessionStatus(session.ID, db.StatusAbandoned)
			retErr = fmt.Errorf("%w\n\nto resume this session run: cmt play --resume %s", retErr, session.ID)
		} else {
			database.UpdateSessionStatus(session.ID, db.StatusCompleted)
		}
	}()

	return runPlaybook(cli, database, session.ID, pb, state.NextPhase, state)
}

// selectAbandonedPlaySession finds an abandoned play session interactively.
func selectAbandonedPlaySession(database *db.DB) (*db.Session, error) {
	sessions, err := database.ListAbandonedPlaySessions()
	if err != nil {
		return nil, err
	}
	switch len(sessions) {
	case 0:
		return nil, fmt.Errorf("no abandoned play sessions found")
	case 1:
		fmt.Printf("Resuming session %s (%s)\n", sessions[0].ID, sessions[0].PlaybookFile)
		return sessions[0], nil
	default:
		lines := make([]string, len(sessions))
		for i, s := range sessions {
			lines[i] = fmt.Sprintf("%s  %s  %s", s.ID, s.CreatedAt.Format("2006-01-02 15:04"), s.PlaybookFile)
		}
		selected, err := fzfSelect(lines)
		if err != nil {
			return nil, err
		}
		id := strings.Fields(selected)[0]
		return database.GetSession(id)
	}
}

// fzfSelect presents options to fzf and returns the selected line.
func fzfSelect(options []string) (string, error) {
	cmd := exec.Command("fzf")
	cmd.Stdin = strings.NewReader(strings.Join(options, "\n"))
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("fzf selection cancelled")
	}
	return strings.TrimSpace(string(out)), nil
}

// savePlayState persists play state to the database.
func savePlayState(database *db.DB, sessionID string, nextPhase int, phases []playbook.Phase, researchFiles []string, planFile string, taggedFiles map[string][]string, phaseSessionIDs map[int]string) {
	state := PlayState{
		NextPhase:       nextPhase,
		Phases:          phases,
		ResearchFiles:   researchFiles,
		PlanFile:        planFile,
		TaggedFiles:     taggedFiles,
		PhaseSessionIDs: phaseSessionIDs,
	}
	if data, err := json.Marshal(state); err == nil {
		database.UpdatePlayState(sessionID, string(data)) //nolint:errcheck
	}
}

// findLastPhaseOfType returns the index of the last phase with the given type
// before currentIndex, or currentIndex if none found.
func findLastPhaseOfType(phases []playbook.Phase, currentIndex int, phaseType string) int {
	for j := currentIndex - 1; j >= 0; j-- {
		if phases[j].Type == phaseType {
			return j
		}
	}
	return currentIndex
}

// findLastPhaseWithTag returns the index of the last phase with the given tag
// before currentIndex, or currentIndex if none found.
func findLastPhaseWithTag(phases []playbook.Phase, currentIndex int, tag string) int {
	for j := currentIndex - 1; j >= 0; j-- {
		if phases[j].Tag == tag {
			return j
		}
	}
	return currentIndex
}

// runPlaybook runs the phases of a playbook starting from startPhase,
// restoring context from state for resumed sessions.
func runPlaybook(cli *CLI, database *db.DB, sessionID string, pb *playbook.Playbook, startPhase int, state PlayState) error {
	agents := make(map[string]agent.Agent)
	getAgent := func(agentType string) (agent.Agent, error) {
		if agentType == "" {
			agentType = cli.Agent
		}
		if ag, ok := agents[agentType]; ok {
			return ag, nil
		}
		ag, err := newAgent(agentType, database)
		if err != nil {
			return nil, err
		}
		agents[agentType] = ag
		return ag, nil
	}

	researchFiles := state.ResearchFiles
	planFile := state.PlanFile
	taggedFiles := state.TaggedFiles
	if taggedFiles == nil {
		taggedFiles = make(map[string][]string)
	}
	phaseSessionIDs := state.PhaseSessionIDs
	if phaseSessionIDs == nil {
		phaseSessionIDs = make(map[int]string)
	}

	for i := startPhase; i < len(pb.Phases); i++ {
		phase := pb.Phases[i]
		total := len(pb.Phases)

		if phase.Type == "exit" {
			fmt.Printf("\n=== Phase %d/%d: exit ===\n", i+1, total)
			break
		}

		if phase.Type == "play" {
			fmt.Printf("\n=== Phase %d/%d: play %s ===\n", i+1, total, phase.Content)
			nestedPB, err := playbook.Parse(phase.Content)
			if err != nil {
				return fmt.Errorf("phase %d (play): %w", i+1, err)
			}
			expanded := make([]playbook.Phase, 0, len(pb.Phases)-1+len(nestedPB.Phases))
			expanded = append(expanded, pb.Phases[:i]...)
			expanded = append(expanded, nestedPB.Phases...)
			expanded = append(expanded, pb.Phases[i+1:]...)
			pb.Phases = expanded
			i--
			continue
		}

		mapping, ok := phaseMapping[phase.Type]
		if !ok {
			return fmt.Errorf("unknown phase type: %s", phase.Type)
		}

		fmt.Printf("\n=== Phase %d/%d: %s ===\n", i+1, total, phase.Type)

		task := phase.Content

		switch phase.Pick {
		case "true":
			picked, err := plans.SelectPlanFile()
			if err != nil {
				return fmt.Errorf("phase %d (implement pick): %w", i+1, err)
			}
			task = picked
		case "last":
			picked, err := plans.LatestPlanFile()
			if err != nil {
				return fmt.Errorf("phase %d (implement pick:last): %w", i+1, err)
			}
			fmt.Printf("--- Selected latest plan: %s\n", picked)
			task = picked
		}

		var filesToPass []string
		if len(phase.Uses) > 0 {
			for _, ref := range phase.Uses {
				filesToPass = append(filesToPass, taggedFiles[ref]...)
			}
			if phase.Type == "implement" && task == "" {
				if pf := lastPlanFile(filesToPass); pf != "" {
					task = pf
					filesToPass = nil
				} else if len(filesToPass) > 0 {
					task = filesToPass[len(filesToPass)-1]
					filesToPass = filesToPass[:len(filesToPass)-1]
				} else {
					return fmt.Errorf("implement phase references tags with no captured files")
				}
			}
		} else if phase.Pick == "" {
			switch phase.Type {
			case "plan":
				filesToPass = researchFiles
			case "implement":
				if task == "" {
					if planFile != "" {
						task = planFile
					}
					// If planFile is still empty, the rollback check below will handle it
				} else if planFile != "" {
					filesToPass = []string{planFile}
				}
			}
		}

		// Automatic rollback: detect missing prerequisite outputs before running
		rollbackTo := -1
		if phase.Type == "implement" && phase.Pick == "" && len(phase.Uses) == 0 && task == "" && planFile == "" {
			rollbackTo = findLastPhaseOfType(pb.Phases, i, "plan")
		}
		for _, ref := range phase.Uses {
			if len(taggedFiles[ref]) == 0 {
				rollbackTo = findLastPhaseWithTag(pb.Phases, i, ref)
				break
			}
		}
		if rollbackTo >= 0 {
			savePlayState(database, sessionID, rollbackTo, pb.Phases, researchFiles, planFile, taggedFiles, phaseSessionIDs)
			return fmt.Errorf("phase %d (%s): missing prerequisite output; rolling back to phase %d (%s)",
				i+1, phase.Type, rollbackTo+1, pb.Phases[rollbackTo].Type)
		}

		allFiles := append(phase.Include, filesToPass...)
		if len(allFiles) > 0 && task != "" {
			task = PrependFilesToTask(allFiles, task)
		}

		// Save state before running so resume starts from this phase if the agent fails
		savePlayState(database, sessionID, i, pb.Phases, researchFiles, planFile, taggedFiles, phaseSessionIDs)

		var phaseCaptured []string
		var interrupted bool
		var capturedSessionID string

		// If this phase was previously run (e.g. rolled back to), resume the Claude session
		previousClaudeSessionID := phaseSessionIDs[i]

		ag, err := getAgent(phase.Agent)
		if err != nil {
			return fmt.Errorf("phase %d (%s): %w", i+1, phase.Type, err)
		}
		err = ag.Run(context.Background(), agent.RunOptions{
			Command:           mapping.Command,
			WorkflowType:      mapping.Workflow,
			TaskDescription:   task,
			Model:             cli.Model,
			AutoTerminate:     i < total-1,
			AutonomousMode:    cli.Autonomous,
			CapturedFiles:     &phaseCaptured,
			CapturePattern:    phaseCapturePatterns[phase.Type],
			CapturedSessionID: &capturedSessionID,
			ResumeSessionID:   previousClaudeSessionID,
			ParentID:          sessionID,
			Interrupted:       &interrupted,
		})
		if err != nil {
			return fmt.Errorf("phase %d (%s): %w", i+1, phase.Type, err)
		}
		if interrupted {
			fmt.Printf("\n=== Playbook interrupted by user ===\n")
			return fmt.Errorf("interrupted")
		}

		// Store the Claude session ID for this phase (for future rollback/resume)
		if capturedSessionID != "" {
			phaseSessionIDs[i] = capturedSessionID
		}

		validated := existingFiles(phaseCaptured)
		switch phase.Type {
		case "research":
			researchFiles = append(researchFiles, validated...)
		case "plan":
			if pf := lastPlanFile(validated); pf != "" {
				planFile = pf
			}
		}

		if phase.Tag != "" {
			taggedFiles[phase.Tag] = append(taggedFiles[phase.Tag], validated...)
		}

		if len(phaseCaptured) > 0 {
			fmt.Printf("--- Captured files: %s\n", strings.Join(phaseCaptured, ", "))
		}

		// Save state after successful phase so resume skips it next time
		savePlayState(database, sessionID, i+1, pb.Phases, researchFiles, planFile, taggedFiles, phaseSessionIDs)
	}

	fmt.Printf("\n=== Playbook complete (%d phases) ===\n", len(pb.Phases))
	return nil
}

// lastPlanFile returns the last captured file matching thoughts/shared/plans/*.md
func lastPlanFile(files []string) string {
	for i := len(files) - 1; i >= 0; i-- {
		if strings.Contains(files[i], "thoughts/shared/plans/") {
			return files[i]
		}
	}
	return ""
}

// existingFiles returns only the paths that exist as regular files.
func existingFiles(paths []string) []string {
	var valid []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err == nil && !info.IsDir() {
			valid = append(valid, p)
		}
	}
	return valid
}
