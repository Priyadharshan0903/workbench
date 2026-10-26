package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"workbench/internal/launch"
	"workbench/internal/testutil"
)

func fixtureApp(t *testing.T) *App {
	s, _ := testutil.FixtureStore(t)
	sup := launch.New(s)
	sup.Inspector = testutil.FakeInspector("", "")
	return New(sup, 8739)
}
func requestApp(a *App, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "http://127.0.0.1:8739"+path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
		if strings.EqualFold(k, "Host") {
			req.Host = v
		}
	}
	res := httptest.NewRecorder()
	a.ServeHTTP(res, req)
	return res
}
func TestHTTPAssetsAndSameOrigin(t *testing.T) {
	app := fixtureApp(t)
	for _, path := range []string{"/api/health", "/api/state", "/api/registry", "/", "/app.js", "/skills.js", "/styles.css", "/favicon.svg", "/vendor/htmx.min.js", "/fonts/InterVariable.woff2"} {
		res := requestApp(app, "GET", path, "", nil)
		if res.Code != 200 {
			t.Fatalf("%s: %d %s", path, res.Code, res.Body.String())
		}
		if !strings.Contains(res.Header().Get("Content-Security-Policy"), "script-src 'self'") {
			t.Fatal("CSP missing")
		}
	}
	for _, path := range []string{"/registry.json", "/../registry.json", "/.env", "/model.go", "/public/app.js", "/vendor/HTMX-LICENSE"} {
		if res := requestApp(app, "GET", path, "", nil); res.Code != 404 {
			t.Fatalf("private asset: %s %d", path, res.Code)
		}
	}
	for _, headers := range []map[string]string{{"Host": "evil.test"}, {"Origin": "https://evil.test"}, {"Origin": "null"}, {"Origin": ""}, {"Sec-Fetch-Site": "cross-site"}} {
		if res := requestApp(app, "GET", "/api/health", "", headers); res.Code != 403 {
			t.Fatal("foreign request allowed", headers, res.Code)
		}
	}
	if requestApp(app, "DELETE", "/api/project", "", nil).Code != 405 {
		t.Fatal("method accepted")
	}
}
func TestHTTPWritesRequireTokenBoundedJSONObjectAndIntegerRevision(t *testing.T) {
	app := fixtureApp(t)
	if requestApp(app, "POST", "/api/project", "{}", map[string]string{"Content-Type": "application/json"}).Code != 403 {
		t.Fatal("write without token")
	}
	headers := map[string]string{"Content-Type": "application/json", "X-Workbench-Token": app.CSRF}
	for _, body := range []string{"{", "[]", "null", strings.Repeat("x", 32769), `{"revision":true}`, `{"revision":null}`, `{"revision":1.5}`, `{"revision":0}`} {
		if res := requestApp(app, "POST", "/api/project", body, headers); res.Code != 400 {
			t.Fatalf("body %q: %d", body[:min(len(body), 60)], res.Code)
		}
	}
	_, p := testutil.FixtureStore(t)
	payload, _ := json.Marshal(map[string]any{"revision": 1, "project": p})
	res := requestApp(app, "POST", "/api/project", string(payload), headers)
	if res.Code != 200 {
		t.Fatal(res.Code, res.Body.String())
	}
	if requestApp(app, "POST", "/api/project", string(payload), headers).Code != 400 {
		t.Fatal("stale write accepted")
	}
}
func TestLogFragmentEscapesAndRedactsOutput(t *testing.T) {
	app := fixtureApp(t)
	s := app.Supervisor
	r := launch.RunRecord{ProjectID: "sample", RunID: "0123456789abcdef", Phase: "failed", StartedAt: time.Now().UTC().Format(time.RFC3339Nano), LogFile: "sample-0123456789abcdef.log"}
	path := filepath.Join(s.Store.Root, ".runtime/runs/sample.json")
	b, _ := json.Marshal(r)
	testutil.Write(t, path, string(b))
	testutil.Write(t, filepath.Join(s.Store.Root, ".runtime/logs", r.LogFile), "<script>alert(1)</script>\ntoken=private-value Bearer abcd.efg\n")
	res := requestApp(app, "GET", "/fragments/logs?id=sample", "", nil)
	if res.Code != 200 || strings.Contains(res.Body.String(), "<script>") || !strings.Contains(res.Body.String(), "&lt;script&gt;") || strings.Contains(res.Body.String(), "private-value") || strings.Contains(res.Body.String(), "abcd.efg") {
		t.Fatalf("unsafe fragment: %d %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `id="log-output"`) {
		t.Fatal("fragment lost UI contract")
	}
	r.LogFile = "../registry.json"
	b, _ = json.Marshal(r)
	testutil.Write(t, path, string(b))
	if requestApp(app, "GET", "/api/logs?id=sample", "", nil).Code != 500 {
		t.Fatal("log traversal accepted")
	}
}
func TestHTTPReadinessDoesNotFollowRedirectOrTrustRemote(t *testing.T) {
	app := fixtureApp(t)
	remoteHits := 0
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { remoteHits++; w.WriteHeader(200) }))
	defer remote.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", remote.URL)
		w.WriteHeader(302)
	}))
	defer redirect.Close()
	health := app.Supervisor.CheckHTTP(redirect.URL)
	if health.State != "ready" || health.Code != 302 || remoteHits != 0 {
		t.Fatal("redirect followed", health, remoteHits)
	}
	if app.Supervisor.CheckHTTP("https://example.com").State != "unchecked" {
		t.Fatal("remote health probe allowed")
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }))
	defer bad.Close()
	if app.Supervisor.CheckHTTP(bad.URL).State != "error" {
		t.Fatal("404 marked ready")
	}
}
func TestJSONAndHTMLLogsHaveSameContent(t *testing.T) {
	app := fixtureApp(t)
	res := requestApp(app, "GET", "/api/logs?id=sample", "", nil)
	var logs launch.LogOutput
	if e := json.NewDecoder(bytes.NewReader(res.Body.Bytes())).Decode(&logs); e != nil {
		t.Fatal(e)
	}
	fragment := requestApp(app, "GET", "/fragments/logs?id=sample", "", nil)
	content, _ := io.ReadAll(fragment.Result().Body)
	if !strings.Contains(string(content), logs.Message) {
		t.Fatal("HTML and JSON log messages differ")
	}
}
