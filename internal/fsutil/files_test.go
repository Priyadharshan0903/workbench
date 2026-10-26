package fsutil

import (
	"path/filepath"
	"testing"
)

func TestPathBoundaryAndSecretRoots(t *testing.T) {
	root := t.TempDir()
	if Within(root+"-other", root) || !Within(filepath.Join(root, "child"), root) {
		t.Fatal("path boundary failed")
	}
	for _, path := range []string{"/tmp/.env", "/tmp/.env.local/child"} {
		if !SecretPath(path) {
			t.Fatal("secret root accepted")
		}
	}
}
