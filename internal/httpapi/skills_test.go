package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"workbench/internal/skills"
	"workbench/internal/testutil"
)

func fixtureSkillApp(t *testing.T, managed bool) (*App, skills.Document) {
	t.Helper()
	app := fixtureApp(t)
	app.SkillFiles.Home = t.TempDir()
	folder := ".agents/skills/example"
	if managed {
		folder = ".codex/plugins/cache/example/skills/example"
	}
	testutil.Write(t, filepath.Join(app.SkillFiles.Home, folder, "SKILL.md"), "# Instructions\n\nOriginal instructions.\n")
	discovered, err := app.Skills(true)
	if err != nil || len(discovered.Skills) != 1 {
		t.Fatal(discovered, err)
	}
	document, err := app.SkillFiles.View(discovered.Skills[0].ID, "")
	if err != nil {
		t.Fatal(err)
	}

	return app, document
}

func TestSkillReadsAndWritesRespectLocalHTTPBoundary(t *testing.T) {
	app, document := fixtureSkillApp(t, false)
	path := "/api/skill?id=" + url.QueryEscape(document.Skill.ID)
	response := requestApp(app, http.MethodGet, path, "", nil)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Original instructions") {
		t.Fatal(response.Code, response.Body.String())
	}
	if requestApp(app, http.MethodGet, path, "", map[string]string{"Origin": "https://other.test"}).Code != http.StatusForbidden {
		t.Fatal("foreign skill read accepted")
	}
	for _, filename := range []string{"../registry.json", ".env", "/etc/passwd"} {
		response := requestApp(app, http.MethodGet, path+"&file="+url.QueryEscape(filename), "", nil)
		if response.Code != http.StatusNotFound {
			t.Fatal("arbitrary file was readable", filename, response.Code)
		}
	}

	content := strings.Repeat("A meaningful instruction.\n", 2000)
	payload, err := json.Marshal(skills.SaveRequest{ID: document.Skill.ID, Revision: document.Revision, Content: &content})
	if err != nil {
		t.Fatal(err)
	}
	if requestApp(app, http.MethodPost, "/api/skill/save", string(payload), map[string]string{"Content-Type": "application/json"}).Code != http.StatusForbidden {
		t.Fatal("skill write without CSRF accepted")
	}
	headers := map[string]string{"Content-Type": "application/json", "X-Workbench-Token": app.CSRF}
	response = requestApp(app, http.MethodPost, "/api/skill/save", string(payload), headers)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "backupPath") {
		t.Fatal("bounded skill content larger than 32 KB failed", response.Code, response.Body.String())
	}
	if requestApp(app, http.MethodPost, "/api/skill/save", string(payload), headers).Code != http.StatusConflict {
		t.Fatal("stale edit did not return HTTP 409")
	}
	for _, invalid := range []string{"null", "[]", "{}", `{"id":true}`, `{"content":null}`, `{"content":"text","path":"/arbitrary"}`, `{"content":"text"} {}`} {
		if response := requestApp(app, http.MethodPost, "/api/skill/save", invalid, headers); response.Code != http.StatusBadRequest {
			t.Fatal("invalid skill request accepted", invalid, response.Code)
		}
	}
}

func TestManagedSkillHTTPWriteDeniedAndPreviewDoesNotExecuteHTML(t *testing.T) {
	app, document := fixtureSkillApp(t, true)
	headers := map[string]string{"Content-Type": "application/json", "X-Workbench-Token": app.CSRF}
	content := "<script>alert(1)</script>\n\n# Preview\n"
	payload, _ := json.Marshal(skills.SaveRequest{ID: document.Skill.ID, Revision: document.Revision, Content: &content})
	if response := requestApp(app, http.MethodPost, "/api/skill/save", string(payload), headers); response.Code != http.StatusForbidden {
		t.Fatal("managed skill was writable", response.Code, response.Body.String())
	}
	previewPayload, _ := json.Marshal(struct {
		Content string `json:"content"`
	}{Content: content})
	response := requestApp(app, http.MethodPost, "/api/skill/preview", string(previewPayload), headers)
	var preview skills.Preview
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &preview) != nil || strings.Contains(preview.HTML, "<script>") || !strings.Contains(preview.HTML, "<h1>Preview</h1>") {
		t.Fatal("unsafe Markdown preview", response.Code, response.Body.String())
	}

	exportURL := "/api/skill/export?" + url.Values{"id": {document.Skill.ID}, "revision": {document.BundleRevision}}.Encode()
	response = requestApp(app, http.MethodGet, exportURL, "", nil)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/zip" || !strings.HasPrefix(response.Body.String(), "PK") {
		t.Fatal("skill bundle download failed", response.Code)
	}
}
