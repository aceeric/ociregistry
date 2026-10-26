package subcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/serialize"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

func legacyManifest(t *testing.T, td string) {
	t.Helper()
	b, err := json.Marshal(imgpull.ManifestHolder{Type: imgpull.V1ociIndex, Digest: "1111111111111111111111111111111111111111111111111111111111111111", ImageUrl: "foo.io/foo:v1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(td, globals.LtsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(td, globals.LtsPath, "1111111111111111111111111111111111111111111111111111111111111111"), b, 0644); err != nil {
		t.Fatal(err)
	}
}

// Tests that the server migrates a legacy cache at startup.
func TestMigrateOnStartup(t *testing.T) {
	td := t.TempDir()
	legacyManifest(t, td)
	if err := migrateOnStartup(td); err != nil {
		t.Fatal(err)
	}
	if _, found := serialize.MhFromFilesystem("foo.io/foo:v1", td); !found {
		t.Fatal("expected the manifest to be migrated")
	}
	if needs, _ := serialize.NeedsMigration(td); needs {
		t.Fatal("expected the cache to be migrated")
	}
}

// Tests that the server refuses to start if the cache can't be migrated.
func TestMigrateOnStartupIncomplete(t *testing.T) {
	td := t.TempDir()
	legacyManifest(t, td)
	os.WriteFile(filepath.Join(td, globals.ManifestPath), []byte("x"), 0644)
	if err := migrateOnStartup(td); err == nil {
		t.Fatal("expected an error so the server does not start")
	}
}
