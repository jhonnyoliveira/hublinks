package main

import (
	"context"
	"fmt"
	"github.com/hublinks/hublinks/internal/config"
	"github.com/hublinks/hublinks/internal/server"
	"github.com/hublinks/hublinks/internal/store"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata" // fuso de relatório mesmo em imagens sem /usr/share/zoneinfo
)

func main() {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if command != "serve" && command != "migrate" && command != "maintenance" && command != "healthcheck" {
		fmt.Fprintln(os.Stderr, "uso: hublinks [serve|migrate [status]|maintenance run [--day YYYY-MM-DD]|healthcheck]")
		os.Exit(2)
	}
	c, err := config.LoadEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if command == "migrate" {
		pool, err := store.Open(ctx, c.DatabaseURL)
		if err == nil {
			defer pool.Close()
			if len(os.Args) > 2 && os.Args[2] == "status" {
				err = store.Status(ctx, pool)
			} else {
				err = store.Up(ctx, pool)
			}
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if command == "healthcheck" {
		if err := runHealthcheck(ctx, c); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if command == "maintenance" {
		if err := runMaintenance(ctx, c, os.Args[2:], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := server.Run(ctx, c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
