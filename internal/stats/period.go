package stats

import (
	"errors"
	"fmt"
	"time"
)

// MaxPeriodDays é o maior intervalo aceito (inclusive) em uma consulta.
const MaxPeriodDays = 366

const dateLayout = "2006-01-02"

var ErrInvalidPeriod = errors.New("período inválido")

// Period é um intervalo de dias civis em REPORT_TZ, com extremos inclusivos.
type Period struct {
	From, To time.Time // meia-noite de cada dia em TZ
	TZ       *time.Location
}

// ParsePeriod interpreta ?period=7d|30d|90d (padrão 7d) ou ?from=&to=
// (YYYY-MM-DD, inclusive). Os períodos relativos terminam hoje, em tz.
func ParsePeriod(period, from, to string, now time.Time, tz *time.Location) (Period, error) {
	today := dayStart(now.In(tz), tz)
	p := Period{TZ: tz, To: today}
	if from != "" || to != "" {
		var err error
		if p.From, err = time.ParseInLocation(dateLayout, from, tz); err != nil {
			return Period{}, fmt.Errorf("%w: from deve ser YYYY-MM-DD", ErrInvalidPeriod)
		}
		if p.To, err = time.ParseInLocation(dateLayout, to, tz); err != nil {
			return Period{}, fmt.Errorf("%w: to deve ser YYYY-MM-DD", ErrInvalidPeriod)
		}
		if p.From.After(p.To) {
			return Period{}, fmt.Errorf("%w: from é posterior a to", ErrInvalidPeriod)
		}
		if p.Days() > MaxPeriodDays {
			return Period{}, fmt.Errorf("%w: máximo de %d dias", ErrInvalidPeriod, MaxPeriodDays)
		}
		return p, nil
	}
	days := 7
	switch period {
	case "", "7d":
	case "30d":
		days = 30
	case "90d":
		days = 90
	default:
		return Period{}, fmt.Errorf("%w: use 7d, 30d, 90d ou from/to", ErrInvalidPeriod)
	}
	p.From = today.AddDate(0, 0, -(days - 1))
	return p, nil
}

func dayStart(t time.Time, tz *time.Location) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, tz)
}

// Days devolve a quantidade de dias civis do período.
func (p Period) Days() int {
	a := time.Date(p.From.Year(), p.From.Month(), p.From.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(p.To.Year(), p.To.Month(), p.To.Day(), 0, 0, 0, 0, time.UTC)
	return int(b.Sub(a).Hours()/24) + 1
}

// Start e End delimitam o período como instantes: [Start, End).
func (p Period) Start() time.Time { return p.From.UTC() }
func (p Period) End() time.Time   { return p.To.AddDate(0, 0, 1).UTC() }
