package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"syscall"
	"testing"
)

func TestProjectRemovalRequiresReviewAndNeverSignalsProcesses(t *testing.T) {
	app := fixtureApp(t)
	registry, err := app.Supervisor.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	id := registry.Projects[0].ID
	headers := map[string]string{"Content-Type": "application/json", "X-Workbench-Token": app.CSRF}
	signals := 0
	app.Signal = func(int, syscall.Signal) error { signals++; return nil }
	before, _ := os.ReadFile(app.Supervisor.Store.Path())
	for _, body := range []string{
		`{}`, `{"id":"` + id + `"}`, `{"id":"` + id + `","revision":null}`,
		`{"id":"` + id + `","revision":true}`, `{"id":"` + id + `","revision":0}`,
		`{"id":"` + id + `","revision":99}`, `{"id":"workbench","revision":1}`,
	} {
		if response := requestApp(app, "POST", "/api/project/remove", body, headers); response.Code != 400 {
			t.Fatal("invalid removal accepted", response.Code, response.Body.String())
		}
	}
	after, _ := os.ReadFile(app.Supervisor.Store.Path())
	if string(before) != string(after) {
		t.Fatal("invalid requests changed the catalog")
	}
	body := `{"id":"` + id + `","revision":1}`
	if response := requestApp(app, "POST", "/api/project/remove", body, map[string]string{"Content-Type": "application/json"}); response.Code != http.StatusForbidden {
		t.Fatal("removal without CSRF protection accepted")
	}
	foreignHeaders := map[string]string{"Content-Type": "application/json", "X-Workbench-Token": app.CSRF, "Origin": "https://other.example"}
	if response := requestApp(app, "POST", "/api/project/remove", body, foreignHeaders); response.Code != http.StatusForbidden {
		t.Fatal("cross-origin removal accepted")
	}
	response := requestApp(app, "POST", "/api/project/remove", body, headers)
	var result ActionResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 || !result.OK || signals != 0 {
		t.Fatal("confirmed removal failed or signalled a process", response.Code, response.Body.String(), err)
	}
}
