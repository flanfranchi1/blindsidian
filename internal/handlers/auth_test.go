package handlers

import (
	"database/sql"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/flanfranchi1/notty/internal/auth"
	"github.com/flanfranchi1/notty/internal/database"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

// minimalTemplates parses a minimal version of the templates needed by auth handlers.
func minimalTemplates(t *testing.T) *template.Template {
	t.Helper()
	tmpl := template.Must(template.New("forgot_password.gohtml").Parse(`{{define "forgot_password.gohtml"}}ERROR={{.Error}} SUCCESS={{.Success}}{{end}}`))
	return tmpl
}

func setupSystemDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open system db: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`)
	if err != nil {
		t.Fatalf("create users table: %v", err)
	}
	return db
}

func newForgotServer(t *testing.T) (*Server, *sql.DB) {
	t.Helper()
	systemDB := setupSystemDB(t)
	s := &Server{
		DBManager:         &database.DatabaseManager{},
		SessionStore:      auth.NewSessionStore(),
		SystemDB:          systemDB,
		Templates:         minimalTemplates(t),
		SessionCookieName: "test_session",
	}
	return s, systemDB
}

func TestForgotPasswordHandler_GET(t *testing.T) {
	s, db := newForgotServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/forgot-password", nil)
	w := httptest.NewRecorder()

	s.ForgotPasswordHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestForgotPasswordHandler_MissingFields(t *testing.T) {
	s, db := newForgotServer(t)
	defer db.Close()

	form := url.Values{"email": {"a@b.com"}} // missing passwords
	req := httptest.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	s.ForgotPasswordHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "All fields are required") {
		t.Errorf("expected 'All fields are required' in body, got: %s", body)
	}
}

func TestForgotPasswordHandler_PasswordMismatch(t *testing.T) {
	s, db := newForgotServer(t)
	defer db.Close()

	form := url.Values{
		"email":            {"a@b.com"},
		"new_password":     {"password1"},
		"confirm_password": {"password2"},
	}
	req := httptest.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	s.ForgotPasswordHandler(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Passwords do not match") {
		t.Errorf("expected 'Passwords do not match' in body, got: %s", body)
	}
}

func TestForgotPasswordHandler_PasswordTooShort(t *testing.T) {
	s, db := newForgotServer(t)
	defer db.Close()

	form := url.Values{
		"email":            {"a@b.com"},
		"new_password":     {"short"},
		"confirm_password": {"short"},
	}
	req := httptest.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	s.ForgotPasswordHandler(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "at least 8 characters") {
		t.Errorf("expected length error in body, got: %s", body)
	}
}

func TestForgotPasswordHandler_UnknownEmail(t *testing.T) {
	s, db := newForgotServer(t)
	defer db.Close()

	form := url.Values{
		"email":            {"ghost@example.com"},
		"new_password":     {"newpassword1"},
		"confirm_password": {"newpassword1"},
	}
	req := httptest.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	s.ForgotPasswordHandler(w, req)

	// Unknown email must show same success message (no enumeration)
	body := w.Body.String()
	if !strings.Contains(body, "password has been updated") {
		t.Errorf("expected success message for unknown email, got: %s", body)
	}
}

func TestForgotPasswordHandler_Success(t *testing.T) {
	s, db := newForgotServer(t)
	defer db.Close()

	oldHash, _ := bcrypt.GenerateFromPassword([]byte("oldpassword"), bcrypt.MinCost)
	user := database.User{ID: "user-99", Email: "carol@example.com", PasswordHash: string(oldHash)}
	if err := s.DBManager.CreateSystemUser(db, user); err != nil {
		t.Fatalf("CreateSystemUser: %v", err)
	}

	newPassword := "newpassword123"
	form := url.Values{
		"email":            {user.Email},
		"new_password":     {newPassword},
		"confirm_password": {newPassword},
	}
	req := httptest.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	s.ForgotPasswordHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "password has been updated") {
		t.Errorf("expected success message, got: %s", body)
	}

	// Verify the hash stored in the DB matches the new password
	updated, err := s.DBManager.GetUserByEmail(db, user.Email)
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte(newPassword)); err != nil {
		t.Errorf("stored hash does not match new password: %v", err)
	}
}
