package assets

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestHandlerServesEmbeddedDevelopmentCSS(t *testing.T) {
	r := httptest.NewRecorder()
	Handler().ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/static/css/base.css", nil))
	if r.Code != http.StatusOK {
		t.Fatalf("status %d", r.Code)
	}
	if r.Header().Get("Cache-Control") != "" {
		t.Fatal("arquivo sem hash não pode ser imutável")
	}
}

func TestHandlerForMarksOnlyHashedFilesImmutable(t *testing.T) {
	fsys := fstest.MapFS{
		"css/app.0c45c7d0.css": {Data: []byte("a{}")},
		"css/base.css":         {Data: []byte("b{}")},
		"js/panel.js":          {Data: []byte("1")},
		"fonts/inter.woff2":    {Data: []byte("f")},
	}
	h := HandlerFor(fsys)
	cases := map[string]string{
		"/static/css/app.0c45c7d0.css": "public, max-age=31536000, immutable",
		"/static/css/base.css":         "",
		"/static/js/panel.js":          "",
		"/static/fonts/inter.woff2":    "",
	}
	for path, want := range cases {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != want {
			t.Errorf("%s: %d Cache-Control=%q (esperado %q)", path, w.Code, w.Header().Get("Cache-Control"), want)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/static/css/app.ffffffff.css", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("arquivo inexistente: %d", w.Code)
	}
}

func TestHandlerServesEmbeddedFilesUsedByTheLayouts(t *testing.T) {
	for _, path := range []string{"/static/css/base.css", "/static/js/panel.js", "/static/vendor/htmx.min.js", "/static/vendor/alpine.min.js", "/static/fonts/inter-latin-wght-normal.woff2"} {
		w := httptest.NewRecorder()
		Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK || w.Body.Len() == 0 {
			t.Errorf("%s: %d", path, w.Code)
		}
	}
}
