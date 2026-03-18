package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felan/blindsidian/internal/auth"
	"github.com/felan/blindsidian/internal/database"
)

func TestNotesHandler_NoSession(t *testing.T) {
	sessionStore := auth.NewSessionStore()
	dbManager := &database.DatabaseManager{} // Not used in this test

	server := &Server{
		SessionStore: sessionStore,
		DBManager:    dbManager,
	}

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	w := httptest.NewRecorder()

	server.NotesHandler(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("Expected status %d, got %d", http.StatusSeeOther, w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/login" {
		t.Errorf("Expected redirect to /login, got %s", location)
	}
}
