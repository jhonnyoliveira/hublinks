package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
)

type Catalog struct {
	Pool    *pgxpool.Pool
	newCode func() (string, error)
}

// ListOptions define filtros comuns às listagens administrativas. Page starts at
// one; callers that do not need pagination may leave it zero.
type ListOptions struct {
	Query         string
	Trash         bool
	MarketplaceID uuid.UUID
	Active        *bool
	// Sort é "campo" ou "-campo" (descendente); campos fora da lista permitida
	// de cada listagem são ignorados e valem a ordem padrão.
	Sort          string
	Page, PerPage int
}

func NewCatalog(p *pgxpool.Pool) *Catalog { return &Catalog{Pool: p, newCode: domain.NewCode} }

// NewCatalogWithCodeGenerator is used by integration tests to make a database
// collision deterministic. Application code must use NewCatalog.
func NewCatalogWithCodeGenerator(p *pgxpool.Pool, generator func() (string, error)) *Catalog {
	return &Catalog{Pool: p, newCode: generator}
}

func (c *Catalog) count(ctx context.Context, table, where string, args []any) (int, error) {
	var total int
	err := c.Pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE "+where, args...).Scan(&total)
	return total, err
}

func paged(q string, args []any, o ListOptions) (string, []any) {
	if o.Page < 1 {
		return q, args
	}
	per := o.PerPage
	if per < 1 {
		per = 20
	}
	if per > 100 {
		per = 100
	}
	args = append(args, per, (o.Page-1)*per)
	return fmt.Sprintf("%s LIMIT $%d OFFSET $%d", q, len(args)-1, len(args)), args
}

// uniqueConflict converts a unique-constraint violation (SQLSTATE 23505) into
// domain.ErrConflict; any other error is returned unchanged.
func uniqueConflict(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return fmt.Errorf("%w: valor já em uso", domain.ErrConflict)
	}
	return err
}

// likePattern monta o padrão ILIKE de busca por trecho, escapando os curingas
// que o usuário possa ter digitado.
func likePattern(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(q) + "%"
}

// orderBy traduz o Sort pedido para uma cláusula ORDER BY usando somente as
// colunas da lista permitida (nunca texto vindo do usuário).
func orderBy(sort string, allowed map[string]string, def, tie string) string {
	dir := " ASC"
	if strings.HasPrefix(sort, "-") {
		dir, sort = " DESC", strings.TrimPrefix(sort, "-")
	}
	col, ok := allowed[sort]
	if !ok {
		return def
	}
	return "ORDER BY " + col + dir + ", " + tie
}
