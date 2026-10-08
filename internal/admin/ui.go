package admin

import (
	webtmpl "github.com/hublinks/hublinks/web"
	"net/http"
)

func UI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := webtmpl.Render(w, "admin", "admin/ui", struct{ CSRF string }{""}); err != nil {
		http.Error(w, "erro ao renderizar página", 500)
	}
}
