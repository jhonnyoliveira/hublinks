// Package web embeds server-rendered HTML templates.
package web

import (
	"embed"
	"html/template"
	"io"
)

// Files contains all templates served by the application.
//
//go:embed templates static
var Files embed.FS

// Render parses a layout and the requested view for one response. Templates are small and this
// keeps template changes safe in development; a cache can be introduced without changing handlers.
func Render(w io.Writer, layout, view string, data any) error {
	t, err := template.New("root").Funcs(template.FuncMap{"dict": func(values ...any) map[string]any {
		out := map[string]any{}
		for i := 0; i+1 < len(values); i += 2 {
			if key, ok := values[i].(string); ok {
				out[key] = values[i+1]
			}
		}
		return out
	}}).ParseFS(Files, "templates/layouts/"+layout+".html", "templates/components/*.html", "templates/"+view+".html")
	if err != nil {
		return err
	}
	return t.ExecuteTemplate(w, layout, data)
}
