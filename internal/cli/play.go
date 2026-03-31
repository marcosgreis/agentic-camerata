package cli

import (
	"context"
	"fmt"
	"os"
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

// phaseCapturePatterns maps phase types to their file capture regex.
var phaseCapturePatterns = map[string]*regexp.Regexp{
	"research": regexp.MustCompile(`(thoughts/shared/research/\S+\.md)`),
	"plan":     regexp.MustCompile(`(thoughts/shared/plans/\S+\.md)`),
	"review":   regexp.MustCompile(`(thoughts/shared/reviews/\S+\.md)`),
}

// PlayCmd runs a multi-phase playbook workflow
type PlayCmd struct {
	Playbook string `arg:"" help:"Path to playbook markdown file"`
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
	"look-and-fix": {agent.CommandLookAndFix, db.WorkflowFix},
	"review":       {agent.CommandReview, db.WorkflowReview},
}

// Run executes the play command
func (c *PlayCmd) Run(cli *CLI) (retErr error) {
	pb, err := playbook.Parse(c.Playbook)
	if err != nil {
		return err
	}

	database := cli.Database()
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

	// Create parent play session
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
		} else {
			database.UpdateSessionStatus(sessionID, db.StatusCompleted)
		}
	}()

	var researchFiles []string
	var planFile string
	taggedFiles := make(map[string][]string)
	total := len(pb.Phases)

	for i := 0; i < len(pb.Phases); i++ {
		phase := pb.Phases[i]
		total = len(pb.Phases)

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
					if planFile == "" {
						return fmt.Errorf("implement phase has no content and no plan file was captured from previous phases")
					}
					task = planFile
				} else if planFile != "" {
					filesToPass = []string{planFile}
				}
			}
		}

		allFiles := append(phase.Include, filesToPass...)
		if len(allFiles) > 0 && task != "" {
			task = PrependFilesToTask(allFiles, task)
		}

		var phaseCaptured []string
		var interrupted bool
		ag, err := getAgent(phase.Agent)
		if err != nil {
			return fmt.Errorf("phase %d (%s): %w", i+1, phase.Type, err)
		}
		err = ag.Run(context.Background(), agent.RunOptions{
			Command:         mapping.Command,
			WorkflowType:    mapping.Workflow,
			TaskDescription: task,
			Model:           cli.Model,
			AutoTerminate:   i < total-1,
			AutonomousMode:  cli.Autonomous,
			CapturedFiles:   &phaseCaptured,
			CapturePattern:  phaseCapturePatterns[phase.Type],
			ParentID:        sessionID,
			Interrupted:     &interrupted,
		})
		if err != nil {
			return fmt.Errorf("phase %d (%s): %w", i+1, phase.Type, err)
		}
		if interrupted {
			fmt.Printf("\n=== Playbook interrupted by user ===\n")
			return fmt.Errorf("interrupted")
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
	}

	fmt.Printf("\n=== Playbook complete (%d phases) ===\n", total)
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
