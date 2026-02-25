package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// GetCharacterFile handles GET /api/v1/character/{file}.
func (h *Handler) GetCharacterFile(w http.ResponseWriter, r *http.Request) {
	name := sanitizeCharFileName(pathID(r, "file"))
	if name == "" {
		fail(w, http.StatusBadRequest, "invalid file name")
		return
	}
	var content string
	switch name {
	case "IDENTITY":
		content = h.loader.LoadIdentity()
	case "THINKING":
		content = h.loader.LoadThinking()
	case "RULES":
		content = h.loader.LoadRules()
	case "MEMORY":
		content = h.loader.LoadMemory()
	default:
		fail(w, http.StatusNotFound, "unknown character file")
		return
	}
	ok(w, map[string]string{"file": name, "content": content})
}

// UpdateCharacterFile handles PUT /api/v1/character/{file}.
func (h *Handler) UpdateCharacterFile(w http.ResponseWriter, r *http.Request) {
	name := sanitizeCharFileName(pathID(r, "file"))
	if name == "" {
		fail(w, http.StatusBadRequest, "invalid file name")
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	path := filepath.Join(h.config.CharacterDir, name+".md")
	if err := os.WriteFile(path, []byte(req.Content), 0644); err != nil {
		fail(w, http.StatusInternalServerError, "write: "+err.Error())
		return
	}
	ok(w, map[string]string{"message": "updated"})
}

// ListSkills handles GET /api/v1/character/skills.
func (h *Handler) ListSkills(w http.ResponseWriter, r *http.Request) {
	skills := h.loader.ListSkills()
	ok(w, map[string]interface{}{"skills": skills})
}

// CreateSkill handles POST /api/v1/character/skills.
func (h *Handler) CreateSkill(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" {
		fail(w, http.StatusBadRequest, "name is required")
		return
	}
	safe := sanitizeSkillName(req.Name)
	if safe == "" {
		fail(w, http.StatusBadRequest, "invalid skill name")
		return
	}
	path := filepath.Join(h.config.CharacterDir, "skills", safe+".md")
	if err := os.WriteFile(path, []byte(req.Content), 0644); err != nil {
		fail(w, http.StatusInternalServerError, "write: "+err.Error())
		return
	}
	ok(w, map[string]string{"name": safe})
}

// UpdateSkill handles PUT /api/v1/character/skills/{name}.
func (h *Handler) UpdateSkill(w http.ResponseWriter, r *http.Request) {
	name := sanitizeSkillName(pathID(r, "name"))
	if name == "" {
		fail(w, http.StatusBadRequest, "invalid skill name")
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	path := filepath.Join(h.config.CharacterDir, "skills", name+".md")
	if err := os.WriteFile(path, []byte(req.Content), 0644); err != nil {
		fail(w, http.StatusInternalServerError, "write: "+err.Error())
		return
	}
	ok(w, map[string]string{"message": "updated"})
}

// DeleteSkill handles DELETE /api/v1/character/skills/{name}.
func (h *Handler) DeleteSkill(w http.ResponseWriter, r *http.Request) {
	name := sanitizeSkillName(pathID(r, "name"))
	if name == "" {
		fail(w, http.StatusBadRequest, "invalid skill name")
		return
	}
	path := filepath.Join(h.config.CharacterDir, "skills", name+".md")
	if err := os.Remove(path); err != nil {
		fail(w, http.StatusNotFound, "skill not found")
		return
	}
	ok(w, map[string]string{"message": "deleted"})
}

func sanitizeCharFileName(name string) string {
	allowed := map[string]bool{"IDENTITY": true, "THINKING": true, "RULES": true, "MEMORY": true}
	upper := strings.ToUpper(name)
	if allowed[upper] {
		return upper
	}
	return ""
}

func sanitizeSkillName(name string) string {
	var out strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' {
			out.WriteRune(r)
		}
	}
	return out.String()
}
