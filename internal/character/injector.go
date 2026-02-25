package character

import (
	"os"
	"strings"

	"github.com/Manjussha/clawdaemon/internal/tokenizer"
)

// Injector builds the full context string for a task using the defined injection order.
type Injector struct {
	loader *Loader
}

// NewInjector creates an Injector.
func NewInjector(loader *Loader) *Injector {
	return &Injector{loader: loader}
}

// BuildContext assembles context in the canonical injection order from CLAUDE.md:
//  1. IDENTITY.md (compressed at YELLOW+)
//  2. THINKING.md (compressed at YELLOW+)
//  3. RULES.md (never compressed)
//  4. MEMORY.md (relevant facts only)
//  5. Skill file
//  6. Project CLAUDE.md
//  7. Project memory.md
//  8. Checkpoint output (if resuming)
//  9. Task prompt (never compressed)
//
// Sections are joined with "\n\n---\n\n".
func (inj *Injector) BuildContext(opts BuildOpts) string {
	zone := opts.Zone
	var parts []string

	add := func(label, content string) {
		if content == "" {
			return
		}
		parts = append(parts, "# "+label+"\n\n"+content)
	}

	// 1. IDENTITY (compressed at YELLOW+)
	identity := inj.loader.LoadIdentity()
	if zone >= tokenizer.ZoneYellow {
		identity = tokenizer.OptimizeContext(identity, zone)
	}
	add("IDENTITY", identity)

	// 2. THINKING (compressed at YELLOW+)
	thinking := inj.loader.LoadThinking()
	if zone >= tokenizer.ZoneYellow {
		thinking = tokenizer.OptimizeContext(thinking, zone)
	}
	add("THINKING", thinking)

	// 3. RULES (never compressed)
	add("RULES", inj.loader.LoadRules())

	// 4. MEMORY (skip at ORANGE+)
	if zone < tokenizer.ZoneOrange {
		memory := inj.loader.LoadMemory()
		if memory != "" {
			add("MEMORY", tokenizer.OptimizeContext(memory, zone))
		}
	}

	// 5. Skill file
	skillName := opts.SkillName
	if skillName == "" && opts.Prompt != "" {
		skillName = inj.loader.AutoSelectSkill(opts.Prompt)
	}
	if skillName != "" {
		add("SKILL", inj.loader.LoadSkill(skillName))
	}

	// 6. Project CLAUDE.md
	if opts.ProjectCLAUDEMD != "" && zone < tokenizer.ZoneRed {
		add("PROJECT INSTRUCTIONS", opts.ProjectCLAUDEMD)
	}

	// 7. Project memory.md
	if opts.ProjectMemory != "" && zone < tokenizer.ZoneOrange {
		add("PROJECT MEMORY", opts.ProjectMemory)
	}

	// 8. Checkpoint
	if opts.Checkpoint != "" {
		add("CHECKPOINT (resuming from)", opts.Checkpoint)
	}

	// 9. Task prompt (never compressed)
	if opts.Prompt != "" {
		parts = append(parts, "# TASK\n\n"+opts.Prompt)
	}

	return strings.Join(parts, "\n\n---\n\n")
}

// BuildOpts holds all inputs for BuildContext.
type BuildOpts struct {
	Zone            tokenizer.BudgetZone
	SkillName       string
	ProjectCLAUDEMD string
	ProjectMemory   string
	Checkpoint      string
	Prompt          string
}

// ReadProjectFile reads a file path, returning empty string on error.
func ReadProjectFile(path string) string {
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}
