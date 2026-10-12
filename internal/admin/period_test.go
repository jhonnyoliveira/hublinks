package admin

import "testing"

func TestNormalizeDay(t *testing.T) {
	for in, want := range map[string]string{"12/10/2026": "2026-10-12", " 01/02/2026 ": "2026-02-01", "2026-10-12": "2026-10-12", "": "", "10/12": "10/12"} {
		if got := normalizeDay(in); got != want {
			t.Errorf("normalizeDay(%q) = %q, quero %q", in, got, want)
		}
	}
}
