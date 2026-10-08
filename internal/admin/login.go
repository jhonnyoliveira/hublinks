package admin

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"html"
	"net/http"
	"strings"
)

type Login struct {
	Pool     *pgxpool.Pool
	Sessions auth.Manager
}

func (l Login) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		l.form(w, "")
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
		l.form(w, "E-mail ou senha inválidos")
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
func (l Login) form(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html><html lang="pt-BR"><body><h1>Entrar</h1><p>%s</p><form method="post"><label>E-mail <input name="email" type="email" required></label><label>Senha <input name="password" type="password" required></label><button>Entrar</button></form></body></html>`, html.EscapeString(msg))
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
	_, _ = fmt.Fprintf(w, `<!doctype html><html lang="pt-BR"><body><nav><form action="/admin/logout" method="post"><input type="hidden" name="csrf_token" value="%s"><button>Sair</button></form></nav><h1>Painel</h1><p>Cadastre marketplaces, canais e links pela API administrativa. As estatísticas são atualizadas de forma assíncrona.</p></body></html>`, html.EscapeString(s.CSRF))
}

var _ = pgx.ErrNoRows
