package web

import (
	"github.com/hublinks/hublinks/internal/config"
	webtmpl "github.com/hublinks/hublinks/web"
	"net/http"
)

func Privacy(c config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := webtmpl.Render(w, "public", "public/privacy", c); err != nil {
			http.Error(w, "erro ao renderizar página", http.StatusInternalServerError)
		}
	}
}
