package playbook

import (
	"fmt"
	"os"
	"strings"
)

// Phase represents a single phase in a playbook
type Phase struct {
	Type    string   // "research", "plan", "implement", "new", "fix", "look-and-fix"
	Content string   // task description (body text under heading, metadata stripped)
	Tag     string   // optional tag for referencing this phase's outputs
	Uses    []string // optional tags of phases whose outputs to use
	Include []string // optional file paths to prepend to the phase prompt
}

// Playbook represents a parsed playbook file
type Playbook struct {
	Phases []Phase
}

// validPhaseTypes maps normalized heading text to phase type
var validPhaseTypes = map[string]string{
	"research":     "research",
	"plan":         "plan",
	"implement":    "implement",
	"new":          "new",
	"fix":          "fix",
	"look-and-fix": "look-and-fix",
	"review":       "review",
	"exit":         "exit",
}

// Parse reads a playbook markdown file and extracts phases
func Parse(path string) (*Playbook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read playbook: %w", err)
	}

	return ParseContent(string(data))
}

// ParseContent parses playbook content from a string
func ParseContent(content string) (*Playbook, error) {
	lines := strings.Split(content, "\n")

	var phases []Phase
	var currentType string
	var currentLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			// Save previous phase if any
			if currentType != "" {
				tag, uses, include, rest := extractMetadata(currentLines)
				phases = append(phases, Phase{
					Type:    currentType,
					Content: strings.TrimSpace(strings.Join(rest, "\n")),
					Tag:     tag,
					Uses:    uses,
					Include: include,
				})
			}

			// Parse heading
			heading := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			normalized := strings.ToLower(heading)

			phaseType, ok := validPhaseTypes[normalized]
			if !ok {
				return nil, fmt.Errorf("unknown phase type: %q (valid: research, plan, implement, new, fix, look-and-fix, review, exit)", heading)
			}

			currentType = phaseType
			currentLines = nil
			continue
		}

		if currentType != "" {
			currentLines = append(currentLines, line)
		}
	}

	// Save last phase
	if currentType != "" {
		tag, uses, include, rest := extractMetadata(currentLines)
		phases = append(phases, Phase{
			Type:    currentType,
			Content: strings.TrimSpace(strings.Join(rest, "\n")),
			Tag:     tag,
			Uses:    uses,
			Include: include,
		})
	}

	if len(phases) == 0 {
		return nil, fmt.Errorf("no phases found in playbook (use ## Research, ## Plan, ## Implement headings)")
	}

	// Validate unique tags
	seen := make(map[string]bool)
	for _, p := range phases {
		if p.Tag != "" {
			if seen[p.Tag] {
				return nil, fmt.Errorf("duplicate phase tag: %q", p.Tag)
			}
			seen[p.Tag] = true
		}
	}

	// Validate included files exist
	for i, p := range phases {
		for _, f := range p.Include {
			info, err := os.Stat(f)
			if err != nil {
				return nil, fmt.Errorf("phase %d include file not found: %s", i+1, f)
			}
			if info.IsDir() {
				return nil, fmt.Errorf("phase %d include path is a directory: %s", i+1, f)
			}
		}
	}

	return &Playbook{Phases: phases}, nil
}

// extractMetadata parses tag:, uses:, and include: lines from the top of phase body lines.
// Returns tag, uses, include, and remaining content lines with metadata stripped.
func extractMetadata(lines []string) (tag string, uses []string, include []string, rest []string) {
	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			i++
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "tag:") {
			tag = strings.TrimSpace(trimmed[4:])
			i++
			continue
		}
		if strings.HasPrefix(lower, "uses:") {
			raw := strings.TrimSpace(trimmed[5:])
			for _, u := range strings.Split(raw, ",") {
				u = strings.TrimSpace(u)
				if u != "" {
					uses = append(uses, u)
				}
			}
			i++
			continue
		}
		if strings.HasPrefix(lower, "include:") {
			raw := strings.TrimSpace(trimmed[8:])
			for _, f := range strings.Split(raw, ",") {
				f = strings.TrimSpace(f)
				if f != "" {
					include = append(include, f)
				}
			}
			i++
			continue
		}
		break
	}
	rest = lines[i:]
	return
}
