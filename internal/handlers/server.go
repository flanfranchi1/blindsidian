package handlers

import (
	"database/sql"
	"html/template"

	"github.com/flanfranchi1/notty/internal/auth"
	"github.com/flanfranchi1/notty/internal/database"
)

type Server struct {
	DBManager         *database.DatabaseManager
	SessionStore      *auth.SessionStore
	SystemDB          *sql.DB
	Templates         *template.Template
	SessionCookieName string
}
