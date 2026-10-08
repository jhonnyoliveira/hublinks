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
	return http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hashedName.MatchString(path.Base(r.URL.Path)) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.FileServer(http.FS(static)).ServeHTTP(w, r)
	}))
}
