package preload

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// --- shared fixture helpers ---

func writeRealTar(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating %q: %s", path, err)
	}
	defer f.Close()
	tw := tar.NewWriter(f)
	defer tw.Close()
	content := []byte("hello world")
	hdr := &tar.Header{Name: "hello.txt", Mode: 0644, Size: int64(len(content))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("writing tar header: %s", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("writing tar content: %s", err)
	}
}

func writeGzipFile(t *testing.T, path string, content []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating %q: %s", path, err)
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	defer gw.Close()
	if _, err := gw.Write(content); err != nil {
		t.Fatalf("writing gzip content: %s", err)
	}
}

func writePlainFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("writing %q: %s", path, err)
	}
}

// --- looksLikeTar ---

func TestLooksLikeTar_RealTar(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "real.tar")
	writeRealTar(t, path)

	isTar, err := looksLikeTar(path)
	if err != nil {
		t.Fatalf("looksLikeTar: %s", err)
	}
	if !isTar {
		t.Fatalf("expected a real tar file to be detected as a tar")
	}
}

func TestLooksLikeTar_Gzip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "real.tar.gz")
	// Content doesn't need to be a real tar for this test - looksLikeTar
	// returns true on gzip magic bytes alone, without decompressing to
	// look further (see its doc comment - full validation is LoadTarball's
	// job, via the real tarball reader).
	writeGzipFile(t, path, []byte("anything at all"))

	isTar, err := looksLikeTar(path)
	if err != nil {
		t.Fatalf("looksLikeTar: %s", err)
	}
	if !isTar {
		t.Fatalf("expected a gzip-compressed file to be detected as a (possible) tar")
	}
}

func TestLooksLikeTar_PlainTextList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "list.txt")
	writePlainFile(t, path, []byte("registry.k8s.io/pause:3.10.2\nregistry.k8s.io/pause:3.9\n"))

	isTar, err := looksLikeTar(path)
	if err != nil {
		t.Fatalf("looksLikeTar: %s", err)
	}
	if isTar {
		t.Fatalf("expected a plain text image list to NOT be detected as a tar")
	}
}

func TestLooksLikeTar_ShortFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "short.txt")
	// Shorter than offset 257+5 - exercises the "too short to even have a
	// tar header" path distinctly from an ordinary-length non-tar file.
	writePlainFile(t, path, []byte("x"))

	isTar, err := looksLikeTar(path)
	if err != nil {
		t.Fatalf("looksLikeTar: %s", err)
	}
	if isTar {
		t.Fatalf("expected a short file to NOT be detected as a tar")
	}
}

func TestLooksLikeTar_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	writePlainFile(t, path, []byte{})

	isTar, err := looksLikeTar(path)
	if err != nil {
		t.Fatalf("looksLikeTar: %s", err)
	}
	if isTar {
		t.Fatalf("expected an empty file to NOT be detected as a tar")
	}
}

func TestLooksLikeTar_NonexistentFile(t *testing.T) {
	if _, err := looksLikeTar("/does/not/exist/anywhere"); err == nil {
		t.Fatalf("expected an error for a nonexistent path")
	}
}

// --- expandPaths ---

func TestExpandPaths_SinglePlainFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.tar")
	writePlainFile(t, path, []byte("x"))

	got, err := expandPaths(path)
	if err != nil {
		t.Fatalf("expandPaths: %s", err)
	}
	if len(got) != 1 || got[0] != path {
		t.Fatalf("expected [%q], got %v", path, got)
	}
}

func TestExpandPaths_CommaSeparatedPlainFiles(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.tar")
	b := filepath.Join(dir, "b.tar")
	writePlainFile(t, a, []byte("x"))
	writePlainFile(t, b, []byte("x"))

	got, err := expandPaths(a + "," + b)
	if err != nil {
		t.Fatalf("expandPaths: %s", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 paths, got %d: %v", len(got), got)
	}
}

func TestExpandPaths_WhitespaceAroundCommas(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.tar")
	b := filepath.Join(dir, "b.tar")
	writePlainFile(t, a, []byte("x"))
	writePlainFile(t, b, []byte("x"))

	got, err := expandPaths(a + " , " + b)
	if err != nil {
		t.Fatalf("expandPaths: %s", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 paths (whitespace around commas should be trimmed), got %d: %v", len(got), got)
	}
}

func TestExpandPaths_Glob(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"multi-1.tar", "multi-2.tar", "other.txt"} {
		writePlainFile(t, filepath.Join(dir, n), []byte("x"))
	}

	got, err := expandPaths(filepath.Join(dir, "multi-*.tar"))
	if err != nil {
		t.Fatalf("expandPaths: %s", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 matches for multi-*.tar, got %d: %v", len(got), got)
	}
}

func TestExpandPaths_MixedPlainAndGlob(t *testing.T) {
	dir := t.TempDir()
	solo := filepath.Join(dir, "solo.tar")
	writePlainFile(t, solo, []byte("x"))
	for _, n := range []string{"xyz-1.tar", "xyz-2.tar"} {
		writePlainFile(t, filepath.Join(dir, n), []byte("x"))
	}

	got, err := expandPaths(solo + "," + filepath.Join(dir, "xyz-*.tar"))
	if err != nil {
		t.Fatalf("expandPaths: %s", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 paths (1 plain + 2 glob matches), got %d: %v", len(got), got)
	}
}

func TestExpandPaths_NoMatches(t *testing.T) {
	dir := t.TempDir()
	if _, err := expandPaths(filepath.Join(dir, "nonexistent-*.tar")); err == nil {
		t.Fatalf("expected an error when a glob matches nothing")
	}
}

func TestExpandPaths_InvalidGlobPattern(t *testing.T) {
	// An unmatched '[' is a malformed pattern per filepath.Match's syntax
	// rules, which filepath.Glob uses internally.
	if _, err := expandPaths("["); err == nil {
		t.Fatalf("expected an error for a malformed glob pattern")
	}
}

func TestExpandPaths_TrailingCommaIgnored(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.tar")
	writePlainFile(t, a, []byte("x"))

	// A trailing (or doubled) comma should not produce a spurious empty
	// segment that errors as "no matches".
	got, err := expandPaths(a + ",")
	if err != nil {
		t.Fatalf("expandPaths: %s", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 path, got %d: %v", len(got), got)
	}
}
