package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/flanfranchi1/notty/internal/database"
	"github.com/flanfranchi1/notty/internal/i18n"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const sessionDuration = 24 * time.Hour

func (s *Server) currentUserID(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(s.SessionCookieName)
	if err != nil {
		return "", false
	}
	userID, ok := s.SessionStore.GetUserID(cookie.Value)
	return userID, ok
}

func (s *Server) SignupHandler(w http.ResponseWriter, r *http.Request) {
	errMsg := ""
	if r.Method == http.MethodPost {
		email := r.FormValue("email")
		password := r.FormValue("password")
		if email == "" || password == "" {
			errMsg = "Email and password are required."
		} else {
			existingUser, err := s.DBManager.GetUserByEmail(s.SystemDB, email)
			if err != nil {
				log.Printf("SignupHandler: GetUserByEmail: %v", err)
				http.Error(w, s.t(r, "error.internal"), http.StatusInternalServerError)
				return
			}
			if existingUser != nil {
				errMsg = "Email is already registered."
			} else {
				uid := uuid.NewString()
				hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
				if err != nil {
					log.Printf("SignupHandler: GenerateFromPassword: %v", err)
					http.Error(w, s.t(r, "error.internal"), http.StatusInternalServerError)
					return
				}

				if err := s.DBManager.CreateSystemUser(s.SystemDB, database.User{ID: uid, Email: email, PasswordHash: string(hash)}); err != nil {
					log.Printf("SignupHandler: CreateSystemUser: %v", err)
					http.Error(w, s.t(r, "error.internal"), http.StatusInternalServerError)
					return
				}

				if _, err := s.DBManager.CreateUserDB(uid); err != nil {
					log.Printf("SignupHandler: CreateUserDB: %v", err)
					http.Error(w, s.t(r, "error.internal"), http.StatusInternalServerError)
					return
				}

				if s.Bundle != nil {
					locale := i18n.LocaleFromContext(r.Context())
					if userDB, err := s.DBManager.OpenUserDB(uid); err != nil {
						log.Printf("SignupHandler: OpenUserDB for tutorial: %v", err)
					} else {
						if seedErr := s.DBManager.SeedTutorial(userDB, s.Bundle.Translations(locale)); seedErr != nil {
							log.Printf("SignupHandler: SeedTutorial: %v", seedErr)
						}
						userDB.Close()
					}
				}

				token, err := s.SessionStore.CreateSession(uid)
				if err != nil {
					log.Printf("SignupHandler: CreateSession: %v", err)
					http.Error(w, s.t(r, "error.internal"), http.StatusInternalServerError)
					return
				}

				http.SetCookie(w, &http.Cookie{
					Name:     s.SessionCookieName,
					Value:    token,
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
					Secure:   false,
					Expires:  time.Now().Add(sessionDuration),
				})
				http.Redirect(w, r, "/notes", http.StatusSeeOther)
				return
			}
		}
	}

	s.RenderTemplate(w, r, "signup.gohtml", map[string]string{"Error": errMsg})
}

func (s *Server) LoginHandler(w http.ResponseWriter, r *http.Request) {
	errMsg := ""
	if r.Method == http.MethodPost {
		email := r.FormValue("email")
		password := r.FormValue("password")
		if email == "" || password == "" {
			errMsg = "Email and password are required."
		} else {
			user, err := s.DBManager.GetUserByEmail(s.SystemDB, email)
			if err != nil {
				log.Printf("LoginHandler: GetUserByEmail: %v", err)
				http.Error(w, s.t(r, "error.internal"), http.StatusInternalServerError)
				return
			}
			if user == nil {
				errMsg = "Invalid email or password."
			} else if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
				errMsg = "Invalid email or password."
			} else {
				token, err := s.SessionStore.CreateSession(user.ID)
				if err != nil {
					log.Printf("LoginHandler: CreateSession: %v", err)
					http.Error(w, s.t(r, "error.internal"), http.StatusInternalServerError)
					return
				}
				http.SetCookie(w, &http.Cookie{
					Name:     s.SessionCookieName,
					Value:    token,
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
					Secure:   false,
					Expires:  time.Now().Add(sessionDuration),
				})
				http.Redirect(w, r, "/notes", http.StatusSeeOther)
				return
			}
		}
	}
	s.RenderTemplate(w, r, "login.gohtml", map[string]string{"Error": errMsg})
}

func (s *Server) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(s.SessionCookieName)
	if err == nil {
		s.SessionStore.DeleteSession(cookie.Value)
		http.SetCookie(w, &http.Cookie{Name: s.SessionCookieName, Value: "", Path: "/", Expires: time.Unix(0, 0), MaxAge: -1})
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) ForgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]string{"Error": "", "Success": ""}
	if r.Method == http.MethodPost {
		email := r.FormValue("email")
		newPassword := r.FormValue("new_password")
		confirmPassword := r.FormValue("confirm_password")

		switch {
		case email == "" || newPassword == "" || confirmPassword == "":
			data["Error"] = "All fields are required."
		case newPassword != confirmPassword:
			data["Error"] = "Passwords do not match."
		case len(newPassword) < 8:
			data["Error"] = "Password must be at least 8 characters."
		default:
			user, err := s.DBManager.GetUserByEmail(s.SystemDB, email)
			if err != nil {
				log.Printf("ForgotPasswordHandler: GetUserByEmail: %v", err)
				http.Error(w, s.t(r, "error.internal"), http.StatusInternalServerError)
				return
			}
			if user == nil {
				// Return same success message to avoid email enumeration
				data["Success"] = "If that email is registered, the password has been updated."
			} else {
				hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
				if err != nil {
					log.Printf("ForgotPasswordHandler: GenerateFromPassword: %v", err)
					http.Error(w, s.t(r, "error.internal"), http.StatusInternalServerError)
					return
				}
				if err := s.DBManager.UpdateUserPassword(s.SystemDB, user.ID, string(hash)); err != nil {
					log.Printf("ForgotPasswordHandler: UpdateUserPassword: %v", err)
					http.Error(w, s.t(r, "error.internal"), http.StatusInternalServerError)
					return
				}
				data["Success"] = "If that email is registered, the password has been updated."
			}
		}
	}
	s.RenderTemplate(w, r, "forgot_password.gohtml", data)
}
