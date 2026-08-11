package preload

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Load dispatches to LoadFromListFile or LoadTarball for each file resolved
// from path. path may be:
//   - a single plain file
//   - a glob expression, e.g. "*.tar"
//   - a comma-separated list mixing plain paths and glob expressions, e.g.
//     "multi-1.tar,multi-2.tar,xyz*.tar"
//
// Each resolved file is inspected and routed independently to LoadFromListFile
// or LoadTarball based on the content of the file at path - not its name or
// the CLI flag that supplied it, so the --image-file or --preload-images args
// can point at any combination of list-of-image files or tarballs by name, glob,
// or combination thereof.
//
// The resolveRefStr arg is the plain --resolve-ref command line value
// (empty string if not supplied); it is only meaningful for the tarball
// case and is ignored (with a warning) if path turns out to contain any image list
// file(s) instead.
func Load(path string, resolveRefStr string) error {
	paths, err := expandPaths(path)
	if err != nil {
		return err
	}
	for _, p := range paths {
		isTar, err := looksLikeTar(p)
		if err != nil {
			return fmt.Errorf("error inspecting %q: %w", p, err)
		}
		if isTar {
			if err := LoadTarball(p, resolveRefStr); err != nil {
				return err
			}
			continue
		}
		if resolveRefStr != "" {
			log.Warnf("--resolve-ref %q was supplied for %q but it looks like a text image list, not a tarball - ignoring it", resolveRefStr, p)
		}
		if err := LoadFromListFile(p); err != nil {
			return err
		}
	}
	return nil
}

// expandPaths splits path on commas and expands each segment as a glob
// pattern via filepath.Glob, which also matches a plain path with no
// wildcard characters literally - so a comma-separated list of ordinary
// filenames (no globs at all) resolves the same way it always did.
// Segments are trimmed of surrounding whitespace so "a.tar, b.tar" works
// the same as "a.tar,b.tar". Returns an error if any segment is a
// malformed glob pattern or matches no files, rather than silently
// producing an empty or partial result.
func expandPaths(path string) ([]string, error) {
	var out []string
	for _, segment := range strings.Split(path, ",") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		matches, err := filepath.Glob(segment)
		if err != nil {
			return nil, fmt.Errorf("invalid path or glob %q: %w", segment, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("no files matched %q", segment)
		}
		out = append(out, matches...)
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
