package handlers

import (
	"database/sql"
	"html/template"

	"github.com/flanfranchi1/notty/internal/auth"
	"github.com/flanfranchi1/notty/internal/database"
	"github.com/flanfranchi1/notty/internal/i18n"
)

type Server struct {
	DBManager         *database.DatabaseManager
	SessionStore      *auth.SessionStore
	SystemDB          *sql.DB
	Templates         *template.Template
	SessionCookieName string
	Bundle            *i18n.Bundle
}
