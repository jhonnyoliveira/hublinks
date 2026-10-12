package maintenance

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const tzKey = "report_timezone"

// CheckTimezone aborta se REPORT_TZ difere do fuso já usado nas consolidações.
// Sem consolidação anterior não há nada a conferir: o fuso só é fixado na
// primeira consolidação (pinTimezone).
func (m Manager) CheckTimezone(ctx context.Context) error {
	var stored string
	err := m.Pool.QueryRow(ctx, "SELECT value FROM app_settings WHERE key=$1", tzKey).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return m.compareTimezone(stored)
}

func (m Manager) compareTimezone(stored string) error {
	if stored != m.loc().String() {
		return fmt.Errorf("REPORT_TZ=%s difere do fuso %s já usado nas agregações; restaure REPORT_TZ=%s (mudar o fuso invalidaria os dias já consolidados)", m.loc(), stored, stored)
	}
	return nil
}

// pinTimezone grava o fuso na primeira consolidação e, nas seguintes, confere-o.
func (m Manager) pinTimezone(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, "INSERT INTO app_settings(key,value) VALUES($1,$2) ON CONFLICT (key) DO NOTHING", tzKey, m.loc().String()); err != nil {
		return err
	}
	var stored string
	if err := tx.QueryRow(ctx, "SELECT value FROM app_settings WHERE key=$1", tzKey).Scan(&stored); err != nil {
		return err
	}
	return m.compareTimezone(stored)
}
