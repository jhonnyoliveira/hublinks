package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestLoadRequiredAndDefaults(t *testing.T) {
	env := map[string]string{"DATABASE_URL": "postgres://x", "PEPPER": "12345678901234567890123456789012", "BASE_URL": "http://localhost:8080", "PRIVACY_CONTACT": "x@example.com"}
	c, err := Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":8080" || c.EventsRetentionDays != 395 || c.ReportTZ.String() != "America/Sao_Paulo" {
		t.Fatalf("defaults: %#v", c)
	}
	for _, key := range []string{"DATABASE_URL", "PEPPER", "BASE_URL", "PRIVACY_CONTACT"} {
		copy := map[string]string{}
		for k, v := range env {
			copy[k] = v
		}
		delete(copy, key)
		if _, err := Load(func(k string) string { return copy[k] }); err == nil {
			t.Fatalf("wanted error for %s", key)
		}
	}
}

func TestLoadParsesConfiguredValues(t *testing.T) {
	env := map[string]string{
		"DATABASE_URL": "postgres://x", "PEPPER": "12345678901234567890123456789012", "BASE_URL": "https://hub.example", "PRIVACY_CONTACT": "x@example.com",
		"BOT_WINDOW": "20s", "LOGIN_WINDOW": "9m", "REPORT_TZ": "UTC", "TRUSTED_PROXIES": "10.0.0.0/8,2001:db8::/32", "EVENT_QUEUE_SIZE": "7",
	}
	c, err := Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if c.BotWindow.String() != "20s" || c.LoginWindow.String() != "9m0s" || c.ReportTZ.String() != "UTC" || len(c.TrustedProxies) != 2 || c.EventQueueSize != 7 {
		t.Fatalf("parse incorreto: %#v", c)
	}
	env["PEPPER"] = "curto"
	if _, err := Load(func(k string) string { return env[k] }); err == nil || !strings.Contains(err.Error(), "PEPPER") {
		t.Fatalf("PEPPER curto aceito: %v", err)
	}
	env["PEPPER"] = "12345678901234567890123456789012"
	env["TRUSTED_PROXIES"] = "inválido"
	if _, err := Load(func(k string) string { return env[k] }); err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXIES") {
		t.Fatalf("CIDR inválido aceito: %v", err)
	}
}

func baseEnv() map[string]string {
	return map[string]string{"DATABASE_URL": "postgres://u:senha-do-banco@h/db", "PEPPER": "12345678901234567890123456789012", "BASE_URL": "http://localhost:8080", "PRIVACY_CONTACT": "x@example.com"}
}

func load(env map[string]string) (Config, error) {
	return Load(func(k string) string { return env[k] })
}

func TestMissingRequiredVariableIsNamedInTheError(t *testing.T) {
	for _, key := range []string{"DATABASE_URL", "PEPPER", "BASE_URL", "PRIVACY_CONTACT"} {
		env := baseEnv()
		delete(env, key)
		if _, err := load(env); err == nil || !strings.Contains(err.Error(), key) {
			t.Fatalf("o erro deveria citar %s: %v", key, err)
		}
	}
}

func TestEveryDefaultFromTheContractIsApplied(t *testing.T) {
	c, err := load(baseEnv())
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]bool{
		"HTTP_ADDR":                 c.HTTPAddr == ":8080",
		"METRICS_ADDR":              c.MetricsAddr == ":9091",
		"APP_ENV":                   c.AppEnv == "production",
		"TRUSTED_PROXIES":           len(c.TrustedProxies) == 0,
		"ORG_NAME":                  c.OrgName == "Minha organização",
		"REPORT_TZ":                 c.ReportTZ.String() == "America/Sao_Paulo",
		"EVENTS_RETENTION_DAYS":     c.EventsRetentionDays == 395,
		"BOT_EVENTS_RETENTION_DAYS": c.BotEventsRetentionDays == 30,
		"TRASH_RETENTION_DAYS":      c.TrashRetentionDays == 30,
		"BOT_DISTINCT_CODES":        c.BotDistinctCodes == 5,
		"BOT_WINDOW":                c.BotWindow == 10*time.Second,
		"PUBLIC_RATE_LIMIT":         c.PublicRateLimit == 300,
		"LOGIN_MAX_FAILURES":        c.LoginMaxFailures == 5,
		"LOGIN_WINDOW":              c.LoginWindow == 15*time.Minute,
		"LOGIN_LOCKOUT":             c.LoginLockout == 15*time.Minute,
		"EVENT_QUEUE_SIZE":          c.EventQueueSize == 10000,
		"EVENT_BATCH_SIZE":          c.EventBatchSize == 500,
		"EVENT_FLUSH_INTERVAL":      c.EventFlushInterval == time.Second,
		"MAINTENANCE_AT":            c.MaintenanceAt == "00:30",
		"LOG_LEVEL":                 c.LogLevel == "info",
	}
	for name, ok := range checks {
		if !ok {
			t.Errorf("padrão incorreto para %s: %s", name, c)
		}
	}
}

func TestInvalidValuesAbortNamingTheVariable(t *testing.T) {
	cases := map[string]string{
		"BASE_URL": "ftp://x", "REPORT_TZ": "Marte/Olympus", "BOT_WINDOW": "dez", "LOGIN_WINDOW": "x", "LOGIN_LOCKOUT": "x",
		"EVENT_FLUSH_INTERVAL": "x", "EVENTS_RETENTION_DAYS": "-1", "BOT_EVENTS_RETENTION_DAYS": "0", "TRASH_RETENTION_DAYS": "abc",
		"BOT_DISTINCT_CODES": "0", "PUBLIC_RATE_LIMIT": "x", "LOGIN_MAX_FAILURES": "-5", "EVENT_QUEUE_SIZE": "x", "EVENT_BATCH_SIZE": "0",
		"MAINTENANCE_AT": "25:99", "LOG_LEVEL": "verbose", "TRUSTED_PROXIES": "10.0.0.1",
	}
	for key, bad := range cases {
		env := baseEnv()
		env[key] = bad
		if _, err := load(env); err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("%s=%q deveria abortar citando a variável: %v", key, bad, err)
		}
	}
}

func TestConfigNeverPrintsSecrets(t *testing.T) {
	env := baseEnv()
	env["ADMIN_PASSWORD"] = "senha-do-admin-123"
	env["PEPPER"] = "pepper-secreto-pepper-secreto-123456"
	c, err := load(env)
	if err != nil {
		t.Fatal(err)
	}
	var logged bytes.Buffer
	slog.New(slog.NewJSONHandler(&logged, nil)).Info("config", "config", c)
	outputs := []string{c.String(), fmt.Sprintf("%v", c), fmt.Sprintf("%+v", c), fmt.Sprintf("%#v", c), fmt.Sprintf("%s", c), logged.String()}
	for _, out := range outputs {
		for _, secret := range []string{"senha-do-banco", "senha-do-admin-123", "pepper-secreto"} {
			if strings.Contains(out, secret) {
				t.Fatalf("segredo %q vazou: %s", secret, out)
			}
		}
		if !strings.Contains(out, "REDACTED") {
			t.Fatalf("a saída deveria indicar a omissão: %s", out)
		}
	}
	if !strings.Contains(c.String(), "localhost:8080") {
		t.Fatal("valores não secretos devem aparecer")
	}
}
