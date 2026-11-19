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
// tarballs told from list files by content. With TarballsOnly only the tarballs.
func TestResolveFilesDirectory(t *testing.T) {
	dir := resolveFixture(t)
	files, err := ResolveFiles(dir, ResolveOpts{})
	if want := []string{"a.tar*", "b.tgz*", "link.txt", "list.txt"}; err != nil || !slices.Equal(names(files), want) {
		t.Fatalf("expected %v, got %v %v", want, names(files), err)
	}
	for _, f := range files {
		if !filepath.IsAbs(f.Path) {
			t.Errorf("expected an absolute path, got %q", f.Path)
		}
	}
	files, err = ResolveFiles(dir, ResolveOpts{TarballsOnly: true})
	if want := []string{"a.tar*", "b.tgz*"}; err != nil || !slices.Equal(names(files), want) {
		t.Fatalf("tarballs only: expected %v, got %v %v", want, names(files), err)
	}
}

// Tests that a file named more than once resolves once, in the position it first appears, and
// that a relative path resolves to an absolute one.
func TestResolveFilesDedupAndRelative(t *testing.T) {
	dir := resolveFixture(t)
	files, err := ResolveFiles(filepath.Join(dir, "list.txt")+","+dir+","+filepath.Join(dir, "*.tar"), ResolveOpts{})
	if want := []string{"list.txt", "a.tar*", "b.tgz*", "link.txt"}; err != nil || !slices.Equal(names(files), want) {
		t.Fatalf("expected %v, got %v %v", want, names(files), err)
	}
	t.Chdir(dir)
	files, err = ResolveFiles("a.tar", ResolveOpts{})
	if err != nil || len(files) != 1 || files[0].Path != filepath.Join(dir, "a.tar") {
		t.Fatalf("expected the absolute path of a.tar, got %v %v", files, err)
	}
}

// Tests that a .part file named explicitly is skipped, and that matching nothing is an error
// unless AllowNoMatch, while an empty directory is never an error.
func TestResolveFilesNoMatch(t *testing.T) {
	dir := resolveFixture(t)
	if files, err := ResolveFiles(filepath.Join(dir, "c.tar.part"), ResolveOpts{}); err != nil || len(files) != 0 {
		t.Errorf("expected a .part file to be skipped, got %v %v", files, err)
	}
	glob := filepath.Join(dir, "none-*.tar")
	if _, err := ResolveFiles(glob, ResolveOpts{}); err == nil {
		t.Error("expected an error for a glob that matches nothing")
	}
	if files, err := ResolveFiles(glob, ResolveOpts{AllowNoMatch: true}); err != nil || len(files) != 0 {
		t.Errorf("AllowNoMatch: expected nothing and no error, got %v %v", files, err)
	}
	if files, err := ResolveFiles(t.TempDir(), ResolveOpts{}); err != nil || len(files) != 0 {
		t.Errorf("empty directory: expected nothing and no error, got %v %v", files, err)
	}
}
