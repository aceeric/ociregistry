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
// tarball (by content) or otherwise an image list file.
type ResolvedFile struct {
	Path    string
	Tarball bool
}

// ResolveOpts are the options for ResolveFiles. TarballsOnly drops files that aren't
// tarballs. AllowNoMatch makes a path or glob that matches nothing resolve to nothing rather
// than be an error.
type ResolveOpts struct {
	TarballsOnly bool
	AllowNoMatch bool
}

// ResolveFiles resolves the passed path spec to the files to load, in order, each once. The
// spec is a comma-separated list of any of: a file, a directory (its entries, one level
// deep), or a glob expression. Each entry is a file only if it's a regular file, following
// symlinks (so the files of a mounted Kubernetes ConfigMap or Secret resolve), and its name
// doesn't end in .part. Each file is inspected by content to tell a tarball from an image list
// (see looksLikeTar). Paths are returned absolute and cleaned. It's an error if a segment is
// a malformed glob, or (unless opts.AllowNoMatch) matches nothing. A directory with no files
// matches, so it's not an error.
func ResolveFiles(spec string, opts ResolveOpts) ([]ResolvedFile, error) {
	paths, err := expandPaths(spec, opts.AllowNoMatch)
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
			return nil, fmt.Errorf("error inspecting %q: %w", abs, err)
		}
		if opts.TarballsOnly && !isTar {
			continue
		}
		files = append(files, ResolvedFile{Path: abs, Tarball: isTar})
	}
	return files, nil
}

// expandPaths splits path on commas and expands each segment: a directory to its entries
// (one level deep, in name order), anything else as a glob pattern via filepath.Glob, which
// also matches a plain path with no wildcard characters literally - so a comma-separated
// list of ordinary filenames (no globs at all) resolves the same way it always did. Segments
// are trimmed of surrounding whitespace so "a.tar, b.tar" works the same as "a.tar,b.tar".
// Returns an error if any segment is a malformed glob pattern, or matches no files unless
// allowNoMatch, rather than silently producing an empty or partial result.
func expandPaths(path string, allowNoMatch bool) ([]string, error) {
	var out []string
	for segment := range strings.SplitSeq(path, ",") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		if fi, err := os.Stat(segment); err == nil && fi.IsDir() {
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
		if len(matches) == 0 && !allowNoMatch {
			return nil, fmt.Errorf("no files matched %q", segment)
		}
		out = append(out, slices.Sorted(slices.Values(matches))...)
	}
	return out, nil
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
