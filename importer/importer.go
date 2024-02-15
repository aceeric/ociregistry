package importer

import (
	"errors"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/fsnotify/fsnotify"
)

var (
	waitFor = 100 * time.Millisecond
	mu      sync.Mutex
	timers  = make(map[string]*time.Timer)
)

// Importer creates a file system notifier, watching for archive files to appear
// in the passed 'tarfilePath' directory. When such a file appears, it is inflated into
// a directory structure that the registry server understands and can map to an
// image pull request. The use case for this is to load up a registry from staged
// tarballs exported from an OCI registry, or the containerd cache, etc.
//
// This function uses the Go fsnotify library which can emanate many messages during
// creation of a single file. The approach implemented in the code to address that uses
// time-based event deduplication based on:
//
// https://github.com/fsnotify/fsnotify/blob/main/cmd/fsnotify/dedup.go
//
// However even using this dedup logic there can still be multiple events for the creation of
// a single file. So - after deduplication - the function sends the notifier event to a channel
// which effectively sequences the _mostly_ deduplicated events. This handles the case where
// depdup doesn't catch all the dups. The last thing the importer does is delete the incoming
// archive so - if two events to unarchive the same file are enqueued then when the process
// dequeues the second one, the handler does not treat a missing file as an error, it simply
// ignores the event, assuming it was a dup.
func Importer(tarfilePath string) error {
	if fi, err := os.Stat(tarfilePath); err != nil {
		if err := os.MkdirAll(tarfilePath, 0755); err != nil {
			return err
		}
	} else if !fi.Mode().IsDir() {
		return errors.New("path exists and is not a directory: " + tarfilePath)
	}
	log.Debug("initializing watcher for " + tarfilePath)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic("fsnotify NewWatched failed")
	}
	defer watcher.Close()

	done := make(chan bool)

	go func() {
		for {
			select {
			case _, ok := <-watcher.Errors:
				if !ok {
					// Channel was closed (i.e. Watcher.Close() was called)
					done <- true
					return
				}
			// Read from Events
			case event, ok := <-watcher.Events:
				if !ok {
					// Channel was closed (i.e. Watcher.Close() was called).
					done <- true
					return
				}
				if !event.Has(fsnotify.Create) && !event.Has(fsnotify.Write) {
					continue
				}
				// ignore directories
				if fi, err := os.Stat(event.Name); err == nil && fi.Mode().IsDir() {
					continue
				}
				// ensure supported archive extensions
				var supportedExt bool = false
				for _, ext := range []string{".tgz", ".tar", ".tar.gz"} {
					if strings.HasSuffix(event.Name, ext) {
						supportedExt = true
						break
					}
				}
				if !supportedExt {
					log.Warn("file has unsupported extension. Ignoring: " + event.Name)
					continue
				}

				mu.Lock()
				t, ok := timers[event.Name]
				mu.Unlock()

				// No timer yet, so create one.
				if !ok {
					t = time.AfterFunc(math.MaxInt64, func() {
						handleArchive(tarfilePath, event)
					})
					t.Stop()
					mu.Lock()
					timers[event.Name] = t
					mu.Unlock()
				}
				t.Reset(waitFor)
			}
		}
	}()
	err = watcher.Add(tarfilePath)
	if err != nil {
		panic("fsnotify watcher.Add failed")
	}
	<-done
	log.Debug("terminating watcher")
	return nil
}

// handleArchive extracts the passed archive and then removes it. If the file
// in the passed event doesn't exist then the function assumes it was a dup event
// and just ignores it.
func handleArchive(tarfilePath string, e fsnotify.Event) {
	mu.Lock()
	delete(timers, e.Name)
	mu.Unlock()
	if _, err := os.Stat(e.Name); err == nil {
		if err := Extract(e.Name, tarfilePath); err == nil {
			log.Debug("removing: " + e.Name)
			err := os.Remove(e.Name)
			if err != nil {
				log.Errorf("error attempting to remove file %s. Error: %s", e.Name, err)
			}
		} else {
			log.Errorf("error extracting archive: %s. Error: %s", e.Name, err)
		}
	} else {
		log.Debug("file not found (already processed): " + e.Name)
	}
}
