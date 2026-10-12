package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/hublinks/hublinks/internal/config"
	"github.com/hublinks/hublinks/internal/maintenance"
	"github.com/hublinks/hublinks/internal/store"
)

// runMaintenance executa `hublinks maintenance run [--day YYYY-MM-DD]`: consolida,
// limpa e expira a lixeira uma vez. Sem --day, consolida todos os dias passados
// pendentes; com --day, reconsolida aquele dia.
func runMaintenance(ctx context.Context, c config.Config, args []string, out io.Writer) error {
	if len(args) == 0 || args[0] != "run" {
		return errors.New("uso: hublinks maintenance run [--day YYYY-MM-DD]")
	}
	fs := flag.NewFlagSet("maintenance run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dayFlag := fs.String("day", "", "reconsolida o dia informado (YYYY-MM-DD, em REPORT_TZ)")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() > 0 {
		return errors.New("uso: hublinks maintenance run [--day YYYY-MM-DD]")
	}
	m := maintenance.Manager{Config: c}
	var day *time.Time
	if *dayFlag != "" {
		parsed, err := m.ParseDay(*dayFlag)
		if err != nil {
			return err
		}
		day = &parsed
	}
	pool, err := store.Open(ctx, c.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	m.Pool = pool
	report, ran, err := m.RunExclusive(ctx, day)
	if err != nil {
		return err
	}
	if !ran {
		return errors.New("outra manutenção já está em andamento; tente novamente em instantes")
	}
	fmt.Fprintf(out, "Manutenção concluída.\n  dias consolidados (organização × dia): %d\n  eventos removidos pela retenção: %d\n  exclusões definitivas: %d links, %d marketplaces, %d canais\n",
		report.AggregatedDays, report.DeletedEvents, report.Purged.Links, report.Purged.Marketplaces, report.Purged.Channels)
	return nil
}
