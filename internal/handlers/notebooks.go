package handlers

import (
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/flanfranchi1/notty/internal/database"
	"github.com/flanfranchi1/notty/internal/markup"
	"github.com/google/uuid"
)

func (s *Server) CreateNotebookHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUserID(r)
	if !ok {
		http.Error(w, s.t(r, "error.unauthorized"), http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, s.t(r, "error.method_not_allowed"), http.StatusMethodNotAllowed)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Redirect(w, r, "/notes?msg=Notebook+name+required", http.StatusSeeOther)
		return
	}

	db, err := s.DBManager.OpenUserDB(userID)
	if err != nil {
		log.Printf("CreateNotebookHandler: OpenUserDB: %v", err)
		http.Error(w, s.t(r, "error.db_unavailable"), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	notebook := database.Notebook{ID: uuid.NewString(), Name: name}
	if err := s.DBManager.CreateNotebook(db, notebook); err != nil {
		log.Printf("CreateNotebookHandler: CreateNotebook: %v", err)
		http.Error(w, s.t(r, "error.internal"), http.StatusInternalServerError)
		return
	}
	indexContent := "## Index\n\nWelcome to your new notebook. Use headings to create chapters.\n\n### Chapter 1\n[[Write your first note here]]\n\n#index"

	indexNote := database.Note{
		ID:         uuid.NewString(),
		Title:      "Index - " + name,
		Content:    indexContent,
		NotebookID: notebook.ID,
	}

	if err := s.DBManager.CreateNote(db, indexNote); err != nil {
		log.Printf("CreateNotebookHandler: CreateNote (index): %v", err)
		http.Error(w, s.t(r, "error.internal"), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/notes?msg=Notebook+created", http.StatusSeeOther)
}

func (s *Server) NotebookViewHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	notebookID := strings.TrimPrefix(r.URL.Path, "/notebooks/")
	notebookID = strings.TrimSuffix(notebookID, "/")

	db, err := s.DBManager.OpenUserDB(userID)
	if err != nil {
		log.Printf("NotebookViewHandler: OpenUserDB: %v", err)
		http.Error(w, s.t(r, "error.db_unavailable"), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	notebook, err := s.DBManager.GetNotebookByID(db, notebookID)
	if err != nil {
		log.Printf("NotebookViewHandler: GetNotebookByID: %v", err)
		http.Error(w, s.t(r, "error.notebook_not_found"), http.StatusNotFound)
		return
	}
	if notebook == nil {
		http.Error(w, s.t(r, "error.notebook_not_found"), http.StatusNotFound)
		return
	}

	notes, err := s.DBManager.GetNotesByNotebookID(db, notebookID)
	if err != nil {
		log.Printf("NotebookViewHandler: GetNotesByNotebookID: %v", err)
		http.Error(w, s.t(r, "error.internal"), http.StatusInternalServerError)
		return
	}

	type RenderNote struct {
		ID           string
		Title        string
		Content      string
		UpdatedAt    string
		RenderedHTML template.HTML
		NotebookID   string
	}

	noteExists := func(title string) (string, bool, error) {
		n, err := s.DBManager.GetNoteByTitle(db, title)
		if err != nil {
			return "", false, err
		}
		if n == nil {
			return "", false, nil
		}
		return n.ID, true, nil
	}

	rendered := []RenderNote{}
	for _, note := range notes {
		htmlContent, err := markup.RenderMarkdownWithWikiLinks(note.Content, noteExists)
		if err != nil {
			log.Printf("NotebookViewHandler: RenderMarkdownWithWikiLinks: %v", err)
			http.Error(w, s.t(r, "error.internal"), http.StatusInternalServerError)
			return
		}
		rendered = append(rendered, RenderNote{
			ID:           note.ID,
			Title:        note.Title,
			Content:      note.Content,
			UpdatedAt:    note.UpdatedAt,
			RenderedHTML: template.HTML(htmlContent),
			NotebookID:   note.NotebookID,
		})
	}

	notebooks, err := s.DBManager.ListNotebooks(db)
	if err != nil {
		notebooks = []database.Notebook{}
	}

	s.RenderTemplate(w, r, "notes.gohtml", map[string]interface{}{"Notes": rendered, "Notebooks": notebooks, "Notebook": notebook})
}
