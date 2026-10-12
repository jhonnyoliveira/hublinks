package admin

import (
	"net/http"

	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/stats"
	webtmpl "github.com/hublinks/hublinks/web"
)

type navItem struct {
	Href, Text string
	Current    bool
}

var sampleDays = []stats.DayCount{
	{Day: "2026-10-01", Clicks: 12, Unique: 9}, {Day: "2026-10-02", Clicks: 18, Unique: 11}, {Day: "2026-10-03", Clicks: 7, Unique: 6},
	{Day: "2026-10-04", Clicks: 25, Unique: 17}, {Day: "2026-10-05", Clicks: 21, Unique: 15}, {Day: "2026-10-06", Clicks: 30, Unique: 19}, {Day: "2026-10-07", Clicks: 16, Unique: 12},
}

// UI exibe o guia de componentes (somente em desenvolvimento).
func UI(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	data := map[string]any{
		"CSRF": s.CSRF,
		"Table": map[string]any{
			"Caption": "Exemplo de tabela", "Columns": []string{"Nome", "Cliques"},
			"Rows": [][]string{{"Produto A", "120"}, {"Produto B", "87"}}, "EmptyText": "Sem dados.",
		},
		"Tabs":         map[string]any{"Label": "Seções de exemplo", "Items": []navItem{{"#um", "Resumo", true}, {"#dois", "Detalhes", false}}},
		"Periods":      map[string]any{"Items": []navItem{{"?period=7d", "7 dias", false}, {"?period=30d", "30 dias", true}, {"?period=90d", "90 dias", false}}},
		"PeriodCustom": periodView{From: "2026-10-01", To: "2026-10-07"},
		"Chart":        chartData(sampleDays),
		"DayRows":      dayRows(sampleDays),
	}
	var buf bufferedWriter
	if err := webtmpl.Render(&buf, "admin", "admin/ui", data); err != nil {
		http.Error(w, "erro ao renderizar página", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.b)
}
