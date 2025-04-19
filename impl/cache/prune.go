package cache

import (
	"errors"
	"fmt"
	"ociregistry/impl/config"
	"ociregistry/impl/helpers"
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/serialize"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

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
// to the REST client if they choose. To do its work, it just doPrune, just like RunPruner.
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

// newLogWriter returns a strut that impements the io.Writer interface
func newLogWriter() *logWriter {
	return &logWriter{
		log: "",
	}
}

// io.Writer Write
func (w *logWriter) Write(b []byte) (cnt int, err error) {
	w.log += string(b)
	return len(b), nil
}

// io.Writer Close
func (w *logWriter) Close() error {
	return nil
}

// ParseCriteria parses the prune criteria in the passed arg and returns a manifest comparer
// function implementing the logic expressed in the config.
func ParseCriteria(cfg config.PruneConfig) (ManifestComparer, error) {
	if !slices.Contains([]string{createdType, accessedType, patternType}, strings.ToLower(cfg.Type)) {
		return nil, fmt.Errorf("unknown criteria type %q, expect %q or %q", cfg.Type, createdType, accessedType)
	}
	var cutoffDate time.Time
	if slices.Contains([]string{createdType, accessedType}, strings.ToLower(cfg.Type)) {
		if len(cfg.Duration) < 2 || cfg.Type == "" {
			return nil, errors.New("missing/invalid duration/type in prune criteria")
		}
		durStr := cfg.Duration
		if !strings.HasPrefix(cfg.Duration, "-") {
			durStr = "-" + cfg.Duration
		}
		durStr, err := days2hrs(durStr)
		if err != nil {
			return nil, err
		}
		dur, err := time.ParseDuration(durStr)
		if err != nil {
			return nil, err
		}
		cutoffDate = time.Now().Add(dur)
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
// cache: once by tag and once by digest for retrieval both ways.
func rmManifest(mh imgpull.ManifestHolder, imagePath string) {
	pr, err := pullrequest.NewPullRequestFromUrl(mh.ImageUrl)
	if err != nil {
		log.Errorf("error parsing manifest URL: %s", err)
		return
	}
	delete(mc.manifests, pr.Url())
	if pr.PullType == pullrequest.ByTag {
		delete(mc.manifests, pr.UrlWithDigest("sha256:"+mh.Digest))
	}
	if err := serialize.RmManifest(imagePath, mh); err != nil {
		log.Errorf("error removing manifest %q from the file system. the error was: %s", pr.Url(), err)
	}
	log.Infof("pruned: %s", pr.Url())
}

// rmBlobs decrements the ref count of all blobs ref'd by the passed image manifest in the in-mem
// blob map and if the ref count is zero, the blob is removed from the map and the file system.
func rmBlobs(mh imgpull.ManifestHolder, imagePath string) {
	for _, layer := range mh.Layers() {
		digest := helpers.GetDigestFrom(layer.Digest)
		bc.blobs[digest]--
		if bc.blobs[digest] == 0 {
			delete(bc.blobs, digest)
			if err := serialize.RmBlob(imagePath, digest); err != nil {
				log.Errorf("error removing blob %q from the file system. the error was: %s", digest, err)
			}
		} else if bc.blobs[digest] < 0 {
			log.Errorf("negative blob count for digest %q (should never happen)", digest)
		} else {
			log.Infof("pruned blob: %s", digest)
		}
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
