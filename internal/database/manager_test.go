package database

import (
	"database/sql"
	"reflect"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	manager := &DatabaseManager{}
	if err := manager.ensureUserSchema(db); err != nil {
		t.Fatalf("Failed to setup schema: %v", err)
	}

	return db
}

func TestCreateNoteAndGetNoteByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	manager := &DatabaseManager{}

	note := Note{
		ID:         "test-id",
		Title:      "Test Note",
		Content:    "This is test content.",
		NotebookID: "notebook-1",
	}

	err := manager.CreateNote(db, note)
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	retrieved, err := manager.GetNoteByID(db, "test-id")
	if err != nil {
		t.Fatalf("GetNoteByID failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Note not found")
	}

	if retrieved.ID != note.ID || retrieved.Title != note.Title || retrieved.Content != note.Content || retrieved.NotebookID != note.NotebookID {
		t.Errorf("Retrieved note does not match: got %+v, want %+v", retrieved, note)
	}
}

func TestInsertNoteTags(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	manager := &DatabaseManager{}

	note := Note{
		ID:    "test-id",
		Title: "Test Note",
	}

	err := manager.CreateNote(db, note)
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	initialTags := []string{"tag1", "tag2"}
	err = manager.InsertNoteTags(db, "test-id", initialTags)
	if err != nil {
		t.Fatalf("InsertNoteTags failed: %v", err)
	}

	rows, err := db.Query("SELECT tag FROM note_tags WHERE note_id = ? ORDER BY tag", "test-id")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		tags = append(tags, tag)
	}

	expected := []string{"tag1", "tag2"}
	if !reflect.DeepEqual(tags, expected) {
		t.Errorf("Initial tags = %v, want %v", tags, expected)
	}

	newTags := []string{"tag3", "tag4"}
	err = manager.InsertNoteTags(db, "test-id", newTags)
	if err != nil {
		t.Fatalf("InsertNoteTags update failed: %v", err)
	}

	rows, err = db.Query("SELECT tag FROM note_tags WHERE note_id = ? ORDER BY tag", "test-id")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	tags = []string{}
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		tags = append(tags, tag)
	}

	expected = []string{"tag3", "tag4"}
	if !reflect.DeepEqual(tags, expected) {
		t.Errorf("Updated tags = %v, want %v", tags, expected)
	}
}

func TestSearchNotes(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	manager := &DatabaseManager{}

	notes := []Note{
		{ID: "1", Title: "Go Programming", Content: "Learning Go language."},
		{ID: "2", Title: "Python Basics", Content: "Introduction to Python."},
		{ID: "3", Title: "Database Design", Content: "Designing databases."},
	}

	for _, note := range notes {
		err := manager.CreateNote(db, note)
		if err != nil {
			t.Fatalf("CreateNote failed: %v", err)
		}
	}

	results, err := manager.SearchNotes(db, "Go")
	if err != nil {
		t.Fatalf("SearchNotes failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0].Title != "Go Programming" {
		t.Errorf("Expected title 'Go Programming', got '%s'", results[0].Title)
	}

	results, err = manager.SearchNotes(db, "Python")
	if err != nil {
		t.Fatalf("SearchNotes failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0].Title != "Python Basics" {
		t.Errorf("Expected title 'Python Basics', got '%s'", results[0].Title)
	}
}
