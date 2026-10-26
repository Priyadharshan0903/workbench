package skills

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"workbench/internal/testutil"
)

const fixtureInstructions = "---\nname: sample\ndescription: A test skill\n---\n# Instructions\n\nRead [the reference](references/guide.md).\n"

func fixtureLibrary(t *testing.T, managed bool) (*Library, Document) {
	t.Helper()
	store, _ := testutil.FixtureStore(t)
	library := &Library{Store: store, Home: t.TempDir()}
	relative := ".agents/skills/sample"
	if managed {
		relative = ".claude/plugins/cache/example/skills/sample"
	}
	folder := filepath.Join(library.Home, relative)
	testutil.Write(t, filepath.Join(folder, "SKILL.md"), fixtureInstructions)
	testutil.Write(t, filepath.Join(folder, "references/guide.md"), "# Reference\n\nDo useful work.\n")
	testutil.Write(t, filepath.Join(folder, "scripts/check.sh"), "#!/bin/sh\necho fixture\n")
	if err := os.Chmod(filepath.Join(folder, "scripts/check.sh"), 0755); err != nil {
		t.Fatal(err)
	}

	discovered, err := library.Discover()
	if err != nil || len(discovered.Skills) != 1 {
		t.Fatalf("discovery: %+v %v", discovered, err)
	}
	document, err := library.View(discovered.Skills[0].ID, "")
	if err != nil {
		t.Fatal(err)
	}

	return library, document
}

func TestViewReadsOnlyDiscoveredRegularBundleFiles(t *testing.T) {
	library, document := fixtureLibrary(t, false)
	if !document.Editable || document.Content != fixtureInstructions || !strings.Contains(document.PreviewHTML, "<h1>Instructions</h1>") || strings.Contains(document.PreviewHTML, "name: sample") {
		t.Fatalf("unexpected main document: %+v", document)
	}

	folder := filepath.Dir(document.Skill.Path)
	testutil.Write(t, filepath.Join(folder, ".env"), "fixture-secret")
	testutil.Write(t, filepath.Join(folder, "secrets.json"), "fixture-secret")
	outside := filepath.Join(t.TempDir(), "outside.txt")
	testutil.Write(t, outside, "outside-sentinel")
	if err := os.Symlink(outside, filepath.Join(folder, "outside-link.txt")); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"../outside.txt", "/etc/passwd", ".env", "secrets.json", "outside-link.txt", "references/../../outside.txt"} {
		if _, err := library.View(document.Skill.ID, name); !errors.Is(err, ErrNotFound) {
			t.Fatal("unlisted or linked file was readable", name, err)
		}
	}
	if _, err := library.View("../../arbitrary", "SKILL.md"); !errors.Is(err, ErrNotFound) {
		t.Fatal("arbitrary skill path accepted", err)
	}

	reference, err := library.View(document.Skill.ID, "references/guide.md")
	if err != nil || reference.Editable || !strings.Contains(reference.Content, "Do useful work") {
		t.Fatal(reference, err)
	}
	refreshed, err := library.View(document.Skill.ID, "")
	if err != nil || len(refreshed.Files) != 3 || len(refreshed.Warnings) != 3 {
		t.Fatal("bundle exclusions are missing", refreshed, err)
	}
}

func TestMarkdownPreviewDoesNotRenderHTMLOrUnsafeURLs(t *testing.T) {
	preview, err := Render("# Heading\n\n<script>alert(1)</script>\n\n[unsafe](javascript:alert(1))\n\n```html\n<script>example</script>\n```\n")
	if err != nil || strings.Contains(preview.HTML, "<script>") || strings.Contains(preview.HTML, `href="javascript:`) || !strings.Contains(preview.HTML, "&lt;script&gt;example") {
		t.Fatal("unsafe or broken Markdown preview", preview, err)
	}
	for _, invalid := range []string{"a\x00b", string([]byte{0xff}), strings.Repeat("x", MaxContentBytes+1)} {
		if _, err := Render(invalid); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid Markdown was accepted", err)
		}
	}
}

func TestSavePreservesPreviousVersionAndRejectsStaleEdits(t *testing.T) {
	library, document := fixtureLibrary(t, false)
	updated := fixtureInstructions + "\nAn explicit edit.\n"
	result, err := library.Save(SaveRequest{ID: document.Skill.ID, Revision: document.Revision, Content: &updated})
	if err != nil || !result.OK || result.Revision == document.Revision {
		t.Fatal(result, err)
	}
	backup, err := os.ReadFile(result.BackupPath)
	if err != nil || string(backup) != fixtureInstructions {
		t.Fatal("backup does not contain the opened version", string(backup), err)
	}
	backupInfo, _ := os.Stat(result.BackupPath)
	if backupInfo.Mode().Perm() != 0600 {
		t.Fatal("backup has unexpected permissions", backupInfo.Mode())
	}
	if _, err := library.Save(SaveRequest{ID: document.Skill.ID, Revision: document.Revision, Content: &updated}); !errors.Is(err, ErrConflict) {
		t.Fatal("stale edit overwrote a newer file", err)
	}
	contents, _ := os.ReadFile(document.Skill.Path)
	if string(contents) != updated {
		t.Fatal("save did not preserve the complete updated file")
	}

	current, _ := library.View(document.Skill.ID, "")
	testutil.Write(t, document.Skill.Path, "External editor change\n")
	if _, err := library.Save(SaveRequest{ID: current.Skill.ID, Revision: current.Revision, Content: &updated}); !errors.Is(err, ErrConflict) {
		t.Fatal("external editor change was overwritten", err)
	}
}

func TestConcurrentSavesHaveOneWinner(t *testing.T) {
	library, document := fixtureLibrary(t, false)
	other := &Library{Store: library.Store, Home: library.Home}
	var group sync.WaitGroup
	results := make(chan error, 2)
	for _, writer := range []*Library{library, other} {
		group.Add(1)
		go func(writer *Library) {
			defer group.Done()
			content := "# Updated\n"
			_, err := writer.Save(SaveRequest{ID: document.Skill.ID, Revision: document.Revision, Content: &content})
			results <- err
		}(writer)
	}
	group.Wait()
	close(results)
	winners, conflicts := 0, 0
	for err := range results {
		if err == nil {
			winners++
		} else if errors.Is(err, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if winners != 1 || conflicts != 1 {
		t.Fatal("concurrent saves did not protect the first edit", winners, conflicts)
	}
}

func TestManagedSkillsCanBeCopiedButNotEdited(t *testing.T) {
	library, document := fixtureLibrary(t, true)
	if document.Editable {
		t.Fatal("plugin original is editable")
	}
	updated := "# Unreviewed edit\n"
	if _, err := library.Save(SaveRequest{ID: document.Skill.ID, Revision: document.Revision, Content: &updated}); !errors.Is(err, ErrReadOnly) {
		t.Fatal("plugin original was modified", err)
	}
	if err := os.Chmod(document.Skill.Path, 0444); err != nil {
		t.Fatal(err)
	}
	document, _ = library.View(document.Skill.ID, "")
	copyRequest := CopyRequest{ID: document.Skill.ID, Revision: document.BundleRevision, Destination: document.Destinations[0].ID, Name: "editable-copy"}
	result, err := library.Duplicate(copyRequest)
	if err != nil {
		t.Fatal(err)
	}
	copy, err := library.View(result.ID, "")
	if err != nil || !copy.Editable || len(copy.Files) != 3 || copy.Content != fixtureInstructions {
		t.Fatal("copied bundle is not complete and editable", copy, err)
	}
	scriptInfo, err := os.Stat(filepath.Join(result.Path, "scripts/check.sh"))
	if err != nil || scriptInfo.Mode().Perm()&0100 == 0 {
		t.Fatal("supporting script permissions were lost", err)
	}
	if _, err := library.Duplicate(copyRequest); err == nil {
		t.Fatal("duplicate silently overwrote an existing folder")
	}
	original, _ := os.ReadFile(document.Skill.Path)
	if string(original) != fixtureInstructions {
		t.Fatal("copy changed plugin original")
	}
}

func TestBundleExportAndCopyRequireReviewedContentsAndDestinations(t *testing.T) {
	library, document := fixtureLibrary(t, false)
	archive, err := library.Export(document.Skill.ID, document.BundleRevision)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil || len(reader.File) != 3 {
		t.Fatal("export lost supporting files", err)
	}
	for _, file := range reader.File {
		stream, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(stream)
		stream.Close()
		if err != nil || len(content) == 0 || !strings.HasPrefix(file.Name, "skill/") {
			t.Fatal("invalid exported file", file.Name, err)
		}
	}
	testutil.Write(t, filepath.Join(filepath.Dir(document.Skill.Path), "references/guide.md"), "Changed after review")
	if _, err := library.Export(document.Skill.ID, document.BundleRevision); !errors.Is(err, ErrConflict) {
		t.Fatal("stale bundle exported", err)
	}
	for _, request := range []CopyRequest{
		{ID: document.Skill.ID, Revision: document.BundleRevision, Destination: document.Destinations[0].ID, Name: "new-copy"},
		{ID: document.Skill.ID, Revision: document.BundleRevision, Destination: "/arbitrary/folder", Name: "new-copy"},
		{ID: document.Skill.ID, Revision: document.BundleRevision, Destination: document.Destinations[0].ID, Name: "../escape"},
	} {
		if _, err := library.Duplicate(request); err == nil {
			t.Fatal("stale or arbitrary copy accepted", request)
		}
	}
}
