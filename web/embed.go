// Package web embeds server-rendered HTML templates.
package web

import (
	"embed"
	"html/template"
	"io"
)

// Files contains all templates served by the application.
//
//go:embed templates
var Files embed.FS

// Render parses a layout and the requested view for one response. Templates are small and this
// keeps template changes safe in development; a cache can be introduced without changing handlers.
func Render(w io.Writer, layout, view string, data any) error {
	t, err := template.ParseFS(Files, "templates/layouts/"+layout+".html", "templates/components/privacy_notice.html", "templates/"+view+".html")
	if err != nil {
		return err
	}
	return t.ExecuteTemplate(w, layout, data)
}
