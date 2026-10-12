package web

import (
	"bytes"
	"log/slog"
	"net/http"

	"github.com/hublinks/hublinks/internal/config"
	webtmpl "github.com/hublinks/hublinks/web"
)

// Privacy serve /privacidade (FR-023) com os prazos e o contato da configuração.
func Privacy(c config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		if err := webtmpl.Render(&buf, "public", "public/privacy", c); err != nil {
			slog.Error("falha ao renderizar a página de privacidade", "error", err)
			http.Error(w, "erro ao renderizar página", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(buf.Bytes())
	}
}
