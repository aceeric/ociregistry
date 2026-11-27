package preload

import (
	"os"

	"github.com/aceeric/ociregistry/impl/config"
	log "github.com/sirupsen/logrus"
)

// Load implements the 'ociregistry load' sub-command. It delegates to 'preload.load',
// passing a nil ledger which results in a full load from the source.
func Load(path string, resolveRefStr string) (int, error) {
	return load(path, resolveRefStr, nil)
}

// Preload implements the 'ociregistry serve --preload' arg. It also delegates to
// 'preload.load', passing a non-nil ledger which results in loading being performed
// in coordination with a ledger of loads to date, supporting the ability to skip
// already-processed input files.
func Preload(path string, resolveRefStr string) (int, error) {
	log.Infof("preload from %s", path)
	return load(path, resolveRefStr, OpenLedger(config.GetImagePath()))
}

// load supports the Load and Preload shims. Mainly it parses the path arg, and then
// delegates in turn to 'loadFile'. Beyond that it does a little ledger housekeeping,
// maintains counters, logs, and returns stats to the caller.
//
// The path arg can be:
//   - a single plain file
//   - a directory: its regular files, one level deep
//   - a glob expression, e.g. "*.tar"
//   - a comma-separated list mixing all of the above, e.g. "multi-1.tar,/mnt/preload,xyz*.tar"
//
// The path is parsed and each resolved entry with abs path is passed down to
// loadFile.
func load(path string, resolveRefStr string, ledger *Ledger) (int, error) {
	files, err := ResolveFiles(path)
	if err != nil {
		return 0, err
	}
	if len(files) == 0 {
		log.Infof("no files to load in %q", path)
		return 0, nil
	}
	if err := ledger.RemoveMissing(); err != nil {
		log.Errorf("error updating the preload ledger: %s", err)
	}
	counts := map[LedgerResult]int{}
	done := 0
	for _, f := range files {
		result, ok := loadFile(ledger, f, resolveRefStr)
		if !ok {
			done++
			continue
		}
		counts[result]++
	}
	log.Infof("processed %d file(s) in %q: %d loaded, %d skipped, %d failed, %d already in the ledger", len(files), path,
		counts[ResultLoaded], counts[ResultSkipped], counts[ResultFailed], done)
	return counts[ResultFailed], nil
}

// loadFile processes one file. The passed ledger can be nil in which case there
// is no ledger interaction. Otherwise the passed (non-nil) is used to avoid re-
// loading.
//
// loadFile returns the result of the load, and in the bool return value:
//   - false if the ledger already had the file an so the file was skipped. (In this
//     case, result is empty and has no meaning.)
//   - true if the ledger was nil - or - the ledger was non-nil and did not have
//     the file - meaning the file was attempted.
//
// A file that couldn't be read is a failure, but the failure isn't recorded in the ledger
// so if the problem is later resolved, (e.g. fixing permissions) it can be re-tried.
// With a nil ledger every file is loaded and nothing is recorded (the nil ledger's
// methods do nothing).
//
// The passed resolved file is inspected and routed independently to LoadFromListFile
// or LoadTarball based on its content - not its name or the CLI flag that supplied it.
// The resolveRefStr arg is the plain --resolve-ref command line value (empty string
// if not supplied); it only has meaning for a tarball and is ignored (with a warning)
// for an image list file.
func loadFile(ledger *Ledger, f ResolvedFile, resolveRefStr string) (LedgerResult, bool) {
	if f.Err != nil {
		log.Errorf("error loading %q: %s", f.Path, f.Err)
		return ResultFailed, true
	}
	fi, err := os.Stat(f.Path)
	if err != nil {
		log.Errorf("error loading %q: %s", f.Path, err)
		return ResultFailed, true
	}
	if ledger.HasEntry(f.Path, fi) {
		log.Debugf("already processed, per the preload ledger: %q", f.Path)
		return "", false
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
		log.Errorf("error loading %q: %s", f.Path, err)
		result = ResultFailed
	case cnt == 0:
		result = ResultSkipped
	}
	if err := ledger.Record(f.Path, fi, result); err != nil {
		log.Errorf("error updating the preload ledger: %s", err)
	}
	return result, true
}
