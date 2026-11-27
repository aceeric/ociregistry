package preload

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/serialize"
	"github.com/aceeric/ociregistry/mock"
	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

// ledgerCache returns a new, empty cache, set as the configured image path for linux/amd64.
func ledgerCache(t *testing.T) string {
	t.Helper()
	td := t.TempDir()
	if err := config.SetConfigFromStr(fmt.Appendf(nil, "imagePath: %s\nos: linux\narch: amd64\n", td)); err != nil {
		t.Fatal(err)
	}
	if err := serialize.CreateDirs(td, true); err != nil {
		t.Fatal(err)
	}
	return td
}

// stat returns the file info of path, failing the test if there's none.
func stat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi
}

// Tests recording files: the ledger is written sorted by name, read back by a new Ledger, and
// matches a file only with the same size and modification time.
func TestLedgerRoundTrip(t *testing.T) {
	td := ledgerCache(t)
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.tar"), filepath.Join(dir, "b.tar")
	writePlainFile(t, b, []byte("bbb"))
	writePlainFile(t, a, []byte("aa"))
	l := OpenLedger(td)
	if l.HasEntry(a, stat(t, a)) {
		t.Fatal("a new ledger should have nothing")
	}
	l.Record(b, stat(t, b), ResultFailed)
	l.Record(a, stat(t, a), ResultLoaded)

	l = OpenLedger(td)
	if !l.HasEntry(a, stat(t, a)) || !l.HasEntry(b, stat(t, b)) {
		t.Fatal("both files should be done after reading the ledger back")
	}
	entries := l.Entries()
	if len(entries) != 2 || entries[0].Name != a || entries[0].Result != ResultLoaded || entries[0].Size != 2 ||
		!globals.IsUtc(entries[0].Timestamp) || !globals.IsUtc(entries[0].Processed) || entries[1].Result != ResultFailed {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	content, _ := os.ReadFile(filepath.Join(td, globals.MetadataPath, LedgerFile))
	if !strings.HasPrefix(string(content), "version: 1\nfiles:\n") || strings.Index(string(content), "a.tar") > strings.Index(string(content), "b.tar") {
		t.Fatalf("expected a version 1 ledger sorted by name, got:\n%s", content)
	}

	writePlainFile(t, a, []byte("AAAA"))
	if l.HasEntry(a, stat(t, a)) {
		t.Error("a file with a new size should not be done")
	}
	later := time.Now().Add(time.Hour)
	os.Chtimes(b, later, later)
	if l.HasEntry(b, stat(t, b)) {
		t.Error("a file with a new modification time should not be done")
	}
}

// Tests that entries for files that no longer exist are removed and the ledger rewritten.
func TestLedgerRemoveMissing(t *testing.T) {
	td := ledgerCache(t)
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.tar"), filepath.Join(dir, "b.tar")
	writePlainFile(t, a, []byte("a"))
	writePlainFile(t, b, []byte("b"))
	l := OpenLedger(td)
	l.Record(a, stat(t, a), ResultLoaded)
	l.Record(b, stat(t, b), ResultLoaded)
	os.Remove(b)
	if err := l.RemoveMissing(); err != nil {
		t.Fatal(err)
	}
	if entries := OpenLedger(td).Entries(); len(entries) != 1 || entries[0].Name != a {
		t.Fatalf("expected only %s in the rewritten ledger, got %+v", a, entries)
	}
}

// Tests that a ledger that can't be parsed, has an unknown key, or a newer version is logged
// as an error and treated as empty.
func TestLedgerDamaged(t *testing.T) {
	hook := test.NewGlobal()
	defer log.StandardLogger().ReplaceHooks(make(log.LevelHooks))
	for _, content := range []string{"not: [yaml", "version: 1\nfiles: []\nextra: x\n", "version: 2\nfiles: []\n", "files: []\n"} {
		td := ledgerCache(t)
		os.WriteFile(filepath.Join(td, globals.MetadataPath, LedgerFile), []byte(content), 0644)
		hook.Reset()
		if l := OpenLedger(td); len(l.Entries()) != 0 {
			t.Errorf("%q: expected an empty ledger", content)
		}
		if e := hook.LastEntry(); e == nil || e.Level != log.ErrorLevel || !strings.Contains(e.Message, "processed again") {
			t.Errorf("%q: expected an error log, got %v", content, e)
		}
	}
}

// Tests that a nil ledger, as Load uses, has nothing and records nothing.
func TestLedgerNil(t *testing.T) {
	var l *Ledger
	path := filepath.Join(t.TempDir(), "a.tar")
	writePlainFile(t, path, []byte("a"))
	if l.HasEntry(path, stat(t, path)) || l.Record(path, stat(t, path), ResultLoaded) != nil || l.RemoveMissing() != nil || l.Entries() != nil {
		t.Fatal("a nil ledger should do nothing")
	}
}

// Tests Load: every file is tried, so a failing list file processed first doesn't stop an
// archive loading; it returns the number that failed, with no error; and it uses no ledger,
// so loading again loads again.
func TestLoadTriesEveryFile(t *testing.T) {
	td := ledgerCache(t)
	dir := t.TempDir()
	writePlainFile(t, filepath.Join(dir, "a-list.txt"), []byte("not a valid image url\n"))
	if err := mock.DockerSaveTarball(filepath.Join(dir, "app.tar"), mock.TarImage{Ref: "quay.io/org/app:v1", Os: "linux", Arch: "amd64", Layers: []string{"a layer"}}); err != nil {
		t.Fatal(err)
	}
	failed, err := Load(dir, "")
	if err != nil || failed != 1 {
		t.Fatalf("expected 1 failed file and no error, got %d %v", failed, err)
	}
	if _, found := serialize.MhFromFilesystem("quay.io/org/app:v1", td); !found {
		t.Fatal("the archive should be loaded despite the failed list file")
	}
	if _, err := os.Stat(filepath.Join(td, globals.MetadataPath, LedgerFile)); err == nil {
		t.Fatal("Load must not write a ledger")
	}
	if failed, err := Load(filepath.Join(t.TempDir(), "none-*.tar"), ""); err != nil || failed != 0 {
		t.Fatalf("a glob that matches nothing should be no error and no failures, got %d %v", failed, err)
	}
	if _, err := Load(filepath.Join(dir, "missing.tar"), ""); err == nil {
		t.Fatal("a path that doesn't exist should be an error")
	}
}

// Tests that Preload counts a file it can't read as failed but doesn't record it, so it's tried
// again once readable: fixing permissions doesn't change a file's size or modification time.
// (Root can read any file, so this only runs as another user.)
func TestPreloadUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read any file")
	}
	td := ledgerCache(t)
	dir := t.TempDir()
	tarball := filepath.Join(dir, "app.tar")
	if err := mock.DockerSaveTarball(tarball, mock.TarImage{Ref: "quay.io/org/app:v1", Os: "linux", Arch: "amd64", Layers: []string{"a layer"}}); err != nil {
		t.Fatal(err)
	}
	os.Chmod(tarball, 0)
	if failed, err := Preload(dir, ""); err != nil || failed != 1 || len(OpenLedger(td).Entries()) != 0 {
		t.Fatalf("expected 1 failed file and no ledger entry, got %d %v %+v", failed, err, OpenLedger(td).Entries())
	}
	os.Chmod(tarball, 0644)
	if failed, err := Preload(dir, ""); err != nil || failed != 0 || len(OpenLedger(td).Entries()) != 1 {
		t.Fatalf("expected the now readable file loaded and recorded, got %d %v", failed, err)
	}
}

// Tests Preload: an archive is loaded and a list file that fails is recorded failed without
// stopping the rest; a restart processes nothing the ledger has; a changed file is processed
// again (skipped: its image is cached); and a path that matches nothing is not an error.
func TestPreloadLedger(t *testing.T) {
	td := ledgerCache(t)
	dir := t.TempDir()
	tarball := filepath.Join(dir, "app.tar")
	if err := mock.DockerSaveTarball(tarball, mock.TarImage{Ref: "quay.io/org/app:v1", Os: "linux", Arch: "amd64", Layers: []string{"a layer"}}); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(dir, "a-list.txt") // sorts first: a failure must not stop the rest
	writePlainFile(t, list, []byte("not a valid image url\n"))
	results := func() map[string]LedgerResult {
		r := map[string]LedgerResult{}
		for _, e := range OpenLedger(td).Entries() {
			r[filepath.Base(e.Name)] = e.Result
		}
		return r
	}

	if _, err := Preload(dir, ""); err != nil {
		t.Fatal(err)
	}
	if r := results(); r["app.tar"] != ResultLoaded || r["a-list.txt"] != ResultFailed || len(r) != 2 {
		t.Fatalf("expected app.tar loaded and a-list.txt failed, got %v", r)
	}
	if _, found := serialize.MhFromFilesystem("quay.io/org/app:v1", td); !found {
		t.Fatal("the archive's image should be cached")
	}

	// as if a prune removed the image: a restart must not load it again
	os.RemoveAll(filepath.Join(td, globals.ManifestPath))
	os.Mkdir(filepath.Join(td, globals.ManifestPath), 0755)
	before := OpenLedger(td).Entries()
	if _, err := Preload(dir, ""); err != nil {
		t.Fatal(err)
	}
	if _, found := serialize.MhFromFilesystem("quay.io/org/app:v1", td); found {
		t.Fatal("a ledgered archive must not be loaded again")
	}
	if after := OpenLedger(td).Entries(); after[0].Processed != before[0].Processed {
		t.Fatal("a ledgered file must not be processed again")
	}

	// a new version of the archive (same image) is processed again
	later := time.Now().Add(time.Hour)
	os.Chtimes(tarball, later, later)
	if _, err := Preload(dir, ""); err != nil {
		t.Fatal(err)
	}
	if r := results(); r["app.tar"] != ResultLoaded {
		t.Fatalf("expected the changed archive to be loaded again, got %v", r)
	}
	if _, err := Preload(dir, ""); err != nil || results()["app.tar"] != ResultLoaded {
		t.Fatal("an unchanged archive should keep its entry")
	}
	os.Chtimes(tarball, later.Add(time.Hour), later.Add(time.Hour))
	if _, err := Preload(dir, ""); err != nil || results()["app.tar"] != ResultSkipped {
		t.Fatalf("expected a changed archive whose image is cached to be skipped, got %v", results())
	}

	if _, err := Preload(filepath.Join(t.TempDir(), "none-*.tar"), ""); err != nil {
		t.Fatalf("a path that matches nothing should not be an error, got %v", err)
	}
	if _, err := Preload("[", ""); err == nil {
		t.Fatal("a malformed glob should be an error")
	}
}
