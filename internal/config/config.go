// Package config loads the application's environment based configuration.
package config

import (
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL, Pepper, BaseURL, HTTPAddr, MetricsAddr, AppEnv, OrgName, PrivacyContact, LogLevel                                                       string
	TrustedProxies                                                                                                                                       []netip.Prefix
	AdminEmail, AdminPassword                                                                                                                            string
	ReportTZ                                                                                                                                             *time.Location
	EventsRetentionDays, BotEventsRetentionDays, TrashRetentionDays, BotDistinctCodes, PublicRateLimit, LoginMaxFailures, EventQueueSize, EventBatchSize int
	BotWindow, LoginWindow, LoginLockout, EventFlushInterval                                                                                             time.Duration
	MaintenanceAt                                                                                                                                        string
}

func Load(getenv func(string) string) (Config, error) {
	c := Config{HTTPAddr: value(getenv, "HTTP_ADDR", ":8080"), MetricsAddr: value(getenv, "METRICS_ADDR", ":9091"), AppEnv: value(getenv, "APP_ENV", "production"), OrgName: value(getenv, "ORG_NAME", "Minha organização"), LogLevel: value(getenv, "LOG_LEVEL", "info"), EventsRetentionDays: 395, BotEventsRetentionDays: 30, TrashRetentionDays: 30, BotDistinctCodes: 5, PublicRateLimit: 300, LoginMaxFailures: 5, EventQueueSize: 10000, EventBatchSize: 500, MaintenanceAt: value(getenv, "MAINTENANCE_AT", "00:30")}
	for _, pair := range []struct {
		name string
		dst  *string
	}{{"DATABASE_URL", &c.DatabaseURL}, {"PEPPER", &c.Pepper}, {"BASE_URL", &c.BaseURL}, {"PRIVACY_CONTACT", &c.PrivacyContact}} {
		*pair.dst = strings.TrimSpace(getenv(pair.name))
		if *pair.dst == "" {
			return c, fmt.Errorf("%s é obrigatória", pair.name)
		}
	}
	if len(c.Pepper) < 32 {
		return c, fmt.Errorf("PEPPER deve ter ao menos 32 bytes")
	}
	u, err := url.ParseRequestURI(c.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return c, fmt.Errorf("BASE_URL inválida")
	}
	c.AdminEmail, c.AdminPassword = strings.TrimSpace(getenv("ADMIN_EMAIL")), getenv("ADMIN_PASSWORD")
	var tz = value(getenv, "REPORT_TZ", "America/Sao_Paulo")
	c.ReportTZ, err = time.LoadLocation(tz)
	if err != nil {
		return c, fmt.Errorf("REPORT_TZ: %w", err)
	}
	if c.BotWindow, err = time.ParseDuration(value(getenv, "BOT_WINDOW", "10s")); err != nil {
		return c, fmt.Errorf("BOT_WINDOW: %w", err)
	}
	if c.LoginWindow, err = time.ParseDuration(value(getenv, "LOGIN_WINDOW", "15m")); err != nil {
		return c, fmt.Errorf("LOGIN_WINDOW: %w", err)
	}
	if c.LoginLockout, err = time.ParseDuration(value(getenv, "LOGIN_LOCKOUT", "15m")); err != nil {
		return c, fmt.Errorf("LOGIN_LOCKOUT: %w", err)
	}
	if c.EventFlushInterval, err = time.ParseDuration(value(getenv, "EVENT_FLUSH_INTERVAL", "1s")); err != nil {
		return c, fmt.Errorf("EVENT_FLUSH_INTERVAL: %w", err)
	}
	for _, item := range []struct {
		name string
		dst  *int
	}{{"EVENTS_RETENTION_DAYS", &c.EventsRetentionDays}, {"BOT_EVENTS_RETENTION_DAYS", &c.BotEventsRetentionDays}, {"TRASH_RETENTION_DAYS", &c.TrashRetentionDays}, {"BOT_DISTINCT_CODES", &c.BotDistinctCodes}, {"PUBLIC_RATE_LIMIT", &c.PublicRateLimit}, {"LOGIN_MAX_FAILURES", &c.LoginMaxFailures}, {"EVENT_QUEUE_SIZE", &c.EventQueueSize}, {"EVENT_BATCH_SIZE", &c.EventBatchSize}} {
		if err := positive(getenv(item.name), item.dst); err != nil {
			return c, fmt.Errorf("%s: %w", item.name, err)
		}
	}
	for _, raw := range strings.Split(strings.TrimSpace(getenv("TRUSTED_PROXIES")), ",") {
		if raw == "" {
			continue
		}
		p, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			return c, fmt.Errorf("TRUSTED_PROXIES: %w", err)
		}
		c.TrustedProxies = append(c.TrustedProxies, p)
	}
	if !map[string]bool{"debug": true, "info": true, "warn": true, "error": true}[c.LogLevel] {
		return c, fmt.Errorf("LOG_LEVEL inválido")
	}
	return c, nil
}
func value(g func(string) string, key, fallback string) string {
	if v := strings.TrimSpace(g(key)); v != "" {
		return v
	}
	return fallback
}
func positive(raw string, dst *int) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fmt.Errorf("deve ser inteiro positivo")
	}
	*dst = n
	return nil
}
