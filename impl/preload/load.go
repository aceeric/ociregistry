package preload

import (
	"fmt"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
)

// Load dispatches to LoadFromListFile or LoadTarball based on the content
// of the file at path - not its name or the CLI flag that supplied it, so
// either --image-file or --preload-images can point at either a plain-text
// list of image URLs (one per line, handled by LoadFromListFile) or an
// OCI-layout/docker-save tarball, optionally gzip-compressed (handled by
// LoadTarball). resolveRefStr is the plain --resolve-ref command line value
// (empty string if not supplied); it is only meaningful for the tarball
// case - see newFixedRefResolver - and is ignored (with a warning) if path
// turns out to be a text list instead.
func Load(path string, resolveRefStr string) error {
	isTar, err := looksLikeTar(path)
	if err != nil {
		return fmt.Errorf("error inspecting %q: %w", path, err)
	}
	if isTar {
		// create a resolver that ALWAYS returns the override in this case - the
		// operator is responsible to know that a tarball is a particular image
		// and must know the correct image ref
		return LoadTarball(path, newFixedRefResolver(resolveRefStr))
	}
	if resolveRefStr != "" {
		log.Warnf("--resolve-ref %q was supplied for %q but it looks like a text image list, not a tarball - ignoring it", resolveRefStr, path)
	}
	return LoadFromListFile(path)
}

// newFixedRefResolver returns a RefResolver that always returns
// resolveRefStr, ignoring whatever candidate/digest ref the tarball itself
// offered - or nil if resolveRefStr is empty, in which case LoadTarball's
// ordinary "accept an already-valid candidate as-is, otherwise skip"
// behavior applies (see resolveOrDefault). Because a non-nil result here
// always overrides rather than ever deferring to candidate, guardResolver
// will reject its use against a tarball containing more than one image -
// which is the intended effect: --resolve-ref is the operator asserting
// "this tarball is exactly one specific image, and here is its real ref,"
// a claim that only makes sense for a single-image tarball.
func newFixedRefResolver(resolveRefStr string) RefResolver {
	if resolveRefStr == "" {
		return nil
	}
	return func(candidate string, digest string) string {
		return resolveRefStr
	}
}

// looksLikeTar reports whether the file at path is (or gzip-decompresses
// to) a tar archive, sniffed by content rather than file extension:
//   - gzip magic bytes (0x1f 0x8b) at the very start, in which case this
//     assumes a compressed tarball without decompressing to look further -
//     LoadTarball's own ensurePlainTar will decompress it for real and
//     surface a clear error if the result isn't actually a valid tar/image
//     tarball.
//   - otherwise, the "ustar" magic a tar header carries at byte offset 257,
//     which covers both POSIX ustar ("ustar\0") and GNU tar ("ustar  ")
//     variants, and is what every realistic producer here writes (Go's
//     archive/tar, GNU tar, docker save, ctr image export, crane).
//
// A hand-crafted legacy v7-format tar with no ustar magic at all would not
// be detected by the second check and would fall through to
// LoadFromListFile, which would then fail on it as an unparseable image
// list - a clear enough failure mode for what should be a vanishingly rare
// input in practice, but worth knowing about if it ever comes up.
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
