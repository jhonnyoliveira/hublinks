package admin

import (
	"net/http"

	"github.com/hublinks/hublinks/internal/auth"
	webtmpl "github.com/hublinks/hublinks/web"
)

type navItem struct {
	Href, Text string
	Current    bool
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
		"Tabs":    map[string]any{"Label": "Seções de exemplo", "Items": []navItem{{"#um", "Resumo", true}, {"#dois", "Detalhes", false}}},
		"Periods": map[string]any{"Items": []navItem{{"?period=7d", "7 dias", false}, {"?period=30d", "30 dias", true}, {"?period=90d", "90 dias", false}}},
	}
	var buf bufferedWriter
	if err := webtmpl.Render(&buf, "admin", "admin/ui", data); err != nil {
		http.Error(w, "erro ao renderizar página", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.b)
}
