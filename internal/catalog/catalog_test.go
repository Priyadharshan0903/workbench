package catalog_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"workbench/internal/catalog"
	"workbench/internal/testutil"
)

func TestRegistryAtomicRevisionAndBackup(t *testing.T) {
	s, p := testutil.FixtureStore(t)
	original, _ := os.ReadFile(s.Path())
	p.Name = "Updated"
	r, e := s.Save(p, 1)
	if e != nil || r.Revision != 2 || len(r.Projects) != 1 {
		t.Fatalf("save: %+v %v", r, e)
	}
	backup, _ := os.ReadFile(filepath.Join(s.Root, "registry.previous.json"))
	if !bytes.Equal(original, backup) {
		t.Fatal("previous snapshot changed")
	}
	p.Name = "Stale"
	if _, e = s.Save(p, 1); e == nil {
		t.Fatal("stale revision accepted")
	}
	r, _ = s.Read()
	if r.Projects[0].Name != "Updated" {
		t.Fatal("stale write replaced current data")
	}
}
func TestRegistryConcurrentWriters(t *testing.T) {
	s, _ := testutil.FixtureStore(t)
	var wg sync.WaitGroup
	for n := 0; n < 12; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := testutil.FixtureProject(s.Root)
			p.ID = "project-" + testutil.ID(4)
			if _, e := s.Save(p, 0); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	r, e := s.Read()
	if e != nil || r.Revision != 13 || len(r.Projects) != 13 {
		t.Fatalf("lost concurrent write: %+v %v", r, e)
	}
}
func TestRegistryInvalidDataNeverWritten(t *testing.T) {
	s, p := testutil.FixtureStore(t)
	b, _ := json.Marshal(p)
	for _, patch := range []string{`"ports":[true]`, `"ports":[3000,3000]`, `"ports":[65536]`, `"ports":null`, `"id":[]`, `"name":""`, `"name":null`, `"kind":"unknown"`, `"lifecycle":"deleted"`, `"path":"relative"`, `"path":"/tmp/.env.private"`, `"url":"javascript:alert(1)"`, `"url":"http://user:pass@localhost/"`, `"extra":1`} {
		t.Run(patch, func(t *testing.T) {
			var raw map[string]json.RawMessage
			json.Unmarshal(b, &raw)
			var v map[string]json.RawMessage
			json.Unmarshal([]byte("{"+patch+"}"), &v)
			for k, x := range v {
				raw[k] = x
			}
			payload, _ := json.Marshal(raw)
			var invalid catalog.Project
			if e := json.Unmarshal(payload, &invalid); e == nil {
				t.Fatalf("accepted invalid record: %s", payload)
			}
		})
	}
	r, _ := s.Read()
	if r.Revision != 1 {
		t.Fatal("invalid data changed catalog")
	}
}
func TestCorruptRegistryNotInitializedOrSaved(t *testing.T) {
	s, p := testutil.FixtureStore(t)
	testutil.Write(t, s.Path(), "{")
	if e := s.Initialize(8730); e == nil {
		t.Fatal("corrupt catalog initialized")
	}
	if _, e := s.Save(p, 0); e == nil {
		t.Fatal("corrupt catalog saved")
	}
	b, _ := os.ReadFile(s.Path())
	if string(b) != "{" {
		t.Fatal("corrupt catalog replaced")
	}
}
