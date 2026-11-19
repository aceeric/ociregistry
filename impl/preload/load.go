package preload

import (
	log "github.com/sirupsen/logrus"
)

// Load dispatches to LoadFromListFile or LoadTarball for each file resolved
// from path (see ResolveFiles). path may be:
//   - a single plain file
//   - a directory: its regular files, one level deep
//   - a glob expression, e.g. "*.tar"
//   - a comma-separated list mixing plain paths and glob expressions, e.g.
//     "multi-1.tar,multi-2.tar,xyz*.tar"
//
// Each resolved file is inspected and routed independently to LoadFromListFile
// or LoadTarball based on the content of the file at path - not its name or
// the CLI flag that supplied it, so the --images or --preload-images args
// can point at any combination of list-of-image files or tarballs by name, glob,
// or combination thereof.
//
// The resolveRefStr arg is the plain --resolve-ref command line value
// (empty string if not supplied); it is only meaningful for the tarball
// case and is ignored (with a warning) if path turns out to contain any image list
// file(s) instead.
func Load(path string, resolveRefStr string) error {
	files, err := ResolveFiles(path, ResolveOpts{})
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.Tarball {
			if err := LoadTarball(f.Path, resolveRefStr); err != nil {
				return err
			}
			continue
		}
		if resolveRefStr != "" {
			log.Warnf("--resolve-ref %q was supplied for %q but it looks like a text image list, not a tarball - ignoring it", resolveRefStr, f.Path)
		}
		if err := LoadFromListFile(f.Path); err != nil {
			return err
		}
	}
	return nil
}
