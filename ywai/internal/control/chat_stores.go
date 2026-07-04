package control

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

func randID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ─── Session pins ───────────────────────────────────────────────
// OpenCode does not persist pinned sessions, so ywai stores them locally.

type chatPinsStore struct {
	mu   sync.RWMutex
	path string
	ids  []string
}

func newChatPinsStore() *chatPinsStore {
	dir := config.DataDir()
	_ = os.MkdirAll(dir, 0755)
	s := &chatPinsStore{path: filepath.Join(dir, "chat-pins.json")}
	if data, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(data, &s.ids)
	}
	if s.ids == nil {
		s.ids = []string{}
	}
	return s
}

func (s *chatPinsStore) list() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.ids))
	copy(out, s.ids)
	return out
}

func (s *chatPinsStore) pin(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.ids {
		if existing == id {
			return
		}
	}
	s.ids = append(s.ids, id)
	s.save()
}

func (s *chatPinsStore) unpin(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := s.ids[:0:0]
	for _, existing := range s.ids {
		if existing != id {
			filtered = append(filtered, existing)
		}
	}
	s.ids = filtered
	s.save()
}

func (s *chatPinsStore) save() {
	if data, err := json.MarshalIndent(s.ids, "", "  "); err == nil {
		_ = os.WriteFile(s.path, data, 0644)
	}
}

// ─── Prompt templates ───────────────────────────────────────────

type promptTemplate struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type chatTemplatesStore struct {
	mu    sync.RWMutex
	path  string
	items []promptTemplate
}

func newChatTemplatesStore() *chatTemplatesStore {
	dir := config.DataDir()
	_ = os.MkdirAll(dir, 0755)
	s := &chatTemplatesStore{path: filepath.Join(dir, "chat-templates.json")}
	if data, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(data, &s.items)
	}
	if s.items == nil {
		s.items = []promptTemplate{}
	}
	return s
}

func (s *chatTemplatesStore) list() []promptTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]promptTemplate, len(s.items))
	copy(out, s.items)
	return out
}

// upsert creates a template (empty id) or updates an existing one. Returns the
// stored template, or false if an update targeted a missing id.
func (s *chatTemplatesStore) upsert(t promptTemplate) (promptTemplate, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.ID == "" {
		t.ID = randID()
		s.items = append(s.items, t)
		s.save()
		return t, true
	}
	for i := range s.items {
		if s.items[i].ID == t.ID {
			s.items[i].Name = t.Name
			s.items[i].Content = t.Content
			s.save()
			return s.items[i], true
		}
	}
	return promptTemplate{}, false
}

func (s *chatTemplatesStore) delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := s.items[:0:0]
	for _, item := range s.items {
		if item.ID != id {
			filtered = append(filtered, item)
		}
	}
	s.items = filtered
	s.save()
}

func (s *chatTemplatesStore) save() {
	if data, err := json.MarshalIndent(s.items, "", "  "); err == nil {
		_ = os.WriteFile(s.path, data, 0644)
	}
}

// registerChatLocalStores mounts ywai-local chat endpoints (pins, templates).
// These are independent of OpenCode and always available.
func (s *Server) registerChatLocalStores() {
	pins := newChatPinsStore()
	s.mux.HandleFunc("GET /api/chat/pins", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"pins": pins.list()})
	})
	s.mux.HandleFunc("POST /api/chat/pins/{id}", func(w http.ResponseWriter, r *http.Request) {
		pins.pin(r.PathValue("id"))
		writeJSON(w, http.StatusOK, map[string]any{"pins": pins.list()})
	})
	s.mux.HandleFunc("DELETE /api/chat/pins/{id}", func(w http.ResponseWriter, r *http.Request) {
		pins.unpin(r.PathValue("id"))
		writeJSON(w, http.StatusOK, map[string]any{"pins": pins.list()})
	})

	tmpl := newChatTemplatesStore()
	s.mux.HandleFunc("GET /api/chat/templates", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"templates": tmpl.list()})
	})
	s.mux.HandleFunc("POST /api/chat/templates", func(w http.ResponseWriter, r *http.Request) {
		var t promptTemplate
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil || t.Name == "" {
			http.Error(w, "name and content required", http.StatusBadRequest)
			return
		}
		t.ID = "" // force create
		saved, _ := tmpl.upsert(t)
		writeJSON(w, http.StatusCreated, saved)
	})
	s.mux.HandleFunc("PUT /api/chat/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		var t promptTemplate
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		t.ID = r.PathValue("id")
		saved, ok := tmpl.upsert(t)
		if !ok {
			http.Error(w, "template not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, saved)
	})
	s.mux.HandleFunc("DELETE /api/chat/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		tmpl.delete(r.PathValue("id"))
		w.WriteHeader(http.StatusNoContent)
	})
}
