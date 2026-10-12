package admin

import (
	"encoding/json"
	"html/template"
	"net/url"
	"strings"
	"time"

	"github.com/hublinks/hublinks/internal/stats"
)

// periodItem é um atalho do seletor de período (componente period_picker).
type periodItem struct {
	Href, Text string
	Current    bool
}

type hiddenField struct{ Name, Value string }

// periodView é o seletor de período de uma tela. Target, quando preenchido,
// faz o HTMX trocar só aquele contêiner e atualizar a URL (hx-push-url).
type periodView struct {
	Items  []periodItem
	Target string
	// Hidden repete o período atual em formulários de filtro da mesma tela.
	Hidden []hiddenField
	Custom bool
	From   string // AAAA-MM-DD (campos ocultos e URLs)
	To     string
	// FromBR e ToBR são os mesmos dias em dd/mm/aaaa, para os campos de texto.
	FromBR string
	ToBR   string
	Label  string
	Error  string
}

var presets = []struct{ Key, Text string }{{"7d", "7 dias"}, {"30d", "30 dias"}, {"90d", "90 dias"}}

// resolvePeriod lê period/from/to da query. Um período inválido cai no padrão
// (7 dias) e devolve a mensagem para a tela exibir.
func resolvePeriod(svc stats.Service, q url.Values, base, target string) (stats.Period, periodView) {
	now := svc.Today()
	period, from, to := q.Get("period"), normalizeDay(q.Get("from")), normalizeDay(q.Get("to"))
	p, err := stats.ParsePeriod(period, from, to, now, svc.Location())
	v := periodView{Target: target}
	if err != nil {
		v.Error = "Período inválido; mostrando os últimos 7 dias."
		period, from, to = "", "", ""
		p, _ = stats.ParsePeriod("", "", "", now, svc.Location())
	}
	v.Custom = from != "" || to != ""
	current := period
	if current == "" && !v.Custom {
		current = "7d"
	}
	for _, pr := range presets {
		keep := url.Values{}
		for k, vals := range q {
			if k != "page" && k != "from" && k != "to" && k != "period" {
				keep[k] = vals
			}
		}
		keep.Set("period", pr.Key)
		v.Items = append(v.Items, periodItem{Href: base + "?" + keep.Encode(), Text: pr.Text, Current: pr.Key == current})
	}
	if v.Custom {
		v.From, v.To = p.From.Format("2006-01-02"), p.To.Format("2006-01-02")
		v.FromBR, v.ToBR = p.From.Format("02/01/2006"), p.To.Format("02/01/2006")
		v.Hidden = []hiddenField{{"from", v.From}, {"to", v.To}}
	} else {
		v.Hidden = []hiddenField{{"period", current}}
	}
	v.Label = p.From.Format("02/01/2006") + " a " + p.To.Format("02/01/2006")
	return p, v
}

// normalizeDay converte dd/mm/aaaa (campos do painel) para AAAA-MM-DD. Outros
// valores, inclusive o ISO da API e das URLs antigas, seguem inalterados.
func normalizeDay(v string) string {
	if t, err := time.Parse("02/01/2006", strings.TrimSpace(v)); err == nil {
		return t.Format("2006-01-02")
	}
	return v
}

// chartData serializa a série diária para o script do gráfico (chart.js).
func chartData(days []stats.DayCount) template.JS {
	type series struct {
		Days   []string `json:"days"`
		Clicks []int    `json:"clicks"`
		Unique []int    `json:"unique"`
	}
	s := series{Days: []string{}, Clicks: []int{}, Unique: []int{}}
	for _, d := range days {
		s.Days = append(s.Days, d.Day)
		s.Clicks = append(s.Clicks, d.Clicks)
		s.Unique = append(s.Unique, d.Unique)
	}
	b, _ := json.Marshal(s) // json.Marshal escapa <, > e & (seguro em <script>)
	return template.JS(b)   //nolint:gosec // gerado aqui a partir de inteiros e datas
}

// dayRows formata a série como linhas da tabela alternativa ao gráfico.
func dayRows(days []stats.DayCount) [][]string {
	rows := make([][]string, 0, len(days))
	for _, d := range days {
		t, _ := time.Parse("2006-01-02", d.Day)
		rows = append(rows, []string{t.Format("02/01/2006"), webNum(d.Clicks), webNum(d.Unique)})
	}
	return rows
}
