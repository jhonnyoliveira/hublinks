package config

import "testing"

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
