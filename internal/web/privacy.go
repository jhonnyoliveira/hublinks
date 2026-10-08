package web

import (
	"fmt"
	"github.com/hublinks/hublinks/internal/config"
	"net/http"
)

func Privacy(c config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, `<!doctype html><html lang="pt-BR"><body><h1>Privacidade</h1><p>Usamos um identificador anônimo derivado de IP e user-agent, protegido por hash, sem cookies de rastreamento. A finalidade é produzir estatísticas de divulgação.</p><p>Eventos reais são retidos por %d dias; eventos de robôs por %d dias; itens na lixeira por %d dias.</p><p>Para solicitar exclusão, escreva para %s.</p></body></html>`, c.EventsRetentionDays, c.BotEventsRetentionDays, c.TrashRetentionDays, c.PrivacyContact)
	}
}
