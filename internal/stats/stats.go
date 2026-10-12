// Package stats consulta os contadores de cliques: dias fechados vêm de
// click_daily e o dia corrente (ou ainda não agregado) de click_events. Robôs
// ficam sempre de fora e toda consulta é restrita a uma organização.
package stats

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	Pool *pgxpool.Pool
	TZ   *time.Location
	Now  func() time.Time // nil = time.Now
}

type Counts struct {
	Clicks int `json:"clicks"`
	Unique int `json:"unique"`
}

// LinkRef identifica o link nas respostas de estatística.
type LinkRef struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Code        string    `json:"code"`
	Marketplace string    `json:"marketplace"`
	Trackable   bool      `json:"trackable"`
}

// LinkStat: para link não rastreável Clicks e Unique são nil (nunca zero) e
// History guarda o que foi contado quando o link ainda era rastreável.
type LinkStat struct {
	Link    LinkRef `json:"link"`
	Clicks  *int    `json:"clicks"`
	Unique  *int    `json:"unique"`
	History *Counts `json:"history,omitempty"`
}

type ChannelRef struct {
	ID      uuid.UUID `json:"id"`
	Segment string    `json:"segment"`
	Name    string    `json:"name"`
}

// ChannelStat: Channel nil é a URL curta sem canal. Unique = SUM(unique_url).
type ChannelStat struct {
	Channel *ChannelRef `json:"channel"`
	Clicks  int         `json:"clicks"`
	Unique  int         `json:"unique"`
}

type DayCount struct {
	Day    string `json:"day"`
	Clicks int    `json:"clicks"`
	Unique int    `json:"unique"`
}

type LinkDetail struct {
	Link      LinkRef       `json:"link"`
	Totals    *Counts       `json:"totals"`
	History   *Counts       `json:"history,omitempty"`
	ByChannel []ChannelStat `json:"by_channel"`
	Daily     []DayCount    `json:"daily"`
}

type Summary struct {
	Clicks      int        `json:"clicks"`
	Unique      int        `json:"unique"`
	ActiveLinks int        `json:"active_links"`
	TopLinks    []LinkStat `json:"top_links"`
	Daily       []DayCount `json:"daily"`
}

type LinkFilter struct {
	MarketplaceID *uuid.UUID
	Q             string
	OnlyTrackable bool
	Page, PerPage int
}

const nilUUID = "00000000-0000-0000-0000-000000000000"

// dailyCTE expõe "daily(day, target_id, channel_id, clicks, unique_url,
// unique_target)" para o período. Parâmetros fixos: $1 org, $2 from, $3 to,
// $4 hoje, $5 fuso, $6 início e $7 fim (instantes). Dias anteriores a hoje
// vêm de click_daily quando agregados; o resto, de click_events.
const dailyCTE = `WITH ev AS (
  SELECT (occurred_at AT TIME ZONE $5::text)::date AS day, target_id,
         COALESCE(channel_id,'` + nilUUID + `'::uuid) AS channel_id,
         count(*) AS clicks,
         count(*) FILTER (WHERE is_unique_url) AS unique_url,
         count(*) FILTER (WHERE is_unique_target) AS unique_target
  FROM click_events
  WHERE org_id=$1 AND NOT is_bot AND occurred_at >= $6 AND occurred_at < $7
  GROUP BY 1,2,3),
daily AS (
  SELECT d.day, d.target_id, d.channel_id, d.clicks::bigint AS clicks, d.unique_url::bigint AS unique_url, d.unique_target::bigint AS unique_target
  FROM click_daily d
  WHERE d.org_id=$1 AND d.day BETWEEN $2::date AND $3::date AND d.day < $4::date
    AND EXISTS (SELECT 1 FROM aggregated_days a WHERE a.org_id=$1 AND a.day=d.day)
  UNION ALL
  SELECT day, target_id, channel_id, clicks, unique_url, unique_target FROM ev
  WHERE day >= $4::date OR NOT EXISTS (SELECT 1 FROM aggregated_days a WHERE a.org_id=$1 AND a.day=ev.day)
) `

const trackableExpr = `COALESCE(l.shorten_policy_override,m.shorten_policy)='shorten'`

// Location devolve REPORT_TZ (UTC quando não configurado).
func (s Service) Location() *time.Location {
	if s.TZ == nil {
		return time.UTC
	}
	return s.TZ
}

// Today devolve o dia corrente em REPORT_TZ.
func (s Service) Today() time.Time {
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	return dayStart(now().In(s.Location()), s.Location())
}

func (s Service) query(ctx context.Context, p Period, org uuid.UUID, tail string, extra ...any) (pgx.Rows, error) {
	args := append([]any{org, p.From.Format(dateLayout), p.To.Format(dateLayout), s.Today().Format(dateLayout), s.Location().String(), p.Start(), p.End()}, extra...)
	return s.Pool.Query(ctx, dailyCTE+tail, args...)
}

func (s Service) Summary(ctx context.Context, orgID uuid.UUID, p Period) (Summary, error) {
	out := Summary{TopLinks: []LinkStat{}}
	days, err := s.daily(ctx, orgID, p, nil)
	if err != nil {
		return out, err
	}
	out.Daily = days
	for _, d := range days {
		out.Clicks += d.Clicks
		out.Unique += d.Unique
	}
	if err = s.Pool.QueryRow(ctx, "SELECT count(*) FROM affiliate_links WHERE org_id=$1 AND active AND deleted_at IS NULL AND purged_at IS NULL", orgID).Scan(&out.ActiveLinks); err != nil {
		return out, err
	}
	top, _, err := s.Links(ctx, orgID, p, LinkFilter{OnlyTrackable: true, Page: 1, PerPage: 5})
	if err != nil {
		return out, err
	}
	for _, l := range top {
		if l.Clicks != nil && *l.Clicks > 0 {
			out.TopLinks = append(out.TopLinks, l)
		}
	}
	return out, nil
}

// daily soma por dia (unique = unique_target) e preenche com zeros os dias sem
// dados. target limita a um link.
func (s Service) daily(ctx context.Context, orgID uuid.UUID, p Period, target *uuid.UUID) ([]DayCount, error) {
	rows, err := s.query(ctx, p, orgID, `SELECT day, sum(clicks)::int, sum(unique_target)::int FROM daily WHERE ($8::uuid IS NULL OR target_id=$8) GROUP BY day`, target)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byDay := map[string]Counts{}
	for rows.Next() {
		var day time.Time
		var c Counts
		if err = rows.Scan(&day, &c.Clicks, &c.Unique); err != nil {
			return nil, err
		}
		byDay[day.Format(dateLayout)] = c
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	out := make([]DayCount, 0, p.Days())
	for d := p.From; !d.After(p.To); d = d.AddDate(0, 0, 1) {
		key := d.Format(dateLayout)
		c := byDay[key]
		out = append(out, DayCount{Day: key, Clicks: c.Clicks, Unique: c.Unique})
	}
	return out, nil
}

func escapeLike(q string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q)
}

// Links lista os links não excluídos com seus contadores no período, do mais
// clicado ao menos. Unique = SUM(unique_target). O total ignora a paginação.
func (s Service) Links(ctx context.Context, orgID uuid.UUID, p Period, f LinkFilter) ([]LinkStat, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 {
		f.PerPage = 20
	}
	q := escapeLike(strings.TrimSpace(f.Q))
	where := ` WHERE l.org_id=$1 AND l.deleted_at IS NULL AND l.purged_at IS NULL
      AND ($8::uuid IS NULL OR l.marketplace_id=$8)
      AND ($9::text='' OR l.title ILIKE '%'||$9||'%' OR sc.code=lower($10::text))
      AND (NOT $11::bool OR ` + trackableExpr + `)`
	from := ` FROM affiliate_links l JOIN marketplaces m ON m.id=l.marketplace_id AND m.org_id=l.org_id
      JOIN short_codes sc ON sc.target_id=l.id AND sc.target_type='affiliate_link'`
	var total int
	if err := s.Pool.QueryRow(ctx, strings.NewReplacer("$8", "$2", "$9", "$3", "$10", "$4", "$11", "$5").Replace("SELECT count(*)"+from+where), orgID, f.MarketplaceID, q, strings.TrimSpace(f.Q), f.OnlyTrackable).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.query(ctx, p, orgID, `SELECT l.id,l.title,sc.code,m.name,`+trackableExpr+`,
      COALESCE(sum(d.clicks),0)::int, COALESCE(sum(d.unique_target),0)::int`+from+`
      LEFT JOIN daily d ON d.target_id=l.id`+where+`
      GROUP BY l.id,l.title,sc.code,m.name,m.shorten_policy
      ORDER BY 6 DESC, l.title, l.id LIMIT $12 OFFSET $13`,
		f.MarketplaceID, q, strings.TrimSpace(f.Q), f.OnlyTrackable, f.PerPage, (f.Page-1)*f.PerPage)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []LinkStat{}
	for rows.Next() {
		var st LinkStat
		var c Counts
		if err = rows.Scan(&st.Link.ID, &st.Link.Title, &st.Link.Code, &st.Link.Marketplace, &st.Link.Trackable, &c.Clicks, &c.Unique); err != nil {
			return nil, 0, err
		}
		st.apply(c)
		items = append(items, st)
	}
	return items, total, rows.Err()
}

// apply aplica a regra de links não rastreáveis: contadores nil e histórico.
func (st *LinkStat) apply(c Counts) {
	if st.Link.Trackable {
		clicks, unique := c.Clicks, c.Unique
		st.Clicks, st.Unique = &clicks, &unique
		return
	}
	if c.Clicks > 0 {
		st.History = &c
	}
}

// Link devolve o detalhe de um link da organização (mesmo excluído, para
// preservar o histórico) ou domain.ErrNotFound.
func (s Service) Link(ctx context.Context, orgID, id uuid.UUID, p Period) (LinkDetail, error) {
	var d LinkDetail
	d.Link.ID = id
	err := s.Pool.QueryRow(ctx, `SELECT l.title,sc.code,m.name,`+trackableExpr+`
      FROM affiliate_links l JOIN marketplaces m ON m.id=l.marketplace_id AND m.org_id=l.org_id
      JOIN short_codes sc ON sc.target_id=l.id AND sc.target_type='affiliate_link'
      WHERE l.id=$1 AND l.org_id=$2`, id, orgID).Scan(&d.Link.Title, &d.Link.Code, &d.Link.Marketplace, &d.Link.Trackable)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, domain.ErrNotFound
	}
	if err != nil {
		return d, err
	}
	d.ByChannel, err = s.byChannel(ctx, orgID, p, &id)
	if err != nil {
		return d, err
	}
	if d.Daily, err = s.daily(ctx, orgID, p, &id); err != nil {
		return d, err
	}
	var total Counts
	for _, day := range d.Daily {
		total.Clicks += day.Clicks
		total.Unique += day.Unique
	}
	if d.Link.Trackable {
		d.Totals = &total
	} else if total.Clicks > 0 {
		d.History = &total
	}
	return d, nil
}

func (s Service) byChannel(ctx context.Context, orgID uuid.UUID, p Period, target *uuid.UUID) ([]ChannelStat, error) {
	rows, err := s.query(ctx, p, orgID, `SELECT d.channel_id, c.name, c.segment, sum(d.clicks)::int, sum(d.unique_url)::int
      FROM daily d LEFT JOIN channels c ON c.id=d.channel_id AND c.org_id=$1
      WHERE ($8::uuid IS NULL OR d.target_id=$8)
      GROUP BY d.channel_id, c.name, c.segment ORDER BY 4 DESC, 3`, target)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ChannelStat{}
	for rows.Next() {
		var id uuid.UUID
		var name, segment *string
		var st ChannelStat
		if err = rows.Scan(&id, &name, &segment, &st.Clicks, &st.Unique); err != nil {
			return nil, err
		}
		if id != uuid.Nil {
			ref := ChannelRef{ID: id}
			if name != nil {
				ref.Name, ref.Segment = *name, *segment
			}
			st.Channel = &ref
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// Channels soma o período por canal (Unique = SUM(unique_url)), incluindo a URL
// curta sem canal e os canais ativos ainda sem cliques.
func (s Service) Channels(ctx context.Context, orgID uuid.UUID, p Period) ([]ChannelStat, error) {
	out, err := s.byChannel(ctx, orgID, p, nil)
	if err != nil {
		return nil, err
	}
	seen := map[uuid.UUID]bool{}
	for _, c := range out {
		if c.Channel != nil {
			seen[c.Channel.ID] = true
		}
	}
	rows, err := s.Pool.Query(ctx, "SELECT id,name,segment FROM channels WHERE org_id=$1 AND deleted_at IS NULL AND purged_at IS NULL ORDER BY name", orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ref ChannelRef
		if err = rows.Scan(&ref.ID, &ref.Name, &ref.Segment); err != nil {
			return nil, err
		}
		if !seen[ref.ID] {
			r := ref
			out = append(out, ChannelStat{Channel: &r})
		}
	}
	return out, rows.Err()
}

// Totals devolve cliques e únicos (unique_target) de cada link informado no
// período. Links sem dados ficam ausentes do mapa.
func (s Service) Totals(ctx context.Context, orgID uuid.UUID, p Period, ids []uuid.UUID) (map[uuid.UUID]Counts, error) {
	out := map[uuid.UUID]Counts{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.query(ctx, p, orgID, `SELECT target_id, sum(clicks)::int, sum(unique_target)::int FROM daily WHERE target_id = ANY($8::uuid[]) GROUP BY target_id`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var c Counts
		if err = rows.Scan(&id, &c.Clicks, &c.Unique); err != nil {
			return nil, err
		}
		out[id] = c
	}
	return out, rows.Err()
}
