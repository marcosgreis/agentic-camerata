package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/agentic-camerata/cmt/internal/agent"
	"github.com/agentic-camerata/cmt/internal/db"
	"github.com/agentic-camerata/cmt/internal/plans"
	"github.com/agentic-camerata/cmt/internal/playbook"
	"github.com/agentic-camerata/cmt/internal/runner"
	"github.com/agentic-camerata/cmt/internal/tmux"
)

// paneStarter is an optional interface for agents that support background pane execution.
type paneStarter interface {
	StartInPane(ctx context.Context, opts agent.RunOptions) (*runner.PaneSession, error)
}

// phaseCapturePatterns maps phase types to their file capture regex.
var phaseCapturePatterns = map[string]*regexp.Regexp{
	"research": regexp.MustCompile(`(thoughts/shared/research/\S+\.md)`),
	"plan":     regexp.MustCompile(`(thoughts/shared/plans/\S+\.md)`),
	"review":   regexp.MustCompile(`(thoughts/shared/reviews/\S+\.md)`),
}

// PlayCmd runs a multi-phase playbook workflow
type PlayCmd struct {
	Playbook string `arg:"" help:"Path to playbook markdown file"`
	Debug    bool   `help:"Keep background panes alive after completion for debugging"`
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
	ag, err := newAgent(cli.Agent, database)
	if err != nil {
		return err
	}

	// Create parent play session
	playbookPath, err := filepath.Abs(c.Playbook)
	if err != nil {
		return fmt.Errorf("resolve playbook path: %w", err)
	}

	sessionID := uuid.New().String()[:8]
	loc, err := tmux.CurrentLocation()
	if err != nil {
		return fmt.Errorf("get tmux location: %w", err)
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

	session := &db.Session{
		ID:               sessionID,
		WorkflowType:     db.WorkflowPlay,
		Status:           db.StatusWorking,
		WorkingDirectory: workDir,
		TaskDescription:  playbookPath,
		Prefix:           os.Getenv("CMT_PREFIX"),
		TmuxSession:      loc.Session,
		TmuxWindow:       loc.Window,
		TmuxPane:         loc.Pane,
		OutputFile:       filepath.Join(outputDir, sessionID+".log"),
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

		// Check for a group of consecutive independent research phases to run in parallel.
		if phase.Type == "research" {
			j := i + 1
			for j < len(pb.Phases) && pb.Phases[j].Type == "research" {
				j++
			}
			group := pb.Phases[i:j]
			if len(group) > 1 && isParallelizable(group) {
				if _, ok := ag.(paneStarter); ok {
					fmt.Printf("\n=== Phases %d-%d/%d: %d parallel research phases ===\n", i+1, j, total, len(group))
					capturedByPhase, err := runParallelResearch(context.Background(), ag, group, cli, taggedFiles, sessionID, c.Debug)
					if err != nil {
						return fmt.Errorf("parallel research phases %d-%d: %w", i+1, j, err)
					}
					for k, p := range group {
						validated := existingFiles(capturedByPhase[k])
						researchFiles = append(researchFiles, validated...)
						if p.Tag != "" {
							taggedFiles[p.Tag] = append(taggedFiles[p.Tag], validated...)
						}
						if len(capturedByPhase[k]) > 0 {
							fmt.Printf("--- Phase %d captured: %s\n", i+k+1, strings.Join(capturedByPhase[k], ", "))
						}
					}
					i = j - 1 // loop will increment to j
					continue
				}
			}
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
		err := ag.Run(context.Background(), agent.RunOptions{
			Command:         mapping.Command,
			WorkflowType:    mapping.Workflow,
			TaskDescription: task,
			Model:           cli.Model,
			AutoTerminate:   i < total-1,
			AutonomousMode:  cli.Autonomous,
			CapturedFiles:   &phaseCaptured,
			CapturePattern:  phaseCapturePatterns[phase.Type],
			ParentID:        sessionID,
		})
		if err != nil {
			return fmt.Errorf("phase %d (%s): %w", i+1, phase.Type, err)
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

// isParallelizable returns true if the given research phases can all run concurrently.
// Phases are parallelizable when no phase uses a tag produced by another phase in the group.
func isParallelizable(phases []playbook.Phase) bool {
	if len(phases) <= 1 {
		return false
	}
	groupTags := make(map[string]bool)
	for _, p := range phases {
		if p.Tag != "" {
			groupTags[p.Tag] = true
		}
	}
	for _, p := range phases {
		for _, u := range p.Uses {
			if groupTags[u] {
				return false
			}
		}
	}
	return true
}

// runParallelResearch runs multiple research phases concurrently, each in its own tmux pane.
// ag must implement paneStarter. Returns a slice of captured file paths (one entry per phase, in order).
func runParallelResearch(ctx context.Context, ag agent.Agent, phases []playbook.Phase, cli *CLI, taggedFiles map[string][]string, parentID string, debug bool) ([][]string, error) {
	ps := ag.(paneStarter) // caller ensures ag implements paneStarter

	type result struct {
		files []string
		err   error
	}

	results := make([]result, len(phases))
	captureRe := phaseCapturePatterns["research"]

	var wg sync.WaitGroup
	for idx, phase := range phases {
		wg.Add(1)
		go func(idx int, phase playbook.Phase) {
			defer wg.Done()

			task := phase.Content
			var filesToPass []string
			for _, ref := range phase.Uses {
				filesToPass = append(filesToPass, taggedFiles[ref]...)
			}
			allFiles := append(phase.Include, filesToPass...)
			if len(allFiles) > 0 && task != "" {
				task = PrependFilesToTask(allFiles, task)
			}

			pane, err := ps.StartInPane(ctx, agent.RunOptions{
				Command:         agent.CommandResearch,
				WorkflowType:    db.WorkflowResearch,
				TaskDescription: task,
				Model:           cli.Model,
				AutonomousMode:  cli.Autonomous,
				ParentID:        parentID,
			})
			if err != nil {
				results[idx].err = err
				return
			}

			files, err := pane.Wait(ctx, captureRe, runner.WaitOptions{KeepPane: debug})
			results[idx].files = files
			results[idx].err = err
		}(idx, phase)
	}
	wg.Wait()

	capturedAll := make([][]string, len(phases))
	for i, r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("research phase %d: %w", i+1, r.err)
		}
		capturedAll[i] = r.files
	}
	return capturedAll, nil
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
