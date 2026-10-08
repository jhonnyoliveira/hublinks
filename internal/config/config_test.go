package config

import (
	"strings"
	"testing"
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
