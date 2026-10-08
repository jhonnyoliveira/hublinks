package web

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func frag(t *testing.T, name string, data any) string {
	t.Helper()
	var b bytes.Buffer
	if err := RenderFragment(&b, name, data); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return b.String()
}

func has(t *testing.T, got string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(got, p) {
			t.Errorf("faltou %q em:\n%s", p, got)
		}
	}
}

func hasNot(t *testing.T, got string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if strings.Contains(got, p) {
			t.Errorf("não deveria conter %q em:\n%s", p, got)
		}
	}
}

func d(kv ...any) map[string]any { return funcs["dict"].(func(...any) map[string]any)(kv...) }

func TestButtonComponent(t *testing.T) {
	has(t, frag(t, "button", d("Label", "Salvar", "Type", "submit", "Variant", "primary", "Icon", "check")), `type="submit"`, `class="btn btn-primary"`, "<span>Salvar</span>", "<svg", `aria-hidden="true"`)
	// ícone sem texto precisa de nome acessível
	icon := frag(t, "button", d("Icon", "copy", "AriaLabel", "Copiar URL", "Small", true))
	has(t, icon, `aria-label="Copiar URL"`, "btn-icon", "btn-sm", `type="button"`)
	has(t, frag(t, "button", d("Label", "Nope", "Disabled", true)), " disabled")
	hasNot(t, frag(t, "button", d("Label", `<b>x</b>`)), "<b>x</b>")
}

func TestBadgeCardTableComponents(t *testing.T) {
	has(t, frag(t, "badge", d("Text", "Ativo", "Tone", "ok")), `class="badge badge-ok"`, "Ativo")
	has(t, frag(t, "badge", d("Text", "x")), `class="badge"`)
	card := frag(t, "card", d("Title", "Título", "Text", "Corpo", "Href", "/admin", "Action", "Abrir"))
	has(t, card, "<h2", "Título", "Corpo", `href="/admin"`, "Abrir")
	table := frag(t, "table", d("Caption", "Cap", "Columns", []string{"A", "B"}, "Rows", [][]string{{"1", "<i>2</i>"}}, "EmptyText", "Vazio"))
	has(t, table, `<caption class="sr-only">Cap</caption>`, `<th scope="col">A</th>`, "<td>1</td>", "&lt;i&gt;2&lt;/i&gt;")
	hasNot(t, table, "<i>2</i>", "Vazio")
	empty := frag(t, "table", d("Caption", "Cap", "Columns", []string{"A", "B"}, "Rows", [][]string{}, "EmptyText", "Vazio"))
	has(t, empty, `colspan="2"`, "Vazio")
}

func TestModalIsAnAccessibleDialogShell(t *testing.T) {
	has(t, frag(t, "modal", nil), "<dialog", `id="modal"`, `role="dialog"`, `aria-modal="true"`, `aria-labelledby="modal-title"`, `id="modal-body"`)
}

func TestToastRegionIsALiveRegion(t *testing.T) {
	has(t, frag(t, "toast_region", nil), `aria-live="polite"`, `role="status"`, `@toast.window`)
	has(t, frag(t, "toast", "Salvo"), `role="status"`, "Salvo")
}

func TestNavigationComponents(t *testing.T) {
	type item struct {
		Href, Text string
		Current    bool
	}
	tabs := frag(t, "tabs", d("Label", "Seções", "Items", []item{{"/a", "A", true}, {"/b", "B", false}}))
	has(t, tabs, `aria-label="Seções"`, `href="/a" aria-current="page"`, `href="/b">B`)
	hasNot(t, tabs, `href="/b" aria-current`)
	period := frag(t, "period_picker", d("Items", []item{{"?period=7d", "7 dias", false}, {"?period=30d", "30 dias", true}}))
	has(t, period, `aria-label="Período"`, `aria-current="true"`, "30 dias")
}

func TestStateComponents(t *testing.T) {
	has(t, frag(t, "empty_state", d("Text", "Nada aqui", "Icon", "inbox", "ActionHref", "/novo", "ActionLabel", "Criar")), "Nada aqui", "<svg", `href="/novo"`, "Criar")
	hasNot(t, frag(t, "empty_state", d("Text", "Só texto")), "<a ", "<svg")
	has(t, frag(t, "skeleton", nil), `role="status"`, `aria-busy="true"`, "Carregando")
}

func TestFieldComponentWiresErrorAndHintToTheInput(t *testing.T) {
	both := frag(t, "field", d("ID", "x", "Name", "x", "Label", "Nome", "Required", true, "Error", "inválido", "Hint", "dica", "Value", `a"b`))
	has(t, both, `for="x"`, `id="x"`, " required", `aria-invalid="true"`, `aria-describedby="x-error x-hint"`, `id="x-error"`, `role="alert"`, `id="x-hint"`)
	hasNot(t, both, `value="a"b"`)
	clean := frag(t, "field", d("ID", "y", "Name", "y", "Label", "Y"))
	hasNot(t, clean, "aria-invalid", "aria-describedby", "required")
}

func TestPrivacyNoticeIsInformativeWithoutCookies(t *testing.T) {
	n := frag(t, "privacy_notice", nil)
	has(t, n, `href="/privacidade"`, "Entendi", "localStorage", `aria-label="Aviso de privacidade"`)
	hasNot(t, n, "document.cookie", "consent", "Aceitar")
	// o armazenamento pode estar bloqueado: o script precisa tolerar a falha
	if strings.Count(n, "try{") < 2 {
		t.Error("localStorage deve estar protegido por try/catch")
	}
}

func TestThemeInitRunsBeforePaintAndRespectsSystem(t *testing.T) {
	s := frag(t, "theme_init", nil)
	has(t, s, "<script>", `localStorage.getItem("theme")`, "prefers-color-scheme: dark", `classList.add(d?"dark":"light")`)
}

var iconTags = regexp.MustCompile(`<(/?)(path|line|circle|rect|polyline|polygon|ellipse)\b`)

func TestIconsAreWellFormedAndRenderDecoratively(t *testing.T) {
	entries, err := Files.ReadDir("templates/components/icons")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".svg") {
			continue
		}
		n++
		raw, _ := Files.ReadFile("templates/components/icons/" + e.Name())
		rest := iconTags.ReplaceAllString(string(raw), "")
		if strings.Contains(rest, "<script") || strings.Contains(rest, "on") && regexp.MustCompile(`\son\w+=`).MatchString(rest) {
			t.Errorf("%s: conteúdo inesperado", e.Name())
		}
		name := strings.TrimSuffix(e.Name(), ".svg")
		out, err := Icon(name)
		if err != nil || !strings.Contains(string(out), `aria-hidden="true"`) || !strings.Contains(string(out), `class="icon"`) || strings.Contains(regexp.MustCompile(`^<svg[^>]*>`).FindString(string(out)), " width=") {
			t.Errorf("%s: %v %s", name, err, out)
		}
	}
	if n < 20 {
		t.Fatalf("poucos ícones: %d", n)
	}
	for _, bad := range []string{"", "../x", "Plus", "nao-existe", "a/b"} {
		if _, err := Icon(bad); err == nil {
			t.Errorf("Icon(%q) deveria falhar", bad)
		}
	}
	if _, err := Files.ReadFile("templates/components/icons/LICENSE-lucide.txt"); err != nil {
		t.Error("a licença dos ícones deve acompanhá-los")
	}
}

func TestEveryIconUsedInTemplatesExists(t *testing.T) {
	used := regexp.MustCompile(`icon "([a-z0-9-]+)"`)
	var missing []string
	for _, dir := range []string{"templates/layouts", "templates/components", "templates/admin", "templates/admin/links", "templates/admin/channels", "templates/admin/marketplaces", "templates/public"} {
		es, _ := Files.ReadDir(dir)
		for _, e := range es {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
				continue
			}
			b, _ := Files.ReadFile(dir + "/" + e.Name())
			for _, m := range used.FindAllStringSubmatch(string(b), -1) {
				if _, err := Files.ReadFile("templates/components/icons/" + m[1] + ".svg"); err != nil {
					missing = append(missing, dir+"/"+e.Name()+": "+m[1])
				}
			}
		}
	}
	if len(missing) > 0 {
		t.Fatalf("ícones inexistentes: %v", missing)
	}
}

func TestAssetResolvesHashedNameOrFallsBack(t *testing.T) {
	got := Asset("app.css")
	if !regexp.MustCompile(`^/static/css/(base|app\.[0-9a-f]{8})\.css$`).MatchString(got) {
		t.Fatalf("Asset(app.css) = %q", got)
	}
	if _, err := Files.ReadFile(strings.TrimPrefix(got, "/")); err != nil {
		t.Fatalf("o arquivo servido por %s precisa estar embutido: %v", got, err)
	}
	if Asset("js/x.js") != "/static/js/x.js" {
		t.Fatal(Asset("js/x.js"))
	}
}
