package cache

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/helpers"
	"github.com/aceeric/ociregistry/impl/metrics"
	"github.com/aceeric/ociregistry/impl/serialize"

	"github.com/aceeric/imgpull/pkg/imgpull"
	log "github.com/sirupsen/logrus"
)

const (
	createdType      = "created"
	accessedType     = "accessed"
	patternType      = "pattern"
	noLimit          = -1
	defaultPruneFreq = "5h"
)

// pruneLog is what the prune code logs through. The scheduled pruner uses the server's
// logger; the REST prune uses a teeLog so the prune's own messages can be returned to the
// REST client without touching the server's logger.
type pruneLog interface {
	Infof(format string, args ...any)
	Errorf(format string, args ...any)
}

// teeLog writes each message to its own logger (whose output is returned to the REST client)
// and to the server's logger, which applies the server's log level and output as usual.
type teeLog struct {
	own *log.Logger
}

func (t teeLog) Infof(format string, args ...any) {
	t.own.Infof(format, args...)
	log.Infof(format, args...)
}

func (t teeLog) Errorf(format string, args ...any) {
	t.own.Errorf(format, args...)
	log.Errorf(format, args...)
}

// ErrPruneBusy is returned when a prune could not start because upstream pulls were still in
// progress after waiting pruneLockWait.
var ErrPruneBusy = errors.New("prune not started: upstream pulls in progress, try again later")

// pruneLockWait is how long a prune waits for in-progress upstream pulls to finish before
// giving up with ErrPruneBusy. It's a var so tests can shorten it.
var pruneLockWait = 10 * time.Second

// RunPruner runs the pruner goroutine if enabled, performing the prune according to the criteria
// and frequency specified in the passed prune configuration. For example, if the configuration
// string has `{"accessed": "15d"}` then using built-in defaults, the pruner will remove images
// that have not been accessed (pulled) within the last 15 days. Unless configured differently,
// the process will run every 5 hours.
func RunPruner(stopChan, stoppedChan chan bool) error {
	cfg := config.GetPruneConfig()
	if !cfg.Enabled {
		log.Info("pruning not enabled")
		return nil
	}
	log.Info("pruning enabled - parsing configuration")
	comparer, err := ParseCriteria(cfg)
	if err != nil {
		return err
	}
	count := noLimit
	if cfg.Count != 0 {
		count = cfg.Count
	}
	freq := defaultPruneFreq
	if cfg.Freq != "" {
		freq = cfg.Freq
	}
	freq, err = days2hrs(freq)
	if err != nil {
		return err
	}
	runFreq, err := time.ParseDuration(freq)
	if err != nil {
		return err
	}
	log.Infof("starting prune goroutine with configuration %v", cfg)
	go func() {
		for {
			select {
			case <-stopChan:
				stoppedChan <- true
				return
			case <-time.After(runFreq):
				if err := doPrune(config.GetImagePath(), comparer, count, cfg.DryRun, log.StandardLogger()); err != nil {
					log.Infof("scheduled prune skipped until next run: %s", err)
				}
			}
		}
	}()
	return nil
}

// Prune is intended to be called by the REST API handlers. It parses the prune configuration
// received on the API, runs the prune with doPrune just like RunPruner, and returns the
// messages the prune logged as a newline-delimited string for the caller to stream to the
// REST client. Only this prune's messages are returned, at info level regardless of the
// server's log level; they are also logged by the server as usual. The server's logger is
// not modified.
func Prune(pruneType string, dur *string, expr *string, dryRun *string, count *int) (string, error) {
	var out strings.Builder
	own := log.New()
	own.SetOutput(&out)
	own.SetLevel(log.InfoLevel)
	own.SetFormatter(log.StandardLogger().Formatter)
	cfg := config.PruneConfig{
		Enabled: true,
		Type:    pruneType,
		DryRun:  true,
		Count:   5,
	}
	if dur != nil {
		cfg.Duration = *dur
	}
	if expr != nil {
		cfg.Expr = *expr
	}
	if count != nil {
		cfg.Count = *count
	}
	if dryRun != nil {
		b, err := strconv.ParseBool(*dryRun)
		if err != nil {
			return "", fmt.Errorf("invalid dry run param: %s", *dryRun)
		}
		cfg.DryRun = b
	}
	comparer, err := ParseCriteria(cfg)
	if err != nil {
		return "", errors.New("unable to parse prune params")
	}
	if err := doPrune(config.GetImagePath(), comparer, cfg.Count, cfg.DryRun, teeLog{own}); err != nil {
		return "", err
	}
	return out.String(), nil
}

// ParseCriteria parses the prune criteria in the passed arg and returns a manifest comparer
// function implementing the logic expressed in the config.
func ParseCriteria(cfg config.PruneConfig) (ManifestComparer, error) {
	if !slices.Contains([]string{createdType, accessedType, patternType}, strings.ToLower(cfg.Type)) {
		return nil, fmt.Errorf("unknown criteria type %q, expect %q or %q", cfg.Type, createdType, accessedType)
	}
	cutoffDate, err := calcCutoff(cfg)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(cfg.Type) {
	case createdType:
		return func(mh imgpull.ManifestHolder) bool {
			if mh.Created == "" {
				log.Debugf("comparer - manifest has no create date, skipping %q", mh.ImageUrl)
				return false
			}
			manifestCreateDt, err := globals.ParseTime(mh.Created)
			if err != nil {
				log.Errorf("comparer - error parsing manifest create date %q for manifest %q", mh.Created, mh.ImageUrl)
				return false
			}
			return manifestCreateDt.Before(cutoffDate)
		}, nil
	case accessedType:
		return func(mh imgpull.ManifestHolder) bool {
			if mh.Pulled == "" {
				log.Debugf("comparer - manifest has no pull date, skipping %q", mh.ImageUrl)
				return false
			}
			manifestPullDt, err := globals.ParseTime(mh.Pulled)
			if err != nil {
				log.Errorf("comparer - error parsing parse manifest pull date %q for manifest %q", mh.Pulled, mh.ImageUrl)
				return false
			}
			return manifestPullDt.Before(cutoffDate)
		}, nil
	case patternType:
		srchs := []*regexp.Regexp{}
		for ref := range strings.SplitSeq(cfg.Expr, ",") {
			if exp, err := regexp.Compile(ref); err == nil {
				srchs = append(srchs, exp)
			} else {
				return nil, fmt.Errorf("regex did not compile: %q", ref)
			}
		}
		return func(mh imgpull.ManifestHolder) bool {
			if len(srchs) != 0 {
				for _, srch := range srchs {
					if srch.MatchString(mh.ImageUrl) {
						return true
					}
				}
			}
			return false
		}, nil
	}
	return nil, fmt.Errorf("unsupported prune type %q", cfg.Type)
}

// calcCutoff returns a prune cutoff date by parsing the passed config. If no cutoff
// date/time is specified then an empty time.Time struct is returned.
func calcCutoff(cfg config.PruneConfig) (time.Time, error) {
	if !slices.Contains([]string{createdType, accessedType}, strings.ToLower(cfg.Type)) {
		return time.Time{}, nil
	}
	if len(cfg.Duration) < 2 || cfg.Type == "" {
		return time.Time{}, errors.New("missing/invalid duration/type in prune criteria")
	}
	durStr := cfg.Duration
	if !strings.HasPrefix(cfg.Duration, "-") {
		durStr = "-" + cfg.Duration
	}
	durStr, err := days2hrs(durStr)
	if err != nil {
		return time.Time{}, err
	}
	dur, err := time.ParseDuration(durStr)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().Add(dur), nil
}

// doPrune makes one pass through the cache and evaluates each manifest according to the passed
// comparer. If the comparer indicates that a manifest matches the prune criteria it is pruned.
// The count arg is the max number of manifests to prune. If noLimit (-1) then there is no limit.
// If dryRun then the function logs what would be pruned but does not actually prune. The whole
// prune holds pullGate exclusively, so new upstream pulls wait for it while it runs; cache hits
// are not affected. If upstream pulls are still in progress after pruneLockWait, the prune is
// not started and ErrPruneBusy is returned.
func doPrune(imagePath string, comparer ManifestComparer, count int, dryRun bool, plog pruneLog) error {
	if !lockForPrune() {
		return ErrPruneBusy
	}
	defer pullGate.Unlock()
	toPrune := GetManifestsCompare(comparer, count)
	plog.Infof("begin prune - count of manifests to prune: %d", len(toPrune))
	for _, mh := range toPrune {
		if dryRun {
			plog.Infof("doPrune - dry run specified, skipping prune of manifest %q", mh.ImageUrl)
			continue
		}
		prune(mh, comparer, imagePath, plog)
	}
	if !dryRun {
		sweepBlobs(imagePath, plog)
	}
	return nil
}

// lockForPrune takes pullGate exclusively, retrying for up to pruneLockWait, and returns false
// if it couldn't. It polls with TryLock rather than calling Lock: a blocked Lock makes every new
// RLock wait behind it, so a prune waiting on one long pull would stall every new cache miss.
func lockForPrune() bool {
	deadline := time.Now().Add(pruneLockWait)
	for {
		if pullGate.TryLock() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// prune removes the passed manifest from the in-mem cache and the file system, and decrements
// the ref counts of its blobs. The passed manifest comes from a list built earlier, so under the
// lock the function first checks that the same manifest is still cached and still matches the
// comparer (it may have been pulled since the list was built). Blob files are not removed here;
// doPrune calls sweepBlobs afterwards. The caller must hold pullGate exclusively.
func prune(mh imgpull.ManifestHolder, comparer ManifestComparer, imagePath string, plog pruneLog) {
	mc.Lock()
	defer mc.Unlock()
	cur, exists := mc.entries[imageUrl(mh.ImageUrl)]
	if !exists || cur.Digest != mh.Digest {
		plog.Infof("skipping prune of manifest %q: no longer cached with digest %s", mh.ImageUrl, mh.Digest)
		return
	}
	if !comparer(cur) {
		plog.Infof("skipping prune of manifest %q: no longer matches prune criteria", mh.ImageUrl)
		return
	}
	plog.Infof("pruning manifest %q", mh.ImageUrl)
	rmManifest(cur, imagePath, plog)
	bc.Lock()
	defer bc.Unlock()
	rmBlobs(cur)
}

// rmManifest removes the passed manifest's entry from the manifest cache, and its file from the
// file system. Other entries with the same digest (other tags, or the digest url) are not
// affected. The blobs for the manifest (if any) are *not* removed. The caller must hold mc.
func rmManifest(mh imgpull.ManifestHolder, imagePath string, plog pruneLog) {
	mc.drop(imageUrl(mh.ImageUrl))
	if err := serialize.RmManifest(imagePath, mh); err != nil {
		plog.Errorf("error removing manifest %q from the file system. the error was: %s", mh.ImageUrl, err)
	}
	plog.Infof("removed manifest: %s", mh.ImageUrl)
}

// rmBlobs decrements the ref count of each blob (layers and config) in the passed manifest.
// A blob whose count reaches zero is removed from the in-mem blob map, but its file is left
// on disk for sweepBlobs. The caller must hold bc.
func rmBlobs(mh imgpull.ManifestHolder) {
	for _, layer := range mh.Layers() {
		digest := helpers.GetDigestFrom(layer.Digest)
		switch cnt := bc.blobs[digest]; {
		case cnt <= 0:
			log.Errorf("blob %q has no refs to remove (should never happen)", digest)
		case cnt == 1:
			delete(bc.blobs, digest)
			metrics.DeltaBlobBytesOnDisk(float64(-layer.Size))
			metrics.DeltaCachedBlobCount(-1)
		default:
			bc.blobs[digest]--
		}
	}
}

// sweepBlobs deletes every blob file that no cached manifest references: blobs whose refs were
// removed by a prune or a tag move, and blobs left by failed pulls. The caller must hold
// pullGate exclusively, so no pull can be relying on a blob it found on disk but has not yet
// referenced.
func sweepBlobs(imagePath string, plog pruneLog) {
	bc.Lock()
	defer bc.Unlock()
	for digest := range serialize.GetAllBlobs(imagePath) {
		if _, referenced := bc.blobs[digest]; referenced {
			continue
		}
		if err := serialize.RmBlob(imagePath, digest); err != nil {
			plog.Errorf("error removing unreferenced blob %q: %s", digest, err)
		} else {
			plog.Infof("removed unreferenced blob: %s", digest)
		}
	}
}

// trySweepBlobs runs sweepBlobs if no upstream pull or prune is in progress, without waiting.
// If one is, the blobs stay until the next prune (or the next successful try).
func trySweepBlobs(imagePath string) {
	if pullGate.TryLock() {
		defer pullGate.Unlock()
		sweepBlobs(imagePath, log.StandardLogger())
	}
}

// days2hrs converts a days string like "-1d" to an hours string like "-24h" because
// the Go Duration parser doesn't support days. If the passed value is not days then
// it is simply returned unchanged.
func days2hrs(v string) (string, error) {
	if v[len(v)-1] == 'd' {
		days := v[0 : len(v)-1]
		intDays, err := strconv.Atoi(days)
		if err != nil {
			return "", err
		}
		hours := intDays * 24
		v = fmt.Sprintf("%dh", hours)
	}
	return v, nil
}
