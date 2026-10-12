package admin

import (
	"github.com/hublinks/hublinks/internal/service"
	"github.com/hublinks/hublinks/internal/stats"
)

// Catalog reúne as telas do catálogo (marketplaces, canais, links e lixeira).
type Catalog struct {
	Service service.Catalog
	// TrashRetentionDays é o prazo de restauração mostrado na lixeira e nas
	// confirmações de exclusão (TRASH_RETENTION_DAYS).
	TrashRetentionDays int
	// Stats alimenta cliques e únicos nas telas de links. Pool nil desliga as estatísticas.
	Stats stats.Service
}

func (c Catalog) retention() int {
	if c.TrashRetentionDays > 0 {
		return c.TrashRetentionDays
	}
	return 30
}
