package api

import (
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/stats"
	"net/http"
	"time"
)

type Stats struct{ Service stats.Service }

func (s Stats) Summary(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.FromContext(r.Context())
	to := time.Now()
	from := to.AddDate(0, 0, -7)
	if r.URL.Query().Get("period") == "30d" {
		from = to.AddDate(0, 0, -30)
	}
	if r.URL.Query().Get("period") == "90d" {
		from = to.AddDate(0, 0, -90)
	}
	total, err := s.Service.Summary(r.Context(), session.OrgID, from, to)
	if err != nil {
		errorJSON(w, 500, "internal_error", "erro ao consultar estatísticas")
		return
	}
	jsonOut(w, 200, total)
}
