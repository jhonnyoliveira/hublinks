package admin

import (
	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/auth"
	webtmpl "github.com/hublinks/hublinks/web"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"strings"
)

type Login struct {
	Pool     *pgxpool.Pool
	Sessions auth.Manager
}

func (l Login) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		l.form(w, r.URL.Query().Get("next"), "")
		return
	}
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	var id, org uuid.UUID
	var hash string
	err := l.Pool.QueryRow(r.Context(), `SELECT u.id,u.password_hash,m.org_id FROM users u JOIN memberships m ON m.user_id=u.id WHERE u.email=$1 AND u.deleted_at IS NULL LIMIT 1`, email).Scan(&id, &hash, &org)
	if err != nil || !auth.Verify(r.FormValue("password"), hash) {
		l.form(w, r.FormValue("next"), "E-mail ou senha inválidos")
		return
	}
	raw, s, err := l.Sessions.Create(r.Context(), id, org)
	if err != nil {
		http.Error(w, "erro interno", 500)
		return
	}
	auth.SetCookie(w, raw, s.ExpiresAt)
	next := r.FormValue("next")
	if !strings.HasPrefix(next, "/admin") || strings.HasPrefix(next, "//") {
		next = "/admin"
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}
func (l Login) form(w http.ResponseWriter, next, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := webtmpl.Render(w, "admin", "admin/login", struct{ Next, Error string }{next, msg}); err != nil {
		http.Error(w, "erro ao renderizar página", http.StatusInternalServerError)
	}
}
func Logout(m auth.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, e := r.Cookie(auth.CookieName); e == nil {
			_ = m.Destroy(r.Context(), c.Value)
		}
		auth.ClearCookie(w)
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	}
}
func Dashboard(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := webtmpl.Render(w, "admin", "admin/dashboard", struct{ CSRF string }{s.CSRF}); err != nil {
		http.Error(w, "erro ao renderizar página", http.StatusInternalServerError)
	}
}

var _ = pgx.ErrNoRows
