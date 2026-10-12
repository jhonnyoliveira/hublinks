package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// stage devolve o trecho do Dockerfile de um estágio nomeado ("css", "build") ou,
// para "final", o último.
func stage(t *testing.T, dockerfile, name string) string {
	t.Helper()
	parts := regexp.MustCompile(`(?m)^FROM `).Split(dockerfile, -1)[1:] // descarta o preâmbulo
	if name == "final" {
		return parts[len(parts)-1]
	}
	for _, p := range parts {
		if strings.HasSuffix(strings.TrimSpace(strings.SplitN(p, "\n", 2)[0]), " AS "+name) {
			return p
		}
	}
	t.Fatalf("estágio %q não encontrado", name)
	return ""
}

// O estágio de CSS compila web/assets/app.css, que importa base.css e varre os
// templates: sem esses arquivos o build da imagem falha ou gera CSS sem utilitários.
func TestDockerfileCSSStageHasItsInputs(t *testing.T) {
	css := stage(t, read(t, "../../deploy/Dockerfile"), "css")
	for _, want := range []string{"COPY scripts/build-css.sh", "COPY web/assets", "COPY web/static/css/base.css", "COPY web/templates"} {
		if !strings.Contains(css, want) {
			t.Errorf("estágio css sem %q", want)
		}
	}
	if strings.Contains(css, "alpine") {
		t.Error("o Tailwind standalone é glibc: não use Alpine no estágio css")
	}
	app := read(t, "../../web/assets/app.css")
	for _, dep := range []string{"../static/css/base.css", `source("../templates")`} {
		if !strings.Contains(app, dep) {
			t.Fatalf("web/assets/app.css mudou (%s); revise o estágio css do Dockerfile", dep)
		}
	}
}

func TestDockerfileBuildAndFinalStages(t *testing.T) {
	df := read(t, "../../deploy/Dockerfile")
	build := stage(t, df, "build")
	for _, want := range []string{"COPY --from=css /src/web/static/css web/static/css", "COPY --from=css /src/web/static/manifest.json web/static/manifest.json", "CGO_ENABLED=0", "-trimpath", "-s -w"} {
		if !strings.Contains(build, want) {
			t.Errorf("estágio build sem %q", want)
		}
	}
	final := stage(t, df, "final")
	for _, want := range []string{"gcr.io/distroless/static-debian12:nonroot", "EXPOSE 8080", `ENTRYPOINT ["/hublinks"]`, `CMD ["serve"]`} {
		if !strings.Contains(final, want) {
			t.Errorf("estágio final sem %q", want)
		}
	}
}

func TestComposeServesAppAndDatabaseWithoutPublishingMetrics(t *testing.T) {
	c := read(t, "../../deploy/docker-compose.yml")
	for _, want := range []string{"dockerfile: deploy/Dockerfile", "context: ..", "env_file: .env", `"8081:8081"`, "condition: service_healthy", "postgres:16-alpine", "pg_isready", "hublinks-postgres:/var/lib/postgresql/data", `[ "CMD", "/hublinks", "healthcheck" ]`} {
		if !strings.Contains(c, want) {
			t.Errorf("docker-compose.yml sem %q", want)
		}
	}
	if strings.Contains(c, "9091:") {
		t.Error("a porta de métricas não deve ser publicada")
	}
	if !strings.Contains(read(t, "../../.dockerignore"), ".env*") {
		t.Error(".dockerignore deve excluir arquivos .env")
	}
}

// Toda variável do contrato de configuração aparece no .env.example.
func TestEnvExampleCoversEveryDocumentedVariable(t *testing.T) {
	example := read(t, "../../deploy/.env.example")
	contract := read(t, "../../specs/001-encurtador-analytics/contracts/config.md")
	names := map[string]bool{}
	for _, line := range strings.Split(contract, "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cell := strings.SplitN(line, "|", 3)[1]
		for _, m := range regexp.MustCompile("`([A-Z][A-Z0-9_]+)`").FindAllStringSubmatch(cell, -1) {
			names[m[1]] = true
		}
	}
	if len(names) < 20 {
		t.Fatalf("o contrato deveria listar as variáveis (achei %d)", len(names))
	}
	for name := range names {
		if !regexp.MustCompile(`(?m)^#? ?` + name + `=`).MatchString(example) {
			t.Errorf(".env.example sem %s", name)
		}
	}
	if !strings.Contains(example, "PEPPER") || !strings.Contains(strings.ToLower(example), "não rotacione") {
		t.Error(".env.example deve avisar que o PEPPER nunca é rotacionado")
	}
	if !strings.Contains(example, "POSTGRES_PASSWORD=") {
		t.Error(".env.example deve definir POSTGRES_PASSWORD")
	}
}
