package admin

import (
	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/auth"
	webtmpl "github.com/hublinks/hublinks/web"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"strings"
	"sync"
)

type Login struct {
	Pool     *pgxpool.Pool
	Sessions auth.Manager
	Limiter  *auth.LoginLimiter
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
	if l.Limiter != nil && l.Limiter.Blocked(r, email) {
		l.form(w, r.FormValue("next"), "Muitas tentativas. Aguarde alguns minutos.")
		return
	}
	var id, org uuid.UUID
	var hash string
	err := l.Pool.QueryRow(r.Context(), `SELECT u.id,u.password_hash,m.org_id FROM users u JOIN memberships m ON m.user_id=u.id WHERE u.email=$1 AND u.deleted_at IS NULL LIMIT 1`, email).Scan(&id, &hash, &org)
	if err != nil {
		// e-mail inexistente também paga o custo do argon2, para o tempo de
		// resposta não revelar quais e-mails têm conta
		hash = dummyHash()
	}
	if ok := auth.Verify(r.FormValue("password"), hash); err != nil || !ok {
		if l.Limiter != nil {
			l.Limiter.Fail(r, email)
		}
		l.form(w, r.FormValue("next"), "E-mail ou senha inválidos")
		return
	}
	if l.Limiter != nil {
		l.Limiter.Success(r, email)
	}
	raw, s, err := l.Sessions.Create(r.Context(), id, org)
	if err != nil {
		http.Error(w, "erro interno", 500)
		return
	}
	auth.SetCookie(w, raw, s.ExpiresAt)
	next := r.FormValue("next")
	next = safeNext(next)
	http.Redirect(w, r, next, http.StatusSeeOther)
}

// safeNext só aceita caminhos do próprio painel (/admin ou /admin/...); qualquer
// outra coisa, inclusive URLs absolutas e //host, volta para /admin.
func safeNext(next string) string {
	if next == "/admin" || (strings.HasPrefix(next, "/admin/") && !strings.ContainsAny(next, "\\\r\n")) {
		return next
	}
	return "/admin"
}

var (
	dummyOnce sync.Once
	dummy     string
)

func dummyHash() string {
	dummyOnce.Do(func() { dummy, _ = auth.Hash("senha-que-nunca-sera-usada") })
	return dummy
}

func (l Login) form(w http.ResponseWriter, next, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := webtmpl.Render(w, "admin", "admin/login", struct{ Next, Error, CSRF string }{next, msg, ""}); err != nil {
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
