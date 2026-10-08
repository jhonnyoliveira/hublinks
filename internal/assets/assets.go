package assets

import (
	webtmpl "github.com/hublinks/hublinks/web"
	"io/fs"
	"net/http"
	"path"
	"regexp"
)

var hashedName = regexp.MustCompile(`\.[a-f0-9]{8}\.`)

func Handler() http.Handler {
	static, err := fs.Sub(webtmpl.Files, "static")
	if err != nil {
		panic(err)
	}
	return HandlerFor(static)
}

// HandlerFor serve fsys sob /static/. Arquivos cujo nome traz o hash do build
// (app.<8 hex>.css) são imutáveis e podem ficar em cache por um ano; os demais
// não recebem cabeçalho de cache longo.
func HandlerFor(fsys fs.FS) http.Handler {
	files := http.FileServer(http.FS(fsys))
	return http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hashedName.MatchString(path.Base(r.URL.Path)) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	}))
}
