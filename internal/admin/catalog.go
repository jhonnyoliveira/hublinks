package admin

import "github.com/hublinks/hublinks/internal/service"

// Catalog reúne as telas do catálogo (marketplaces, canais, links e lixeira).
type Catalog struct {
	Service service.Catalog
	// TrashRetentionDays é o prazo de restauração mostrado na lixeira e nas
	// confirmações de exclusão (TRASH_RETENTION_DAYS).
	TrashRetentionDays int
}

func (c Catalog) retention() int {
	if c.TrashRetentionDays > 0 {
		return c.TrashRetentionDays
	}
	return 30
}
