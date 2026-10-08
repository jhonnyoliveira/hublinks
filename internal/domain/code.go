package domain

import (
	"crypto/rand"
	"regexp"
)

var CodeRE = regexp.MustCompile(`^[0-9a-z]{7}$`)

const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

func NewCode() (string, error) {
	b := make([]byte, 7)
	for i := range b {
		for {
			n := make([]byte, 1)
			if _, e := rand.Read(n); e != nil {
				return "", e
			}
			if int(n[0]) < 252 {
				b[i] = alphabet[int(n[0])%36]
				break
			}
		}
	}
	if string(b) == "healthz" {
		return NewCode()
	}
	return string(b), nil
}
