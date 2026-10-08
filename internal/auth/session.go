package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const CookieName = "hl_session"

// Duração da sessão sem uso e intervalo mínimo entre renovações (evita uma
// gravação a cada requisição).
const (
	sessionTTL = 7 * 24 * time.Hour
	touchEvery = 10 * time.Minute
)

type Session struct {
	UserID, OrgID uuid.UUID
	CSRF          string
	ExpiresAt     time.Time
	// Renewed indica que este Lookup estendeu a validade; o middleware renova o
	// cookie para acompanhar.
	Renewed bool
}
type Manager struct{ Pool *pgxpool.Pool }

func token() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func (m Manager) Create(ctx context.Context, user, org uuid.UUID) (string, Session, error) {
	raw, csrf := token(), token()
	now := time.Now().UTC()
	s := Session{UserID: user, OrgID: org, CSRF: csrf, ExpiresAt: now.Add(sessionTTL)}
	sum := sha256.Sum256([]byte(raw))
	_, err := m.Pool.Exec(ctx, "INSERT INTO sessions(token_hash,user_id,org_id,csrf_token,created_at,last_seen_at,expires_at) VALUES($1,$2,$3,$4,$5,$5,$6)", sum[:], user, org, csrf, now, s.ExpiresAt)
	return raw, s, err
}

// Lookup devolve a sessão válida do token e, quando ela já não é tocada há
// touchEvery, renova a validade: a sessão expira após 7 dias sem uso, não 7 dias
// após o login.
func (m Manager) Lookup(ctx context.Context, raw string) (Session, error) {
	sum := sha256.Sum256([]byte(raw))
	now := time.Now().UTC()
	var s Session
	var lastSeen time.Time
	err := m.Pool.QueryRow(ctx, "SELECT user_id,org_id,csrf_token,expires_at,last_seen_at FROM sessions WHERE token_hash=$1 AND expires_at>$2", sum[:], now).Scan(&s.UserID, &s.OrgID, &s.CSRF, &s.ExpiresAt, &lastSeen)
	if err != nil {
		return s, err
	}
	if now.Sub(lastSeen) >= touchEvery {
		expires := now.Add(sessionTTL)
		if _, uerr := m.Pool.Exec(ctx, "UPDATE sessions SET last_seen_at=$2, expires_at=$3 WHERE token_hash=$1", sum[:], now, expires); uerr == nil {
			s.ExpiresAt, s.Renewed = expires, true
		}
	}
	return s, nil
}
func (m Manager) Destroy(ctx context.Context, raw string) error {
	sum := sha256.Sum256([]byte(raw))
	_, err := m.Pool.Exec(ctx, "DELETE FROM sessions WHERE token_hash=$1", sum[:])
	return err
}
func SetCookie(w http.ResponseWriter, raw string, expiry time.Time) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: raw, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, Expires: expiry})
}
func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

type contextKey string

const sessionKey contextKey = "session"

func FromContext(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(sessionKey).(Session)
	return s, ok
}

// WithSession attaches an already authenticated session to a request. It is
// useful for composing protected handlers in tests; production requests use
// Manager.Require.
func WithSession(r *http.Request, s Session) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), sessionKey, s))
}
func (m Manager) Require(next http.Handler, api bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := r.Cookie(CookieName)
		if e != nil {
			unauthorized(w, r, api)
			return
		}
		s, e := m.Lookup(r.Context(), c.Value)
		if e != nil {
			if e == pgx.ErrNoRows {
				unauthorized(w, r, api)
				return
			}
			http.Error(w, "erro interno", 500)
			return
		}
		if s.Renewed {
			SetCookie(w, c.Value, s.ExpiresAt)
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey, s)))
	})
}
func RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		s, ok := FromContext(r.Context())
		if !ok || !sameOrigin(r) || !validCSRF(s.CSRF, r.FormValue("csrf_token"), r.Header.Get("X-CSRF-Token")) {
			forbidden(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// validCSRF aceita o token do formulário ou do cabeçalho, comparando em tempo
// constante. Token vazio nunca vale.
func validCSRF(want string, got ...string) bool {
	if want == "" {
		return false
	}
	for _, g := range got {
		if subtle.ConstantTimeCompare([]byte(g), []byte(want)) == 1 {
			return true
		}
	}
	return false
}

// sameOrigin recusa escritas cujo cabeçalho Origin aponte para outro site.
// Clientes sem Origin (curl, testes) seguem para a checagem do token.
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host
}
func forbidden(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		httpx.APIError(w, http.StatusForbidden, "forbidden", "CSRF inválido", nil)
		return
	}
	http.Error(w, "CSRF inválido", http.StatusForbidden)
}
func unauthorized(w http.ResponseWriter, r *http.Request, api bool) {
	if api {
		httpx.APIError(w, http.StatusUnauthorized, "unauthorized", "não autenticado", nil)
		return
	}
	http.Redirect(w, r, "/admin/login?next="+r.URL.EscapedPath(), http.StatusSeeOther)
}
