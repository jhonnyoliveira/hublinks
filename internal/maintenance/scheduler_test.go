package maintenance

import (
	"testing"
	"time"

	"github.com/hublinks/hublinks/internal/config"
)

func TestNextRunIsTheNextMaintenanceAtInReportTZ(t *testing.T) {
	tz, _ := time.LoadLocation("America/Sao_Paulo")
	m := Manager{Config: config.Config{ReportTZ: tz}}
	clock, _ := time.Parse("15:04", "00:30")
	for _, c := range []struct{ now, want string }{
		{"2026-10-12T12:00:00-03:00", "2026-10-13T00:30:00-03:00"}, // depois do horário: amanhã
		{"2026-10-12T00:10:00-03:00", "2026-10-12T00:30:00-03:00"}, // antes: hoje
		{"2026-10-12T00:30:00-03:00", "2026-10-13T00:30:00-03:00"}, // exatamente no horário: amanhã
		{"2026-10-12T03:10:00Z", "2026-10-12T00:30:00-03:00"},      // 00:10 em Brasília, informado em UTC
		{"2026-12-31T23:59:00-03:00", "2027-01-01T00:30:00-03:00"}, // virada de ano
	} {
		now, _ := time.Parse(time.RFC3339, c.now)
		want, _ := time.Parse(time.RFC3339, c.want)
		if got := m.nextRun(clock, now); !got.Equal(want) {
			t.Errorf("now=%s: next=%s, esperado %s", c.now, got, want)
		}
	}
}

func TestStartRejectsInvalidMaintenanceAt(t *testing.T) {
	m := Manager{Config: config.Config{MaintenanceAt: "meia-noite"}}
	if err := m.Start(nil); err == nil { //nolint:staticcheck // o erro vem antes de usar o contexto
		t.Fatal("MAINTENANCE_AT inválido deveria falhar na partida")
	}
}
