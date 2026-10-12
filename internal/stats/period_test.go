package stats

import (
	"errors"
	"testing"
	"time"
)

func TestParsePeriod(t *testing.T) {
	tz, _ := time.LoadLocation("America/Sao_Paulo")
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, tz)
	for _, c := range []struct {
		period, from, to string
		first, last      string
		days             int
	}{
		{"", "", "", "2026-10-06", "2026-10-12", 7},
		{"7d", "", "", "2026-10-06", "2026-10-12", 7},
		{"30d", "", "", "2026-09-13", "2026-10-12", 30},
		{"90d", "", "", "2026-07-15", "2026-10-12", 90},
		{"", "2026-10-01", "2026-10-03", "2026-10-01", "2026-10-03", 3},
		{"", "2025-10-12", "2026-10-12", "2025-10-12", "2026-10-12", 366},
	} {
		p, err := ParsePeriod(c.period, c.from, c.to, now, tz)
		if err != nil {
			t.Fatalf("%+v: %v", c, err)
		}
		if p.From.Format(dateLayout) != c.first || p.To.Format(dateLayout) != c.last || p.Days() != c.days {
			t.Fatalf("%+v: %s..%s (%d dias)", c, p.From.Format(dateLayout), p.To.Format(dateLayout), p.Days())
		}
	}
}

func TestParsePeriodErrors(t *testing.T) {
	tz, _ := time.LoadLocation("America/Sao_Paulo")
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, tz)
	for _, c := range [][3]string{
		{"1y", "", ""},
		{"", "2026-10-05", "2026-10-01"}, // from > to
		{"", "2025-10-11", "2026-10-12"}, // 367 dias
		{"", "2026-10-01", ""},
		{"", "ontem", "2026-10-01"},
	} {
		if _, err := ParsePeriod(c[0], c[1], c[2], now, tz); !errors.Is(err, ErrInvalidPeriod) {
			t.Fatalf("%v: err = %v", c, err)
		}
	}
}

func TestParsePeriodUsesReportTZForToday(t *testing.T) {
	tz, _ := time.LoadLocation("America/Sao_Paulo")
	// 02:30 UTC de 12/10 ainda é 11/10 em Brasília.
	p, err := ParsePeriod("7d", "", "", time.Date(2026, 10, 12, 2, 30, 0, 0, time.UTC), tz)
	if err != nil || p.To.Format(dateLayout) != "2026-10-11" {
		t.Fatalf("to = %v (%v)", p.To, err)
	}
}
