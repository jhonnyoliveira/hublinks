package domain

import (
	"errors"
	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("não encontrado")
	ErrConflict = errors.New("conflito")
)

type ValidationError struct{ Fields map[string]string }

func (e ValidationError) Error() string { return "dados inválidos" }

// MarketplaceInUseError preserves the links that must be removed or moved
// before the marketplace can enter the trash.
type MarketplaceInUseError struct{ Links []LinkRef }
type LinkRef struct {
	ID    uuid.UUID `json:"id"`
	Title string    `json:"title"`
}

func (e MarketplaceInUseError) Error() string { return "marketplace possui links ativos" }
