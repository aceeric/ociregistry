package cache

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/helpers"
	"github.com/aceeric/ociregistry/impl/pullrequest"
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

type logWriter struct {
	log string
}

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
				doPrune(config.GetImagePath(), comparer, count, cfg.DryRun)
			}
		}
	}()
	return nil
}

// Prune is intended to be called by the REST API handlers. It parses the prune configuration
// received on the API. It also "redirects" the logger so that all the logged messages can be
// returned to the caller as a big newline-delimited string. The caller can then stream it
// to the REST client if they choose. To do its work, it calls doPrune, just like RunPruner.
func Prune(pruneType string, dur *string, expr *string, dryRun *string, count *int) (string, error) {
	lw := newLogWriter()
	log.SetOutput(lw)
	defer log.SetOutput(os.Stderr)
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
	doPrune(config.GetImagePath(), comparer, cfg.Count, cfg.DryRun)
	return lw.log, nil
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
			manifestCreateDt, err := time.Parse(dateFormat, mh.Created)
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
			manifestPullDt, err := time.Parse(dateFormat, mh.Pulled)
			if err != nil {
				log.Errorf("comparer - error parsing parse manifest pull date %q for manifest %q", mh.Pulled, mh.ImageUrl)
				return false
			}
			return manifestPullDt.Before(cutoffDate)
		}, nil
	case patternType:
		srchs := []*regexp.Regexp{}
		for _, ref := range strings.Split(cfg.Expr, ",") {
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

// newLogWriter returns a struct that impements the io.Writer interface. It supports
// "redirecting" the system logger to a string array.
func newLogWriter() *logWriter {
	return &logWriter{
		log: "",
	}
}

// Write writes to the log redirection struct.  It supports
// "redirecting" the system logger to a string array.
func (w *logWriter) Write(b []byte) (cnt int, err error) {
	w.log += string(b)
	return len(b), nil
}

// Close closes log redirection.  It supports
// "redirecting" the system logger to a string array.
func (w *logWriter) Close() error {
	return nil
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
// If dryRun then the function logs what would be pruned but does not actually prune.
func doPrune(imagePath string, comparer ManifestComparer, count int, dryRun bool) {
	toPrune := GetManifestsCompare(comparer, count)
	log.Infof("begin prune - count of manifests to prune: %d", len(toPrune))
	for _, mh := range toPrune {
		if dryRun {
			log.Infof("doPrune - dry run specified, skipping prune of manifest %q", mh.ImageUrl)
			continue
		}
		log.Infof("pruning manifest %q", mh.ImageUrl)
		prune(mh, imagePath)
	}
}

// prune removes the passed manifest (and blobs if the manifest is an image manifest)
// from the in-mem cache and from the file system. A lock is held on the manifest cache while
// blobs are being pruned so the cache reports existence or non-existence of an image manifest
// and its blobs as a single unit.
func prune(mh imgpull.ManifestHolder, imagePath string) {
	mc.Lock()
	defer mc.Unlock()
	rmManifest(mh, imagePath)
	if mh.IsImageManifest() {
		bc.Lock()
		defer bc.Unlock()
		rmBlobs(mh, imagePath)
	}
}

// rmManifest removes the passed manifest from the manifest cache and the file system. If the
// manifest is by tag, then the pair by-digest manifest is also removed from in-mem cache if
// one exists. Manifests only exist once on the file system, but may exist twice in the in-mem
// cache: once by tag and once by digest for retrieval both ways. The blobs for the manifest
// (if any) are *not* removed.
func rmManifest(mh imgpull.ManifestHolder, imagePath string) {
	pr, err := pullrequest.NewPullRequestFromUrl(mh.ImageUrl)
	if err != nil {
		log.Errorf("error parsing manifest URL: %s", err)
		return
	}
	mc.delete(pr, mh.Digest)
	if err := serialize.RmManifest(imagePath, mh); err != nil {
		log.Errorf("error removing manifest %q from the file system. the error was: %s", pr.Url(), err)
	}
	log.Infof("removed manifest: %s", pr.Url())
}

// delete actually deletes a manifest from the in-mem cache taking into account whether
// the manifest is tagged "latest" or not.
func (mc *manifestCache) delete(pr pullrequest.PullRequest, digest string) {
	if pr.IsLatest() {
		delete(mc.latest, pr.Url())
		// IsLatest means the manifest has tag "latest"
		delete(mc.latest, pr.UrlWithDigest("sha256:"+digest))
	} else {
		delete(mc.manifests, pr.Url())
		if pr.PullType == pullrequest.ByTag {
			delete(mc.manifests, pr.UrlWithDigest("sha256:"+digest))
		}
	}
}

// rmBlobs decrements the ref count of all blobs ref'd by the passed image manifest in the in-mem
// blob map and if the ref count is zero, the blob is removed from the map and the file system.
func rmBlobs(mh imgpull.ManifestHolder, imagePath string) error {
	for _, layer := range mh.Layers() {
		digest := helpers.GetDigestFrom(layer.Digest)
		if err := rmBlob(digest, imagePath); err != nil {
			return err
		}
	}
	return nil
}

// rmBlob decrements the ref count for the passed blob digest. If zero, the function removes the
// blob from the blob map and the file system.
func rmBlob(digest string, imagePath string) error {
	bc.blobs[digest]--
	if bc.blobs[digest] == 0 {
		delete(bc.blobs, digest)
		if err := serialize.RmBlob(imagePath, digest); err != nil {
			return fmt.Errorf("error removing blob %q from the file system. the error was: %s", digest, err)
		} else {
			log.Infof("removed blob: %s", digest)
		}
	} else if bc.blobs[digest] < 0 {
		return fmt.Errorf("negative blob count for digest %q (should never happen)", digest)
	}
	return nil
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
