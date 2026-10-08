package auth

import "testing"

func TestPassword(t *testing.T) {
	h, err := Hash("senha suficientemente longa")
	if err != nil {
		t.Fatal(err)
	}
	if !Verify("senha suficientemente longa", h) || Verify("errada", h) {
		t.Fatal("verificação incorreta")
	}
}
