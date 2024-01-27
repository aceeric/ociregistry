package importer

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/labstack/echo"
)

var (
	waitFor = 100 * time.Millisecond
	mu      sync.Mutex
	timers  = make(map[string]*time.Timer)
)

// Importer creates a file system notifier, watching for archive files to appear
// in the pased "tarfilePath" directory. When such a file appears, it is inflated into
// a directory structure that the registry server understands and can map to an
// image pull request.
//
// This function uses the Go fsnotify library which can emanate many messages during
// creation of a single file. The approach implemented below to address that uses
// time-based event deduplication based on:
//
// https://github.com/fsnotify/fsnotify/blob/main/cmd/fsnotify/dedup.go
//
// However even using this dedup logic there can still be multiple events for the creation of
// a single file. So - after deduplication- the function sends the notifier event to a channel
// which effectively sequences the event stream. This handles the case where depdup doesn't
// catch all the dups. The last thing the importer does is delete the incoming archive so -
// if two events to unarchive the same file are enqueued then when the process deques the
// second one, the handler does not treat a amissing file as an error, it simply ignores
// the event.
func Importer(tarfilePath string, logger echo.Logger) error {
	if fi, err := os.Stat(tarfilePath); err != nil {
		if err := os.MkdirAll(tarfilePath, 0755); err != nil {
			return err
		}
	} else if !fi.Mode().IsDir() {
		return errors.New("path exists and is not a directory: " + tarfilePath)
	}
	logger.Info("initializing watcher for " + tarfilePath)
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
				// ensure supported archive extensions
				var supportedExt bool = false
				for _, ext := range []string{".tgz", ".tar", ".tar.gz"} {
					if strings.HasSuffix(event.Name, ext) {
						supportedExt = true
						break
					}
				}
				if !supportedExt {
					logger.Warn("file has unsupported extension. Ignoring: " + event.Name)
					continue
				}

				mu.Lock()
				t, ok := timers[event.Name]
				mu.Unlock()

				// No timer yet, so create one.
				if !ok {
					t = time.AfterFunc(math.MaxInt64, func() {
						handleArchive(tarfilePath, event, logger)
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
	logger.Info("terminating watcher")
	return nil
}

// handleArchive extracts the passed archive and then removes it. If the file
// in the passed event doesnt exist then the function assumes it was a dup event
// and just ignores it.
func handleArchive(tarfilePath string, e fsnotify.Event, logger echo.Logger) {
	mu.Lock()
	delete(timers, e.Name)
	mu.Unlock()
	if _, err := os.Stat(e.Name); err == nil {
		if err := Extract(e.Name, tarfilePath); err == nil {
			logger.Info("removing: " + e.Name)
			err := os.Remove(e.Name)
			if err != nil {
				logger.Error(fmt.Sprintf("error attempting to remove file %s. Error: %s", e.Name, err))
			}
		}
	} else {
		logger.Info("file not found (already processed): " + e.Name)
	}
}