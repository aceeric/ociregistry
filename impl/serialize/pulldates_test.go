package serialize

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/aceeric/ociregistry/impl/globals"
)

const (
	older = "2026-11-01T10:00:00"
	newer = "2026-11-09T17:09:25"
)

// Tests a write, read back and removal of the pull dates file, a missing file, and that the file
// is indented with sorted urls.
func TestPullDatesRoundTrip(t *testing.T) {
	td := t.TempDir()
	if err := CreateDirs(td, true); err != nil {
		t.Fatal(err)
	}
	if meta, err := ReadPullDates(td); err != nil || len(meta) != 0 {
		t.Fatalf("expected an empty map for a missing file, got %v %v", meta, err)
	}
	want := map[string]UrlMeta{
		"quay.io/org/app:v1":                   {Pulled: "2026-11-10T08:41:02"},
		"docker.io/library/hello-world:latest": {Pulled: newer},
	}
	if err := WritePullDates(td, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPullDates(td)
	if err != nil || !maps.Equal(got, want) {
		t.Fatalf("expected %v, got %v %v", want, got, err)
	}
	b, _ := os.ReadFile(pullDatesFile(td))
	s := string(b)
	if !strings.HasPrefix(s, "{\n  \"version\": 1,\n  \"meta\": {\n") || strings.Index(s, "docker.io") > strings.Index(s, "quay.io") {
		t.Fatalf("expected indented output with sorted urls, got:\n%s", s)
	}
	if err := RemovePullDates(td); err != nil {
		t.Fatal(err)
	}
	if err := RemovePullDates(td); err != nil {
		t.Fatalf("removing a missing file should not be an error: %s", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(td, globals.ManifestPath)); len(entries) != 0 {
		t.Fatalf("expected an empty manifests directory, got %v", entries)
	}
}

// Tests that a pull dates file that can't be parsed is an error, and that a newer version is
// ErrPullDatesVersion.
func TestPullDatesInvalid(t *testing.T) {
	tests := []struct {
		content    string
		wantNewer  bool
		wantSubstr string
	}{
		{"not json", false, "can't parse"},
		{`{"meta": {}}`, false, "missing or invalid version"},
		{`{"version": 2, "meta": {}}`, true, "version 2"},
	}
	for _, tst := range tests {
		td := t.TempDir()
		CreateDirs(td, true)
		os.WriteFile(pullDatesFile(td), []byte(tst.content), 0644)
		_, err := ReadPullDates(td)
		if err == nil || !strings.Contains(err.Error(), tst.wantSubstr) || errors.Is(err, ErrPullDatesVersion) != tst.wantNewer {
			t.Errorf("%q: expected an error containing %q (newer version: %t), got %v", tst.content, tst.wantSubstr, tst.wantNewer, err)
		}
	}
}

// Tests that the pull dates file is not walked as a manifest.
func TestWalkSkipsPullDates(t *testing.T) {
	td := t.TempDir()
	CreateDirs(td, true)
	WritePullDates(td, map[string]UrlMeta{"docker.io/library/hello-world:latest": {Pulled: newer}})
	n := 0
	WalkTheCache(td, func(_ imgpull.ManifestHolder, _ os.FileInfo) error {
		n++
		return nil
	})
	if n != 0 {
		t.Fatalf("expected the pull dates file to be skipped, walked %d", n)
	}
}

// reconcileFixture writes app:v1 and app:v2 manifests pulled at the passed dates to a new cache,
// and returns its path.
func reconcileFixture(t *testing.T, v1Pulled, v2Pulled string) string {
	t.Helper()
	td := t.TempDir()
	CreateDirs(td, true)
	for url, pulled := range map[string]string{"quay.io/org/app:v1": v1Pulled, "quay.io/org/app:v2": v2Pulled} {
		mh := imgpull.ManifestHolder{Type: imgpull.V1ociManifest, ImageUrl: url, Digest: strings.Repeat("d", 64), Created: older, Pulled: pulled}
		if err := MhToFilesystem(mh, td, true); err != nil {
			t.Fatal(err)
		}
	}
	return td
}

// Tests applying a leftover pull dates file: a later date is written to the manifest, an
// earlier one is ignored, a url without a manifest isn't created, and the file is deleted.
// Applying it again changes nothing, and with no file there's nothing to do.
func TestReconcilePullDates(t *testing.T) {
	td := reconcileFixture(t, older, newer)
	if err := ReconcilePullDates(td, MhToFilesystem); err != nil {
		t.Fatalf("no file: %s", err)
	}
	journal := map[string]UrlMeta{
		"quay.io/org/app:v1":  {Pulled: newer}, // later than the manifest: applied
		"quay.io/org/app:v2":  {Pulled: older}, // earlier than the manifest: ignored
		"quay.io/org/gone:v1": {Pulled: newer}, // no manifest: skipped
	}
	WritePullDates(td, journal)
	if err := ReconcilePullDates(td, MhToFilesystem); err != nil {
		t.Fatal(err)
	}
	check := func() {
		t.Helper()
		for url, want := range map[string]string{"quay.io/org/app:v1": newer, "quay.io/org/app:v2": newer} {
			if mh, _ := MhFromFilesystem(url, td); mh.Pulled != want {
				t.Errorf("%s: expected pulled %s, got %s", url, want, mh.Pulled)
			}
		}
		if _, found := MhFromFilesystem("quay.io/org/gone:v1", td); found {
			t.Error("a manifest must not be created for a url without one")
		}
		if _, err := os.Stat(pullDatesFile(td)); !os.IsNotExist(err) {
			t.Error("the pull dates file should be deleted")
		}
	}
	check()
	// e.g. a crash before the file was deleted: applying it again gives the same result
	WritePullDates(td, journal)
	if err := ReconcilePullDates(td, MhToFilesystem); err != nil {
		t.Fatal(err)
	}
	check()
}

// Tests that a pull dates file that can't be used, or a manifest that can't be written, is an
// error that keeps the file.
func TestReconcileErrors(t *testing.T) {
	for _, content := range []string{"not json", `{"version": 2, "meta": {}}`} {
		td := reconcileFixture(t, older, older)
		os.WriteFile(pullDatesFile(td), []byte(content), 0644)
		err := ReconcilePullDates(td, MhToFilesystem)
		if err == nil || !strings.Contains(err.Error(), "delete it") {
			t.Errorf("%q: expected an error saying the file can be deleted, got %v", content, err)
		}
		if b, _ := os.ReadFile(pullDatesFile(td)); string(b) != content {
			t.Errorf("%q: the file should be kept", content)
		}
	}

	failingWrite := func(imgpull.ManifestHolder, string, bool) error { return errors.New("no space left on device") }
	td := reconcileFixture(t, older, older)
	WritePullDates(td, map[string]UrlMeta{"quay.io/org/app:v1": {Pulled: newer}})
	if err := ReconcilePullDates(td, failingWrite); err == nil || !strings.Contains(err.Error(), "no space left on device") {
		t.Fatalf("expected the write error, got %v", err)
	}
	if meta, _ := ReadPullDates(td); meta["quay.io/org/app:v1"].Pulled != newer {
		t.Fatal("the file should be kept when a manifest can't be written")
	}
}
