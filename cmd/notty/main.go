package main

import (
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/flanfranchi1/notty/internal/auth"
	"github.com/flanfranchi1/notty/internal/database"
	"github.com/flanfranchi1/notty/internal/handlers"
)

type Config struct {
	Port        string
	StoragePath string
	SessionName string
}

func loadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	}
	storagePath := os.Getenv("STORAGE_PATH")
	if storagePath == "" {
		storagePath = "./storage"
	}
	sessionName := os.Getenv("SESSION_NAME")
	if sessionName == "" {
		sessionName = "notty_session"
	}
	return Config{
		Port:        port,
		StoragePath: storagePath,
		SessionName: sessionName,
	}
}

func main() {
	cfg := loadConfig()

	mgr := database.NewManager(cfg.StoragePath)
	systemDB, err := mgr.InitSystemDB()
	if err != nil {
		log.Fatalf("failed to init system db: %v", err)
	}
	defer systemDB.Close()

	sessions := auth.NewSessionStore()

	templates := template.Must(template.New("pages").ParseGlob("./web/templates/*.gohtml"))

	server := &handlers.Server{
		DBManager:         mgr,
		SessionStore:      sessions,
		SystemDB:          systemDB,
		Templates:         templates,
		SessionCookieName: cfg.SessionName,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		server.RenderTemplate(w, "home.gohtml", nil)
	})
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static"))))
	http.HandleFunc("/signup", server.SignupHandler)
	http.HandleFunc("/login", server.LoginHandler)
	http.HandleFunc("/notes", server.NotesHandler)
	http.HandleFunc("/notes/create", server.CreateNoteHandler)
	http.HandleFunc("/notebooks/create", server.CreateNotebookHandler)
	http.HandleFunc("/notebooks/", server.NotebookViewHandler)
	http.HandleFunc("/notes/view", server.ViewNoteHandler)
	http.HandleFunc("/notes/view/edit", server.ViewNoteEditHandler)
	http.HandleFunc("/notes/view/update", server.ViewNoteUpdateHandler)
	http.HandleFunc("/notes/", server.NoteActionHandler)
	http.HandleFunc("/search", server.SearchHandler)
	http.HandleFunc("/logout", server.LogoutHandler)

	log.Printf("starting server on %s", cfg.Port)
	if err := http.ListenAndServe(cfg.Port, nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
