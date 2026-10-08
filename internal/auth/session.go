package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"time"
)

const CookieName = "hl_session"

type Session struct {
	UserID, OrgID uuid.UUID
	CSRF          string
	ExpiresAt     time.Time
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
	s := Session{UserID: user, OrgID: org, CSRF: csrf, ExpiresAt: now.Add(7 * 24 * time.Hour)}
	sum := sha256.Sum256([]byte(raw))
	_, err := m.Pool.Exec(ctx, "INSERT INTO sessions(token_hash,user_id,org_id,csrf_token,created_at,last_seen_at,expires_at) VALUES($1,$2,$3,$4,$5,$5,$6)", sum[:], user, org, csrf, now, s.ExpiresAt)
	return raw, s, err
}
func (m Manager) Lookup(ctx context.Context, raw string) (Session, error) {
	sum := sha256.Sum256([]byte(raw))
	var s Session
	err := m.Pool.QueryRow(ctx, "SELECT user_id,org_id,csrf_token,expires_at FROM sessions WHERE token_hash=$1 AND expires_at>$2", sum[:], time.Now().UTC()).Scan(&s.UserID, &s.OrgID, &s.CSRF, &s.ExpiresAt)
	return s, err
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
		if !ok || (r.FormValue("csrf_token") != s.CSRF && r.Header.Get("X-CSRF-Token") != s.CSRF) {
			http.Error(w, "CSRF inválido", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func unauthorized(w http.ResponseWriter, r *http.Request, api bool) {
	if api {
		http.Error(w, "não autenticado", http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/admin/login?next="+r.URL.EscapedPath(), http.StatusSeeOther)
}
