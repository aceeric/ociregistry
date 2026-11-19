package preload

import (
	"os"

	"github.com/aceeric/ociregistry/impl/config"
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
			if _, err := LoadTarball(f.Path, resolveRefStr); err != nil {
				return err
			}
			continue
		}
		if resolveRefStr != "" {
			log.Warnf("--resolve-ref %q was supplied for %q but it looks like a text image list, not a tarball - ignoring it", resolveRefStr, f.Path)
		}
		if _, err := LoadFromListFile(f.Path); err != nil {
			return err
		}
	}
	return nil
}

// Preload loads the files resolved from path into the file system cache when the server
// starts, like Load, but recording each file in the cache's ledger (see Ledger) so that a
// file is processed once per version: a file the ledger already has is skipped, so a restart
// doesn't load again what's been loaded, or what a prune has since removed. Unlike Load:
//   - path matching no files is logged at info, not an error, since files may arrive later
//     (see the watcher)
//   - a file that fails is logged as an error and recorded as failed, and the rest are still
//     processed; it's not processed again until it changes or the ledger is edited
//   - ledger entries for files that no longer exist are removed
//
// The error returned is for a path that can't be resolved, such as a malformed glob.
func Preload(path string, resolveRefStr string) error {
	files, err := ResolveFiles(path, ResolveOpts{AllowNoMatch: true})
	if err != nil {
		return err
	}
	if len(files) == 0 {
		log.Infof("no files to preload in %q", path)
	}
	ledger := OpenLedger(config.GetImagePath())
	if err := ledger.RemoveMissing(); err != nil {
		log.Errorf("error updating the preload ledger: %s", err)
	}
	for _, f := range files {
		preloadFile(ledger, f, resolveRefStr)
	}
	return nil
}

// preloadFile processes one file for Preload, unless the ledger has it, and records the
// result in the ledger.
func preloadFile(ledger *Ledger, f ResolvedFile, resolveRefStr string) {
	fi, err := os.Stat(f.Path)
	if err != nil {
		log.Errorf("error preloading %q: %s", f.Path, err)
		return
	}
	if ledger.Done(f.Path, fi) {
		log.Debugf("already processed, per the preload ledger: %q", f.Path)
		return
	}
	var cnt int
	if f.Tarball {
		cnt, err = LoadTarball(f.Path, resolveRefStr)
	} else {
		if resolveRefStr != "" {
			log.Warnf("--resolve-ref %q was supplied for %q but it looks like a text image list, not a tarball - ignoring it", resolveRefStr, f.Path)
		}
		cnt, err = LoadFromListFile(f.Path)
	}
	result := ResultLoaded
	switch {
	case err != nil:
		log.Errorf("error preloading %q (it won't be processed again until it changes): %s", f.Path, err)
		result = ResultFailed
	case cnt == 0:
		result = ResultSkipped
	}
	if err := ledger.Record(f.Path, fi, result); err != nil {
		log.Errorf("error updating the preload ledger: %s", err)
	}
}
