package domain

import (
	"net/url"
	"regexp"
	"strings"
)

var segmentRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,19}$`)
var reserved = map[string]bool{"api": true, "admin": true, "p": true, "static": true, "healthz": true, "beacon": true, "privacidade": true}

func URL(raw string) bool {
	u, e := url.ParseRequestURI(raw)
	return e == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && len(raw) <= 2048
}
func Text(v string, min, max int) bool { n := len(strings.TrimSpace(v)); return n >= min && n <= max }
func Segment(v string) bool            { return segmentRE.MatchString(v) && !reserved[v] }
func PolicyValid(v Policy) bool        { return v == PolicyShorten || v == PolicyDirect }
