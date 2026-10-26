package serialize

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aceeric/ociregistry/impl/globals"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

const (
	listDigest  = "1111111111111111111111111111111111111111111111111111111111111111"
	imageDigest = "2222222222222222222222222222222222222222222222222222222222222222"
	otherDigest = "3333333333333333333333333333333333333333333333333333333333333333"
	busyDigest  = "4444444444444444444444444444444444444444444444444444444444444444"
)

// legacyCache writes a cache in the legacy layout (manifests named by digest in lts and img)
// covering every case the migration has to account for, and returns its path and the legacy
// file contents keyed by relative path.
func legacyCache(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	td := t.TempDir()
	files := map[string][]byte{}
	write := func(rel string, b []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(td, filepath.Dir(rel)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(td, rel), b, 0644); err != nil {
			t.Fatal(err)
		}
		files[rel] = b
	}
	mh := func(rel string, h imgpull.ManifestHolder) {
		t.Helper()
		b, err := json.Marshal(h)
		if err != nil {
			t.Fatal(err)
		}
		write(rel, b)
	}
	// preload stored an image list and its image manifest both with the tag url
	mh("lts/"+listDigest, imgpull.ManifestHolder{Type: imgpull.V1ociIndex, Digest: listDigest,
		ImageUrl: "quay.io/org/app:latest", Created: "2026-01-01T00:00:00", Pulled: "2026-02-01T00:00:00"})
	mh("lts/"+imageDigest, imgpull.ManifestHolder{Type: imgpull.V1ociManifest, Digest: imageDigest,
		ImageUrl: "quay.io/org/app:latest", Created: "2026-01-01T00:00:00"})
	// a non-canonical docker.io url
	mh("img/"+busyDigest, imgpull.ManifestHolder{Type: imgpull.V1ociManifest, Digest: busyDigest,
		ImageUrl: "docker.io/busybox:1.36", Created: "2026-03-01T00:00:00", Pulled: "2026-03-02T00:00:00"})
	// the same manifest in both legacy directories
	mh("img/"+otherDigest, imgpull.ManifestHolder{Type: imgpull.V1ociManifest, Digest: otherDigest,
		ImageUrl: "quay.io/org/tool:v2"})
	mh("lts/"+otherDigest, imgpull.ManifestHolder{Type: imgpull.V1ociManifest, Digest: otherDigest,
		ImageUrl: "quay.io/org/tool:v2"})
	// files that can't be migrated
	write("img/"+strings.Repeat("5", 64), []byte("{not json"))
	mh("img/"+strings.Repeat("6", 64), imgpull.ManifestHolder{Type: imgpull.V1ociManifest, Digest: strings.Repeat("6", 64),
		ImageUrl: "not-a-url"})
	// a temp file is not a legacy manifest
	if err := os.WriteFile(filepath.Join(td, "img", "x.123.tmp"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	return td, files
}

func entryFor(t *testing.T, report MigrationReport, file string) MigrationEntry {
	t.Helper()
	for _, e := range report.Entries {
		if e.File == file {
			return e
		}
	}
	t.Fatalf("no report entry for %s", file)
	return MigrationEntry{}
}

// Tests the full migration: every legacy file is accounted for with the expected outcome, the
// manifests are readable in the new layout with their dates, nothing in the legacy directories
// changes, and the report and marker are written.
func TestMigrate(t *testing.T) {
	td, legacy := legacyCache(t)
	if needs, err := NeedsMigration(td); err != nil || !needs {
		t.Fatalf("expected the legacy cache to need migration: %t %v", needs, err)
	}
	report, err := Migrate(td, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Found != 7 || len(report.Entries) != 7 || !report.Complete() {
		t.Fatalf("expected 7 files, all accounted for:\n%s", report.String())
	}
	want := map[string]MigrationOutcome{
		"lts/" + listDigest:              Migrated,
		"lts/" + imageDigest:             Relocated,
		"img/" + busyDigest:              Migrated,
		"img/" + otherDigest:             Migrated,
		"lts/" + otherDigest:             AlreadyPresent,
		"img/" + strings.Repeat("5", 64): Skipped,
		"img/" + strings.Repeat("6", 64): Skipped,
	}
	for file, outcome := range want {
		if e := entryFor(t, report, file); e.Outcome != outcome {
			t.Errorf("%s: expected %s, got %s (%s)", file, outcome, e.Outcome, e.Detail)
		}
	}
	if e := entryFor(t, report, "img/"+strings.Repeat("5", 64)); !strings.HasPrefix(e.Detail, "invalid JSON") {
		t.Errorf("expected an invalid JSON reason, got %q", e.Detail)
	}
	if e := entryFor(t, report, "img/"+strings.Repeat("6", 64)); !strings.Contains(e.Detail, "can't be parsed") {
		t.Errorf("expected an unparseable url reason, got %q", e.Detail)
	}
	if e := entryFor(t, report, "img/"+busyDigest); !strings.Contains(e.Detail, "normalized") {
		t.Errorf("expected the docker.io url to be reported as normalized, got %q", e.Detail)
	}

	// the list keeps the tag, the image manifest is under its digest url, with dates kept
	if mh, found := MhFromFilesystem("quay.io/org/app:latest", td); !found || mh.Digest != listDigest || mh.Pulled != "2026-02-01T00:00:00" {
		t.Errorf("tag should hold the list with its dates: found=%t %+v", found, mh)
	}
	if mh, found := MhFromFilesystem("quay.io/org/app@sha256:"+imageDigest, td); !found || mh.Digest != imageDigest {
		t.Errorf("image manifest should be under its digest url: found=%t", found)
	} else if mh.ImageUrl != "quay.io/org/app@sha256:"+imageDigest {
		t.Errorf("relocated manifest should carry its new url, got %q", mh.ImageUrl)
	}
	if mh, found := MhFromFilesystem("docker.io/library/busybox:1.36", td); !found || mh.ImageUrl != "docker.io/library/busybox:1.36" {
		t.Errorf("busybox should be stored under the canonical url: found=%t %q", found, mh.ImageUrl)
	}

	// the legacy layout is untouched
	for rel, b := range legacy {
		if got, err := os.ReadFile(filepath.Join(td, rel)); err != nil || !bytes.Equal(got, b) {
			t.Errorf("legacy file %s was modified", rel)
		}
	}
	for _, name := range []string{MigrationReportFile, MigrationMarker} {
		if _, err := os.Stat(filepath.Join(td, globals.ManifestPath, name)); err != nil {
			t.Errorf("%s not written: %s", name, err)
		}
	}
	if needs, _ := NeedsMigration(td); needs {
		t.Error("a migrated cache should not need migration")
	}
	if b, _ := os.ReadFile(filepath.Join(td, globals.ManifestPath, MigrationReportFile)); !bytes.Contains(b, []byte("result: complete")) {
		t.Errorf("report should say complete:\n%s", b)
	}
	// the walk sees exactly the four migrated manifests, not the report or marker
	cnt := 0
	WalkTheCache(td, func(imgpull.ManifestHolder, os.FileInfo) error { cnt++; return nil })
	if cnt != 4 {
		t.Errorf("expected 4 manifests in the new layout, got %d", cnt)
	}
}

// Tests that a dry run writes nothing, not even the manifests directory, and reports what
// would happen.
func TestMigrateDryRun(t *testing.T) {
	td, _ := legacyCache(t)
	report, err := Migrate(td, true)
	if err != nil {
		t.Fatal(err)
	}
	if !report.DryRun || report.Count(Migrated) != 3 || report.Count(Relocated) != 1 {
		t.Fatalf("unexpected dry run report:\n%s", report.String())
	}
	if _, err := os.Stat(filepath.Join(td, globals.ManifestPath)); !os.IsNotExist(err) {
		t.Fatal("a dry run must not create the manifests directory")
	}
	if !strings.Contains(report.Summary(), "dry run") {
		t.Errorf("summary should say dry run:\n%s", report.Summary())
	}
}

// Tests that once migrated, a migration does nothing: a manifest pruned after migration is
// not copied back from the legacy directories.
func TestMigrateOnce(t *testing.T) {
	td, _ := legacyCache(t)
	if _, err := Migrate(td, false); err != nil {
		t.Fatal(err)
	}
	pruned, _ := MhFromFilesystem("quay.io/org/tool:v2", td)
	if err := RmManifest(td, pruned); err != nil {
		t.Fatal(err)
	}
	report, err := Migrate(td, false)
	if err != nil {
		t.Fatal(err)
	}
	if !report.AlreadyMigrated || len(report.Entries) != 0 {
		t.Fatalf("expected already migrated:\n%s", report.String())
	}
	if _, found := MhFromFilesystem("quay.io/org/tool:v2", td); found {
		t.Fatal("a pruned manifest was copied back")
	}
}

// Tests that a migration that can't write is incomplete: failures are reported, no marker is
// written, an error is returned, and a later run completes.
func TestMigrateIncomplete(t *testing.T) {
	td, _ := legacyCache(t)
	// a regular file where the manifests directory should be makes every write fail
	if err := os.WriteFile(filepath.Join(td, globals.ManifestPath), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	report, err := Migrate(td, false)
	if err == nil {
		t.Fatal("expected an error")
	}
	if report.Complete() || report.Count(Failed) != 4 {
		t.Fatalf("expected 4 failures:\n%s", report.String())
	}
	if !strings.Contains(report.Summary(), "INCOMPLETE") {
		t.Errorf("summary should say incomplete:\n%s", report.Summary())
	}
	if needs, _ := NeedsMigration(td); !needs {
		t.Fatal("an incomplete migration must still need migration")
	}
	// fix the cause and run again
	if err := os.Remove(filepath.Join(td, globals.ManifestPath)); err != nil {
		t.Fatal(err)
	}
	if report, err := Migrate(td, false); err != nil || !report.Complete() {
		t.Fatalf("expected the retry to complete: %v\n%s", err, report.String())
	}
}

// Tests that a retry after a partial migration counts manifests already written as present.
func TestMigrateRetryAfterPartial(t *testing.T) {
	td, _ := legacyCache(t)
	if err := os.MkdirAll(filepath.Join(td, globals.ManifestPath), 0755); err != nil {
		t.Fatal(err)
	}
	// as if a previous run wrote this one before failing
	if err := MhToFilesystem(imgpull.ManifestHolder{Type: imgpull.V1ociManifest, Digest: otherDigest, ImageUrl: "quay.io/org/tool:v2"}, td, false); err != nil {
		t.Fatal(err)
	}
	report, err := Migrate(td, false)
	if err != nil {
		t.Fatal(err)
	}
	if e := entryFor(t, report, "img/"+otherDigest); e.Outcome != AlreadyPresent {
		t.Fatalf("expected already present, got %s", e.Outcome)
	}
}

// Tests that a new cache doesn't need migration and that migrating it writes nothing.
func TestMigrateNewCache(t *testing.T) {
	td := t.TempDir()
	if err := CreateDirs(td, true); err != nil {
		t.Fatal(err)
	}
	if needs, err := NeedsMigration(td); err != nil || needs {
		t.Fatalf("a new cache should not need migration: %t %v", needs, err)
	}
	report, err := Migrate(td, false)
	if err != nil || report.Found != 0 || report.AlreadyMigrated {
		t.Fatalf("unexpected report: %v\n%s", err, report.String())
	}
	if entries, _ := os.ReadDir(filepath.Join(td, globals.ManifestPath)); len(entries) != 0 {
		t.Fatal("migrating a new cache should write nothing")
	}
	// legacy directories holding only temp files have nothing to migrate either
	if err := os.MkdirAll(filepath.Join(td, globals.ImgPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(td, globals.ImgPath, "x.1.tmp"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if needs, _ := NeedsMigration(td); needs {
		t.Fatal("temp files are not legacy manifests")
	}
}
