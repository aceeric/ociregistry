package cache

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/aceeric/ociregistry/impl/serialize"
	log "github.com/sirupsen/logrus"
)

// pullDates protects the pull dates against a crash. A cache hit updates the pull date in the
// in-mem cache, which is authoritative while the server runs, and the manifests on the file
// system are updated from it when the server shuts down (UpdateManifestPullDates). Meanwhile, a
// background writer journals the pull dates of the urls pulled since the server started to the
// pull dates file every metaWriteFreq, so a cache hit never waits on the file system and a crash
// loses at most one interval of pull dates: the next command that opens the cache applies the
// file to the manifests (serialize.ReconcilePullDates).
//
// Locking: a cache hit or a prune changes urls while holding mc, so the changes for a url are in
// the same order as the cache changes. mu is always taken after mc, never the other way round. A
// flush holds mu only to copy urls, and writes the copy outside it, never taking mc, so writing
// the file can't hold up the cache. Flushes never overlap, because the writer goroutine is the
// only flusher, so written needs no lock. The race detector reports a violation: two flushes at
// once race on written.
type pullDates struct {
	mu sync.Mutex
	// urls are the urls pulled since the server started, with their latest pull dates
	urls map[imageUrl]serialize.UrlMeta
	// version is incremented by every change to urls
	version   uint64
	imagePath string

	// written is the version of urls last written to the pull dates file. It is only used by the
	// flusher: the writer goroutine, or a test calling flushPullDates while no other flush can run.
	written uint64

	// stop and done start and stop the writer goroutine; both are nil if it isn't running
	stop chan struct{}
	done chan struct{}
}

var pd = newPullDates()

// StartPullDateWriter starts the goroutine that writes the pull dates of the cache in imagePath to
// its pull dates file every interval (metaWriteFreq). Call it once, after Load: it starts from no
// pull dates, because the manifests hold the pull dates when the cache is loaded, so starting it
// again would drop the pull dates recorded since the first start. It does nothing if the writer
// is already running.
//
// This code is not concurrent-safe because its very intent is to be called before the
// concurrent-safe pull date management code is started.
func StartPullDateWriter(imagePath string, interval time.Duration) {
	if pd.stop != nil {
		return
	}
	pd.imagePath = imagePath
	pd.urls = map[imageUrl]serialize.UrlMeta{}
	pd.version, pd.written = 0, 0
	pd.stop, pd.done = make(chan struct{}), make(chan struct{})
	go func(stop, done chan struct{}) {
		// however the goroutine ends, the file is flushed before StopPullDateWriter returns
		defer func() {
			flushPullDates()
			close(done)
		}()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				flushPullDates()
			case <-stop:
				return
			}
		}
	}(pd.stop, pd.done)
}

// StopPullDateWriter stops the writer goroutine and waits for it to end, after a final flush. A
// normal shutdown calls it before UpdateManifestPullDates, so the file is current if a manifest
// can't be written. It does nothing if the writer isn't running.
func StopPullDateWriter() {
	if pd.stop == nil {
		return
	}
	close(pd.stop)
	<-pd.done
	pd.stop, pd.done = nil, nil
}

// newPullDates returns an empty pullDates.
func newPullDates() *pullDates {
	return &pullDates{urls: map[imageUrl]serialize.UrlMeta{}}
}

// recordPull records the passed pull date for the url. The caller must hold mc.
func recordPull(url imageUrl, pulled string) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.urls[url] = serialize.UrlMeta{Pulled: pulled}
	pd.version++
}

// recordRemove removes the url, whose manifest was removed, from the pull dates. The caller must
// hold mc.
func recordRemove(url imageUrl) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	if _, ok := pd.urls[url]; ok {
		delete(pd.urls, url)
		pd.version++
	}
}

// flushPullDates writes the pull dates to the pull dates file if they changed since the last
// successful write. It copies them under the lock and writes the copy outside it. A write that
// fails is logged and retried by every flush until one succeeds.
func flushPullDates() {
	pd.mu.Lock()
	version, imagePath := pd.version, pd.imagePath
	if version == pd.written || imagePath == "" {
		// nothing changed, or the cache wasn't loaded so there's no file to write
		pd.mu.Unlock()
		return
	}
	urls := make(map[string]serialize.UrlMeta, len(pd.urls))
	for url, um := range pd.urls {
		urls[string(url)] = um
	}
	pd.mu.Unlock()
	if err := serialize.WritePullDates(imagePath, urls); err != nil {
		log.Errorf("error writing the pull dates file (will retry): %s", err)
		return
	}
	pd.written = version
}

// GetPullDates returns the pull dates recorded since the server started (the content of the
// pull dates journal, which may be newer than the file) as two columns, URL and PULLED, sorted by
// url, with at most count rows. Like a flush, it holds the lock only to copy the pull dates, and
// formats the copy outside it. The empty string means no pull dates have been recorded.
func GetPullDates(count int) string {
	pd.mu.Lock()
	urls := make(map[imageUrl]string, len(pd.urls))
	for url, um := range pd.urls {
		urls[url] = um.Pulled
	}
	pd.mu.Unlock()
	if len(urls) == 0 {
		return ""
	}
	var sb strings.Builder
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "URL\tPULLED")
	for i, url := range slices.Sorted(maps.Keys(urls)) {
		if i >= count {
			break
		}
		fmt.Fprintf(tw, "%s\t%s\n", url, urls[url])
	}
	tw.Flush()
	return sb.String()
}

// UpdateManifestPullDates writes the in-mem pull dates to the manifests and deletes the pull
// dates file, as a normal shutdown activity. Call it after the server stops serving requests and
// in progress pulls are done, and after StopPullDateWriter, whose final flush makes the file
// current: if this function fails or the server crashes partway, the next command that opens the
// cache applies the file instead. It is an error to call it while the writer runs, since a flush
// after the file is deleted would write it again. Only manifests pulled since the server started
// that are still cached are written, from the in-mem cache, which is authoritative: a pruned
// manifest is never written back. The file is kept if any manifest can't be written. Manifests
// are written with the passed writer, normally serialize.MhToFilesystem.
func UpdateManifestPullDates(write serialize.ManifestWriter) error {
	if pd.stop != nil {
		return errors.New("the pull date writer must be stopped before updating the manifest pull dates")
	}
	mc.Lock()
	defer mc.Unlock()
	pd.mu.Lock()
	defer pd.mu.Unlock()
	if pd.imagePath == "" {
		return nil
	}
	cnt := 0
	for url := range pd.urls {
		mh, cached := mc.entries[url]
		if !cached {
			continue
		}
		if err := write(mh, pd.imagePath, true); err != nil {
			return fmt.Errorf("unable to update the pull date of %s (the pull dates file is kept): %w", url, err)
		}
		cnt++
	}
	if err := serialize.RemovePullDates(pd.imagePath); err != nil {
		return err
	}
	log.Infof("updated the pull dates of %d manifest(s)", cnt)
	return nil
}
