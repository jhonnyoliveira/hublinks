// Package audit reúne verificações estáticas das invariantes de privacidade:
// elas rodam junto com os testes e falham se o código novo sair do que o projeto
// promete (IP puro nunca persistido nem logado; nenhum cookie fora da sessão do painel).
package audit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// production lista os arquivos .go de produção (sem _test.go) sob a raiz do módulo.
func production(t *testing.T) map[string][]byte {
	t.Helper()
	root := filepath.Join("..", "..")
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "bin", "audit":
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			files[filepath.ToSlash(rel)] = b
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 30 {
		t.Fatalf("achei só %d arquivos de produção: o teste deixou de enxergar o código", len(files))
	}
	return files
}

// codeOnly remove comentários e literais não relevantes? Não: a busca é por
// identificadores e strings de código, então usamos o AST e ignoramos comentários.
func uses(t *testing.T, src []byte, match func(ast.Node) bool) bool {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0) // 0: comentários descartados
	if err != nil {
		t.Fatal(err)
	}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		if n != nil && match(n) {
			found = true
		}
		return !found
	})
	return found
}

func onlyIn(t *testing.T, what string, allowed []string, match func(ast.Node) bool) {
	t.Helper()
	var where []string
	for path, src := range production(t) {
		if uses(t, src, match) {
			where = append(where, path)
		}
	}
	sort.Strings(where)
	allow := map[string]bool{}
	for _, a := range allowed {
		allow[a] = true
	}
	for _, w := range where {
		if !allow[w] {
			t.Errorf("%s aparece em %s; só é permitido em %v", what, w, allowed)
		}
	}
	if len(where) == 0 {
		t.Errorf("%s não aparece em lugar nenhum: o teste está cego (esperado em %v)", what, allowed)
	}
}

func selector(name string) func(ast.Node) bool {
	return func(n ast.Node) bool {
		s, ok := n.(*ast.SelectorExpr)
		return ok && s.Sel.Name == name
	}
}

func stringLit(value string) func(ast.Node) bool {
	return func(n ast.Node) bool {
		l, ok := n.(*ast.BasicLit)
		return ok && l.Kind == token.STRING && strings.Contains(strings.ToLower(l.Value), strings.ToLower(value))
	}
}

// O IP do cliente só é lido em httpx/clientip.go, onde passa pela regra de proxies
// confiáveis e é normalizado antes de virar chave ou identificador.
func TestRemoteAddrIsReadOnlyByClientIP(t *testing.T) {
	onlyIn(t, "r.RemoteAddr", []string{"internal/httpx/clientip.go"}, selector("RemoteAddr"))
}

func TestForwardedForIsReadOnlyByClientIP(t *testing.T) {
	onlyIn(t, `o cabeçalho X-Forwarded-For`, []string{"internal/httpx/clientip.go"}, stringLit("X-Forwarded-For"))
	for path, src := range production(t) {
		if uses(t, src, stringLit("X-Real-IP")) {
			t.Errorf("%s lê X-Real-IP; só X-Forwarded-For de proxies confiáveis é aceito", path)
		}
	}
}

// Cookies: só a sessão do painel (auth/session.go) emite ou lê cookies.
func TestCookiesAreOnlyTouchedBySessionCode(t *testing.T) {
	for _, name := range []string{"SetCookie", "Cookie", "Cookies", "AddCookie"} {
		var where []string
		for path, src := range production(t) {
			if uses(t, src, selector(name)) {
				where = append(where, path)
			}
		}
		sort.Strings(where)
		for _, w := range where {
			// admin/login.go usa o cookie da sessão: auth.SetCookie no login e a leitura dele no logout.
			if w != "internal/auth/session.go" && w != "internal/admin/login.go" {
				t.Errorf("%s usado em %s", name, w)
			}
		}
	}
	// o cookie da sessão é HttpOnly, Secure e SameSite=Lax
	src := string(production(t)["internal/auth/session.go"])
	for _, want := range []string{"HttpOnly: true", "Secure: true", "SameSite: http.SameSiteLaxMode"} {
		if !strings.Contains(src, want) {
			t.Errorf("cookie da sessão sem %s", want)
		}
	}
	// as respostas públicas (redirect, web) não tocam em cookies
	for path, src := range production(t) {
		if strings.HasPrefix(path, "internal/redirect/") || strings.HasPrefix(path, "internal/web/") || strings.HasPrefix(path, "internal/events/") {
			if uses(t, src, stringLit("Set-Cookie")) {
				t.Errorf("%s menciona Set-Cookie", path)
			}
		}
	}
}

// Nenhuma tabela guarda IP: nenhuma coluna de migração se chama ip, remote_addr etc.
func TestNoMigrationStoresAnIPAddress(t *testing.T) {
	entries, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("migrações não encontradas: %v", err)
	}
	for _, f := range entries {
		b, _ := os.ReadFile(f)
		sql := strings.ToLower(string(b))
		for _, bad := range []string{" ip ", " ip_", "_ip ", "remote_addr", "client_ip", " inet", " cidr"} {
			if strings.Contains(sql, bad) {
				t.Errorf("%s parece guardar IP (%q)", filepath.Base(f), bad)
			}
		}
	}
}

// Os logs nunca recebem o IP: nenhuma chamada de slog/logger passa a requisição
// inteira nem um endereço de rede.
func TestLogsNeverReceiveNetworkAddresses(t *testing.T) {
	for path, src := range production(t) {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch sel.Sel.Name {
			case "Info", "Warn", "Error", "Debug", "Log":
			default:
				return true
			}
			for _, arg := range call.Args {
				if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					key := strings.ToLower(lit.Value)
					for _, bad := range []string{`"ip"`, `"remote"`, `"remote_addr"`, `"client_ip"`, `"addr"`, `"x_forwarded_for"`, `"user_agent"`, `"referer"`} {
						if key == bad {
							t.Errorf("%s: log com a chave %s", path, bad)
						}
					}
				}
			}
			return true
		})
	}
}
