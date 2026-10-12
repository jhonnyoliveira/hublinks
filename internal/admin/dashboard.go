package admin

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/hublinks/hublinks/internal/stats"
	webtmpl "github.com/hublinks/hublinks/web"
)

// Dashboard é a tela inicial do painel: totais, série diária, top 5 links e
// comparação entre canais, para o período escolhido.
type Dashboard struct{ Stats stats.Service }

type dashboardData struct {
	CSRF     string
	Period   periodView
	Summary  stats.Summary
	Channels []stats.ChannelStat
	Chart    template.JS
	DayRows  [][]string
	Error    string
}

func (d Dashboard) Show(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	p, view := resolvePeriod(d.Stats, r.URL.Query(), "/admin", "#dashboard")
	data := dashboardData{CSRF: s.CSRF, Period: view}
	summary, err := d.Stats.Summary(r.Context(), s.OrgID, p)
	if err == nil {
		data.Summary = summary
		data.Chart, data.DayRows = chartData(summary.Daily), dayRows(summary.Daily)
		data.Channels, err = d.Stats.Channels(r.Context(), s.OrgID, p)
	}
	if err != nil {
		slog.Error("falha ao consultar estatísticas do painel", "error", err)
		data.Error = "Não foi possível carregar as estatísticas agora. Tente novamente em instantes."
		data.Chart = chartData(nil)
	}
	if isHX(r) && r.Header.Get("HX-Target") == "dashboard" {
		w.Header().Set("Vary", "HX-Request")
		renderFragment(w, "dashboard_body", data, "admin/dashboard.html")
		return
	}
	renderPage(w, http.StatusOK, data, "admin/dashboard.html")
}

func renderFragment(w http.ResponseWriter, name string, data any, files ...string) {
	var buf bufferedWriter
	if err := webtmpl.RenderFragment(&buf, name, data, files...); err != nil {
		slog.Error("falha ao renderizar fragmento do painel", "name", name, "error", err)
		http.Error(w, "erro ao renderizar página", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.b)
}
