package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/agentic-camerata/cmt/internal/claude"
	"github.com/agentic-camerata/cmt/internal/db"
	"github.com/agentic-camerata/cmt/internal/plans"
	"github.com/agentic-camerata/cmt/internal/playbook"
	"github.com/agentic-camerata/cmt/internal/tmux"
)

// phaseCapturePatterns maps phase types to their file capture regex.
// Only files matching the pattern are captured for that phase type.
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
	Command  claude.CommandType
	Workflow db.WorkflowType
}{
	"research":     {claude.CommandResearch, db.WorkflowResearch},
	"plan":         {claude.CommandPlan, db.WorkflowPlan},
	"implement":    {claude.CommandImplement, db.WorkflowImplement},
	"new":          {claude.CommandNew, db.WorkflowGeneral},
	"fix":          {claude.CommandFixTest, db.WorkflowFix},
	"look-and-fix": {claude.CommandLookAndFix, db.WorkflowFix},
	"review":       {claude.CommandReview, db.WorkflowReview},
}

// phaseModels maps phase types to their default model.
var phaseModels = map[string]string{
	"research":     "opus",
	"plan":         "opus",
	"implement":    "sonnet",
	"new":          "opus",
	"fix":          "opus",
	"look-and-fix": "opus",
	"review":       "opus",
}

// Run executes the play command
func (c *PlayCmd) Run(cli *CLI) (retErr error) {
	pb, err := playbook.Parse(c.Playbook)
	if err != nil {
		return err
	}

	database := cli.Database()
	runner, err := claude.NewRunner(database)
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

	// Scoped file tracking: each phase type only sees relevant files
	var researchFiles []string              // files accumulated across all research phases (default behavior)
	var planFile string                     // plan file from the most recent plan phase (default behavior)
	taggedFiles := make(map[string][]string) // tag → captured files (for explicit references)
	total := len(pb.Phases)

	for i := 0; i < len(pb.Phases); i++ {
		phase := pb.Phases[i]
		total = len(pb.Phases)

		if phase.Type == "exit" {
			fmt.Printf("\n=== Phase %d/%d: exit ===\n", i+1, total)
			break
		}

		// Handle nested play phases by inlining the referenced playbook's phases
		if phase.Type == "play" {
			fmt.Printf("\n=== Phase %d/%d: play %s ===\n", i+1, total, phase.Content)
			nestedPB, err := playbook.Parse(phase.Content)
			if err != nil {
				return fmt.Errorf("phase %d (play): %w", i+1, err)
			}
			// Replace the play phase with the nested playbook's phases
			expanded := make([]playbook.Phase, 0, len(pb.Phases)-1+len(nestedPB.Phases))
			expanded = append(expanded, pb.Phases[:i]...)
			expanded = append(expanded, nestedPB.Phases...)
			expanded = append(expanded, pb.Phases[i+1:]...)
			pb.Phases = expanded
			// Re-process current index (now points to first nested phase)
			i--
			continue
		}

		mapping, ok := phaseMapping[phase.Type]
		if !ok {
			return fmt.Errorf("unknown phase type: %s", phase.Type)
		}

		fmt.Printf("\n=== Phase %d/%d: %s ===\n", i+1, total, phase.Type)

		// Build task description
		task := phase.Content

		// Handle pick: select plan file for implement phases
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

		// Determine which files to pass based on phase type
		var filesToPass []string
		if len(phase.Uses) > 0 {
			// Explicit references: only pass files from referenced tags
			for _, ref := range phase.Uses {
				filesToPass = append(filesToPass, taggedFiles[ref]...)
			}
			// For implement with no content, use last referenced file as task
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
			// Default behavior (skip if pick already resolved the task)
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

		// Prepend include files and scoped files to task
		allFiles := append(phase.Include, filesToPass...)
		if len(allFiles) > 0 && task != "" {
			task = PrependFilesToTask(allFiles, task)
		}

		var phaseCaptured []string
		err := runner.Run(context.Background(), claude.RunOptions{
			Command:         mapping.Command,
			WorkflowType:    mapping.Workflow,
			TaskDescription: task,
			Model:           cli.ResolveModel(phaseModels[phase.Type]),
			AutoTerminate:   i < total-1,
			AutonomousMode:  cli.Autonomous,
			CapturedFiles:   &phaseCaptured,
			CapturePattern:  phaseCapturePatterns[phase.Type],
			ParentID:        sessionID,
		})
		if err != nil {
			return fmt.Errorf("phase %d (%s): %w", i+1, phase.Type, err)
		}

		// Route captured files to the appropriate scope
		validated := existingFiles(phaseCaptured)
		switch phase.Type {
		case "research":
			researchFiles = append(researchFiles, validated...)
		case "plan":
			if pf := lastPlanFile(validated); pf != "" {
				planFile = pf
			}
		}

		// Also store under tag if present
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
