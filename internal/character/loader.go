// Package character handles loading and injecting character context files into prompts.
package character

import (
	"os"
	"path/filepath"
	"strings"
)

// Loader reads character definition files from the character directory.
type Loader struct {
	characterDir string
}

// NewLoader creates a Loader pointing to the given directory.
func NewLoader(characterDir string) *Loader {
	return &Loader{characterDir: characterDir}
}

// LoadIdentity reads IDENTITY.md.
func (l *Loader) LoadIdentity() string {
	return l.readFile("IDENTITY.md")
}

// LoadThinking reads THINKING.md.
func (l *Loader) LoadThinking() string {
	return l.readFile("THINKING.md")
}

// LoadRules reads RULES.md. Never compressed.
func (l *Loader) LoadRules() string {
	return l.readFile("RULES.md")
}

// LoadMemory reads MEMORY.md.
func (l *Loader) LoadMemory() string {
	return l.readFile("MEMORY.md")
}

// LoadSkill reads a skill file by name from skills/ subdirectory.
func (l *Loader) LoadSkill(name string) string {
	if name == "" {
		return ""
	}
	// Sanitize: only allow alphanumeric, dash, underscore.
	safe := sanitizeName(name)
	if safe == "" {
		return ""
	}
	return l.readFile(filepath.Join("skills", safe+".md"))
}

// AutoSelectSkill picks the best skill file based on keywords in the prompt.
func (l *Loader) AutoSelectSkill(prompt string) string {
	skills := l.ListSkills()
	lower := strings.ToLower(prompt)

	keywordMap := map[string][]string{
		"laravel-developer":  {"laravel", "php", "eloquent", "artisan", "blade"},
		"flutter-developer":  {"flutter", "dart", "widget", "pub.dev"},
		"wordpress":          {"wordpress", "wp", "plugin", "shortcode", "woocommerce"},
		"bug-fixer":          {"bug", "fix", "error", "crash", "exception", "debug"},
		"code-reviewer":      {"review", "code review", "pull request", "pr", "feedback"},
		"seo-writer":         {"seo", "meta", "keyword", "content", "article", "blog"},
		"devops":             {"docker", "kubernetes", "ci/cd", "nginx", "deploy", "pipeline"},
		"playwright-tester":  {"playwright", "test", "e2e", "automation", "selenium", "puppeteer"},
	}

	for _, skillFile := range skills {
		name := strings.TrimSuffix(filepath.Base(skillFile), ".md")
		if keywords, ok := keywordMap[name]; ok {
			for _, kw := range keywords {
				if strings.Contains(lower, kw) {
					return name
				}
			}
		}
	}
	return ""
}

// ListSkills returns all skill filenames in the skills/ subdirectory.
func (l *Loader) ListSkills() []string {
	skillsDir := filepath.Join(l.characterDir, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil
	}
	var skills []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			skills = append(skills, e.Name())
		}
	}
	return skills
}

func (l *Loader) readFile(name string) string {
	path := filepath.Join(l.characterDir, name)
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func sanitizeName(name string) string {
	var out strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' {
			out.WriteRune(r)
		}
	}
	return out.String()
}

