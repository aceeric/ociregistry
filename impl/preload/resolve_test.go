package preload

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// resolveFixture makes a directory with two tarballs (plain and gzip), an image list file,
// a tarball still being written (.part), a subdirectory, a symlink to the list file and a
// symlink to the subdirectory, and returns its path.
func resolveFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeRealTar(t, filepath.Join(dir, "a.tar"))
	writeGzipFile(t, filepath.Join(dir, "b.tgz"), []byte("anything"))
	writePlainFile(t, filepath.Join(dir, "list.txt"), []byte("quay.io/org/app:v1\n"))
	writeRealTar(t, filepath.Join(dir, "c.tar.part"))
	os.Mkdir(filepath.Join(dir, "sub"), 0755)
	writeRealTar(t, filepath.Join(dir, "sub", "nested.tar"))
	os.Symlink(filepath.Join(dir, "list.txt"), filepath.Join(dir, "link.txt"))
	os.Symlink(filepath.Join(dir, "sub"), filepath.Join(dir, "sublink"))
	return dir
}

// names returns the base names of the resolved files, with a "*" suffix for tarballs.
func names(files []ResolvedFile) []string {
	n := []string{}
	for _, f := range files {
		name := filepath.Base(f.Path)
		if f.Tarball {
			name += "*"
		}
		n = append(n, name)
	}
	return n
}

// Tests resolving a directory: its regular files one level deep in name order, symlinks to
// files followed, .part files, subdirectories and symlinks to directories skipped, and
// tarballs told from list files by content.
func TestResolveFilesDirectory(t *testing.T) {
	dir := resolveFixture(t)
	files, err := ResolveFiles(dir)
	if want := []string{"a.tar*", "b.tgz*", "link.txt", "list.txt"}; err != nil || !slices.Equal(names(files), want) {
		t.Fatalf("expected %v, got %v %v", want, names(files), err)
	}
	for _, f := range files {
		if !filepath.IsAbs(f.Path) {
			t.Errorf("expected an absolute path, got %q", f.Path)
		}
	}
}

// Tests that a file named more than once resolves once, in the position it first appears, and
// that a relative path resolves to an absolute one.
func TestResolveFilesDedupAndRelative(t *testing.T) {
	dir := resolveFixture(t)
	files, err := ResolveFiles(filepath.Join(dir, "list.txt") + "," + dir + "," + filepath.Join(dir, "*.tar"))
	if want := []string{"list.txt", "a.tar*", "b.tgz*", "link.txt"}; err != nil || !slices.Equal(names(files), want) {
		t.Fatalf("expected %v, got %v %v", want, names(files), err)
	}
	t.Chdir(dir)
	files, err = ResolveFiles("a.tar")
	if err != nil || len(files) != 1 || files[0].Path != filepath.Join(dir, "a.tar") {
		t.Fatalf("expected the absolute path of a.tar, got %v %v", files, err)
	}
}

// Tests that a .part file named explicitly is skipped, that a glob that matches nothing and an
// empty directory resolve to nothing, and that a path that doesn't exist, a directory that
// can't be read, or a malformed glob is an error.
func TestResolveFilesNoMatch(t *testing.T) {
	dir := resolveFixture(t)
	if files, err := ResolveFiles(filepath.Join(dir, "c.tar.part")); err != nil || len(files) != 0 {
		t.Errorf("expected a .part file to be skipped, got %v %v", files, err)
	}
	for _, spec := range []string{filepath.Join(dir, "none-*.tar"), t.TempDir()} {
		if files, err := ResolveFiles(spec); err != nil || len(files) != 0 {
			t.Errorf("%s: expected nothing and no error, got %v %v", spec, files, err)
		}
	}
	for _, spec := range []string{filepath.Join(dir, "missing.tar"), filepath.Join(dir, "missing-dir"), dir + ",/no/such/dir", "["} {
		if _, err := ResolveFiles(spec); err == nil {
			t.Errorf("%s: expected an error", spec)
		}
	}
	if os.Geteuid() != 0 {
		locked := filepath.Join(t.TempDir(), "locked")
		os.Mkdir(locked, 0)
		defer os.Chmod(locked, 0755)
		if _, err := ResolveFiles(locked); err == nil {
			t.Error("expected an error for a directory that can't be read")
		}
	}
}

// Tests that a file that can't be read is resolved with Err set, and doesn't stop the others.
// (Root can read any file, so this only runs as another user.)
func TestResolveFilesUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read any file")
	}
	dir := resolveFixture(t)
	os.Chmod(filepath.Join(dir, "list.txt"), 0)
	defer os.Chmod(filepath.Join(dir, "list.txt"), 0644)
	files, err := ResolveFiles(dir)
	if err != nil || len(files) != 4 {
		t.Fatalf("expected all 4 files, got %v %v", files, err)
	}
	for _, f := range files {
		// link.txt is a symlink to list.txt, so it can't be read either
		if name := filepath.Base(f.Path); (name == "list.txt" || name == "link.txt") != (f.Err != nil) {
			t.Errorf("%s: unexpected Err %v", f.Path, f.Err)
		}
	}
}
