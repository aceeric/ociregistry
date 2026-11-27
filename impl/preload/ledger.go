package preload

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/serialize"
	log "github.com/sirupsen/logrus"
	yaml "go.yaml.in/yaml/v4"
)

// LedgerFile is the name of the preload ledger in the cache's metadata directory.
const LedgerFile = "loaded.yaml"

// ledgerVersion is the ledger format version this binary reads and writes.
const ledgerVersion = 1

// LedgerResult is the outcome of processing a file, as recorded in the ledger.
type LedgerResult string

const (
	// ResultLoaded means at least one image was saved to the cache, with no errors.
	ResultLoaded LedgerResult = "loaded"
	// ResultSkipped means there were no errors and nothing changed: the images were already
	// cached with the same digests, or none was for the server's platform.
	ResultSkipped LedgerResult = "skipped"
	// ResultFailed means the file couldn't be read, or an image or url in it failed.
	ResultFailed LedgerResult = "failed"
)

// LedgerEntry records one processed file: its absolute path, size and
// modification time when it was processed (which identify the version of the
// file), when it was processed, and the result. Times are UTC, in
// globals.DateFormat.
type LedgerEntry struct {
	Name      string       `yaml:"name"`
	Size      int64        `yaml:"size"`
	Timestamp string       `yaml:"timestamp"`
	Processed string       `yaml:"processed"`
	Result    LedgerResult `yaml:"result"`
}

// ledgerContent is the ledger file's content.
type ledgerContent struct {
	Version int           `yaml:"version"`
	Files   []LedgerEntry `yaml:"files"`
}

// Ledger records the files that preload (and the watcher) processed into the
// cache, so a file is processed once per version: a file whose path, size and
// modification time match an entry is skipped, whatever the entry's result.
// It's kept in the cache, so a new or rebuilt cache processes everything, and
// it's read once, when opened: to process files again, edit or remove it and
// restart the server. It's safe for concurrent use. A nil *Ledger is a ledger
// that records nothing (as for the load sub-command): Done is always false, and
// the other methods do nothing.
type Ledger struct {
	mu      sync.Mutex
	path    string
	entries map[string]LedgerEntry
}

// OpenLedger reads the ledger in the cache at imagePath. A missing ledger is
// empty. A ledger that can't be read or parsed, or is a newer version, is
// logged as an error (it's not fatal: every file is processed again, which
// re-creates it) and is treated as empty.
func OpenLedger(imagePath string) *Ledger {
	l := &Ledger{
		path:    filepath.Join(imagePath, globals.MetadataPath, LedgerFile),
		entries: map[string]LedgerEntry{},
	}
	entries, err := readLedger(l.path)
	if err != nil {
		log.Errorf("the preload ledger %s can't be used, so all preload files will be processed again: %s", l.path, err)
		return l
	}
	for _, e := range entries {
		l.entries[e.Name] = e
	}
	return l
}

// HasEntry returns true if the ledger has an entry for the file at path with
// the passed size and modification time, i.e. this version of the file was
// already processed. For the nil ledger, false is returned.
func (l *Ledger) HasEntry(path string, fi os.FileInfo) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[path]
	return ok && e.Size == fi.Size() && e.Timestamp == fileTime(fi)
}

// Record records the result of processing the file at path, whose size and
// modification time (before it was processed) are in fi, replacing any entry
// for it, and writes the ledger.  For the nil ledger, nothing happens and nil
// is returned.
func (l *Ledger) Record(path string, fi os.FileInfo, result LedgerResult) error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries[path] = LedgerEntry{Name: path, Size: fi.Size(), Timestamp: fileTime(fi), Processed: globals.CurTime(), Result: result}
	return l.write()
}

// RemoveMissing removes ledger entries for files that no longer exist, and
// writes the ledger if any were removed. Recall that the ledger tracks incoming
// files from the `--preload-images` arg, so running `ociregistry serve
// --preload-images /foo` and then removing /foo/file1 that was already
// processed will remove that entry from the ledger. Subsequently, replacing
// that same file will cause it be be re-processed as new and re-added to the
// ledger.
func (l *Ledger) RemoveMissing() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	removed := false
	for name := range l.entries {
		if _, err := os.Stat(name); errors.Is(err, fs.ErrNotExist) {
			delete(l.entries, name)
			removed = true
		}
	}
	if !removed {
		return nil
	}
	return l.write()
}

// Entries returns the ledger entries sorted by name. For the nil ledger, nil is
// returned.
func (l *Ledger) Entries() []LedgerEntry {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sorted()
}

// write writes the ledger atomically, entries sorted by name. The caller must
// hold mu. The function does not check for nil ledger because it is only called
// "internally" by other ledger funcs when ledger is already known to be
// non-nill.
func (l *Ledger) write() error {
	b, err := yaml.Marshal(ledgerContent{Version: ledgerVersion, Files: l.sorted()})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0755); err != nil {
		return err
	}
	return serialize.WriteFileAtomic(l.path, b, 0644)
}

// sorted returns the entries sorted by name. The caller must hold mu.
func (l *Ledger) sorted() []LedgerEntry {
	return slices.SortedFunc(maps.Values(l.entries), func(a, b LedgerEntry) int { return strings.Compare(a.Name, b.Name) })
}

// readLedger reads and strictly parses the ledger at path. A missing ledger is
// returned as a ledger with no entries.
func readLedger(path string) ([]LedgerEntry, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	content := ledgerContent{}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&content); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if content.Version != ledgerVersion {
		return nil, fmt.Errorf("version %d, this version of the server reads version %d", content.Version, ledgerVersion)
	}
	return content.Files, nil
}

// fileTime returns the modification time of the file as recorded in the ledger.
func fileTime(fi os.FileInfo) string {
	return fi.ModTime().UTC().Format(globals.DateFormat)
}
