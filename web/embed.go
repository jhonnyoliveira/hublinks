// Package web embeds server-rendered HTML templates.
package web

import (
	"embed"
	"html/template"
	"io"
)

var funcs = template.FuncMap{"dict": func(values ...any) map[string]any {
	out := map[string]any{}
	for i := 0; i+1 < len(values); i += 2 {
		if key, ok := values[i].(string); ok {
			out[key] = values[i+1]
		}
	}
	return out
}}

// Files contains all templates served by the application.
//
//go:embed templates static
var Files embed.FS

// Render parses a layout and the requested view for one response. Templates are small and this
// keeps template changes safe in development; a cache can be introduced without changing handlers.
func Render(w io.Writer, layout, view string, data any) error {
	t, err := template.New("root").Funcs(funcs).ParseFS(Files, "templates/layouts/"+layout+".html", "templates/components/*.html", "templates/"+view+".html")
	if err != nil {
		return err
	}
	return t.ExecuteTemplate(w, layout, data)
}

// RenderDir renderiza o template name entre os definidos em templates/<dir>/*.html
// e nos componentes. Com layout vazio, serve para fragmentos HTMX; com layout
// informado, name é o conteúdo de uma página completa e o layout o envolve.
func RenderDir(w io.Writer, layout, dir, name string, data any) error {
	patterns := []string{"templates/components/*.html", "templates/" + dir + "/*.html"}
	if layout != "" {
		patterns = append([]string{"templates/layouts/" + layout + ".html"}, patterns...)
	}
	t, err := template.New("root").Funcs(funcs).ParseFS(Files, patterns...)
	if err != nil {
		return err
	}
	if layout != "" {
		return t.ExecuteTemplate(w, layout, data)
	}
	return t.ExecuteTemplate(w, name, data)
}

// RenderPage renderiza uma página completa com o layout, os componentes e
// somente os arquivos informados (caminhos sob templates/). Serve para telas
// cujos arquivos definem cada um o seu "head" e o seu "content".
func RenderPage(w io.Writer, layout string, data any, files ...string) error {
	patterns := []string{"templates/layouts/" + layout + ".html", "templates/components/*.html"}
	for _, f := range files {
		patterns = append(patterns, "templates/"+f)
	}
	t, err := template.New("root").Funcs(funcs).ParseFS(Files, patterns...)
	if err != nil {
		return err
	}
	return t.ExecuteTemplate(w, layout, data)
}

// RenderFragment executa o template name, definido nos arquivos informados
// (caminhos sob templates/) ou nos componentes, sem layout. Serve para
// fragmentos HTMX de páginas que vivem num diretório compartilhado.
func RenderFragment(w io.Writer, name string, data any, files ...string) error {
	patterns := []string{"templates/components/*.html"}
	for _, f := range files {
		patterns = append(patterns, "templates/"+f)
	}
	t, err := template.New("root").Funcs(funcs).ParseFS(Files, patterns...)
	if err != nil {
		return err
	}
	return t.ExecuteTemplate(w, name, data)
}
