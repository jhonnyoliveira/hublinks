// Package web embeds server-rendered HTML templates.
package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var funcs = template.FuncMap{
	"dict": func(values ...any) map[string]any {
		out := map[string]any{}
		for i := 0; i+1 < len(values); i += 2 {
			if key, ok := values[i].(string); ok {
				out[key] = values[i+1]
			}
		}
		return out
	},
	"list":  func(values ...any) []any { return values },
	"asset": Asset,
	"icon":  Icon,
	"num":   Num,
}

// Num formata um inteiro com separador de milhar em pt-BR (1.234). Aceita int e
// *int; nil vira "—", para contadores de links não rastreáveis nunca virarem 0.
func Num(v any) string {
	var n int
	switch x := v.(type) {
	case int:
		n = x
	case *int:
		if x == nil {
			return "—"
		}
		n = *x
	default:
		return fmt.Sprint(v)
	}
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	if neg {
		s = "-" + s
	}
	return s
}

// fallbackAssets são usados quando o build do CSS ainda não rodou (sem
// manifest.json): o painel segue estilizado, só sem os utilitários do Tailwind.
var fallbackAssets = map[string]string{"app.css": "css/base.css"}

var (
	manifestOnce sync.Once
	manifest     map[string]string
)

// Asset devolve a URL pública de um arquivo estático de nome lógico (por
// exemplo, "app.css"), usando o nome com hash gravado em static/manifest.json
// pelo scripts/build-css.sh.
func Asset(name string) string {
	manifestOnce.Do(func() {
		manifest = map[string]string{}
		if b, err := Files.ReadFile("static/manifest.json"); err == nil {
			_ = json.Unmarshal(b, &manifest)
		}
	})
	if path, ok := manifest[name]; ok {
		return "/static/" + path
	}
	if path, ok := fallbackAssets[name]; ok {
		return "/static/" + path
	}
	return "/static/" + name
}

var iconName = regexp.MustCompile(`^[a-z0-9-]+$`)

// Icon renderiza um ícone Lucide embutido (templates/components/icons/<nome>.svg)
// como SVG decorativo: herda a cor do texto e é oculto de leitores de tela. Quem
// precisa de nome acessível deve rotular o botão ou o link que o contém.
func Icon(name string) (template.HTML, error) {
	if !iconName.MatchString(name) {
		return "", fmt.Errorf("nome de ícone inválido: %q", name)
	}
	inner, err := Files.ReadFile("templates/components/icons/" + name + ".svg")
	if err != nil {
		return "", fmt.Errorf("ícone %q não existe", name)
	}
	return template.HTML(`<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">` + strings.TrimSpace(string(inner)) + `</svg>`), nil //nolint:gosec // conteúdo embutido e versionado
}

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
