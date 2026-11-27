package preload

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// partSuffix marks a file that is still being written: a producer writes x.tar.part, then
// renames it to x.tar when it's complete. Such files are never resolved.
const partSuffix = ".part"

// ResolvedFile is a file resolved by ResolveFiles: its absolute path, and whether it's a
// tarball (by content) or otherwise an image list file. Err is set if the file couldn't be
// read to tell which, e.g. for lack of permission: it's then a failure of that file alone.
type ResolvedFile struct {
	Path    string
	Tarball bool
	Err     error
}

// ResolveFiles resolves the passed path spec to the files to load, in order, each once. The
// spec is a comma-separated list of any of: a file, a directory (its entries, one level
// deep), or a glob expression. Each entry is a file only if it's a regular file, following
// symlinks (so the files of a mounted Kubernetes ConfigMap or Secret resolve), and its name
// doesn't end in .part. Each file is inspected by content to tell a tarball from an image list
// (see looksLikeTar); a file that can't be read has Err set. Paths are returned absolute and
// cleaned. A glob that matches nothing, or an empty directory, resolves to nothing. It's an
// error if a segment can't be resolved at all: a path with no glob characters that doesn't
// exist, a directory that can't be read, or a malformed glob.
func ResolveFiles(spec string) ([]ResolvedFile, error) {
	paths, err := expandPaths(spec)
	if err != nil {
		return nil, err
	}
	files := []ResolvedFile{}
	seen := map[string]bool{}
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		if seen[abs] || strings.HasSuffix(abs, partSuffix) {
			continue
		}
		seen[abs] = true
		if fi, err := os.Stat(abs); err != nil || !fi.Mode().IsRegular() {
			continue
		}
		isTar, err := looksLikeTar(abs)
		if err != nil {
			err = fmt.Errorf("unable to read %q: %w", abs, err)
		}
		files = append(files, ResolvedFile{Path: abs, Tarball: isTar, Err: err})
	}
	return files, nil
}

// expandPaths splits path on commas and expands each segment: a directory to its entries
// (one level deep, in name order), a glob expression to its matches (in name order), and any
// other path to itself. Segments are trimmed of surrounding whitespace so "a.tar, b.tar" works
// the same as "a.tar,b.tar". A glob that matches nothing expands to nothing. It's an error if
// a path with no glob characters doesn't exist, a directory can't be read, or a glob is
// malformed: a typo in a path is an error rather than silently loading nothing.
func expandPaths(path string) ([]string, error) {
	var out []string
	for segment := range strings.SplitSeq(path, ",") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		if !isGlob(segment) {
			fi, err := os.Stat(segment)
			if err != nil {
				return nil, err
			} else if !fi.IsDir() {
				out = append(out, segment)
				continue
			}
			entries, err := os.ReadDir(segment)
			if err != nil {
				return nil, err
			}
			for _, e := range entries {
				out = append(out, filepath.Join(segment, e.Name()))
			}
			continue
		}
		matches, err := filepath.Glob(segment)
		if err != nil {
			return nil, fmt.Errorf("invalid path or glob %q: %w", segment, err)
		}
		out = append(out, slices.Sorted(slices.Values(matches))...)
	}
	return out, nil
}

// isGlob returns true if the path has any of the characters special to filepath.Match.
func isGlob(path string) bool {
	return strings.ContainsAny(path, `*?[\`)
}

// looksLikeTar reports whether the file at path is (or gzip-decompresses
// to) a tar archive, sniffed by content rather than file extension:
//   - gzip magic bytes (0x1f 0x8b) at the very start
//   - otherwise, the "ustar" magic a tar header carries at byte offset 257
func looksLikeTar(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	magic := make([]byte, 2)
	if n, _ := io.ReadFull(f, magic); n == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		return true, nil
	}

	if _, err := f.Seek(257, io.SeekStart); err != nil {
		return false, nil // couldn't seek - treat as not a tar
	}
	ustar := make([]byte, 5)
	if _, err := io.ReadFull(f, ustar); err != nil {
		// Includes the file simply being shorter than offset 257+5 (a normal
		// outcome for a small text list file), not just genuine I/O errors.
		return false, nil
	}
	return string(ustar) == "ustar", nil
}
