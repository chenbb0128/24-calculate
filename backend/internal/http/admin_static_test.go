package httpapi

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdminStaticFilesAreServedFromConfiguredDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "static"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<div id=app>admin</div>"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "static", "app.js"), []byte("console.log('admin')"), 0o640); err != nil {
		t.Fatal(err)
	}

	router, err := NewRouter(testConfig(), slog.Default(), RouterOptions{})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	RegisterAdminStatic(router, root)
	for _, test := range []struct {
		path   string
		status int
		body   string
	}{
		{path: "/admin", status: http.StatusMovedPermanently, body: "/admin/"},
		{path: "/admin/", status: http.StatusOK, body: "admin"},
		{path: "/admin/static/app.js", status: http.StatusOK, body: "console.log('admin')"},
	} {
		t.Run(test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
			if recorder.Code != test.status || !strings.Contains(recorder.Body.String()+recorder.Header().Get("Location"), test.body) {
				t.Fatalf("status = %d, body = %q", recorder.Code, recorder.Body.String())
			}
		})
	}
}
