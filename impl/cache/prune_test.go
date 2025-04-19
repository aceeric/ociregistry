package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"ociregistry/impl/config"
	"ociregistry/impl/pullrequest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetOutput(io.Discard)
}

// Test the mechanics of pruning
func TestPrune(t *testing.T) {
	resetCache()
	td, _ := os.MkdirTemp("", "")
	defer os.RemoveAll(td)
	for _, dir := range []string{"fat", "img", "blobs"} {
		os.Mkdir(filepath.Join(td, dir), 0777)
	}
	mh := imgpull.ManifestHolder{
		Type:     imgpull.V1ociManifest,
		Digest:   strconv.Itoa(int(imgpull.V1ociManifest)),
		ImageUrl: "docker.io/test/manifest:v1.2.3",
	}
	digests := []string{
		"1111111111111111111111111111111111111111111111111111111111111111",
		"1111111111111111111111111111111111111111111111111111111111111112",
		"1111111111111111111111111111111111111111111111111111111111111113",
	}
	for _, digest := range digests {
		if err := os.WriteFile(filepath.Join(td, "blobs", digest), []byte(digest), 0755); err != nil {
			t.Fail()
		}
	}
	if err := json.Unmarshal([]byte(fmt.Sprintf(v1ociManifest, digests[0], digests[1], digests[2])), &mh.V1ociManifest); err != nil {
		t.Fail()
	}
	pr, err := pullrequest.NewPullRequestFromUrl(mh.ImageUrl)
	if err != nil {
		t.Fail()
	}

	addManifestToCache(pr, mh)
	addBlobsToCache(mh, td)
	// manifests that are pulled by tag are added twice - one by tag and a second by digest
	if len(mc.manifests) != 2 || len(bc.blobs) != 3 {
		t.Fail()
	}
	prune(mh, td)
	if len(mc.manifests) != 0 {
		t.Fail()
	}
	for _, digest := range digests {
		if cnt, exists := bc.blobs[digest]; cnt != 0 || exists {
			t.Fail()
		}
	}
}

// Test a simple comparer to test the functionality that traverses the manifest cache
// and calls the comparer.
func TestGetManifestsToPrune(t *testing.T) {
	resetCache()
	for i := 0; i < 100; i++ {
		pr, err := pullrequest.NewPullRequestFromUrl(fmt.Sprintf("foo.io/my-image:%d", i))
		if err != nil {
			t.Fail()
		}
		mh := imgpull.ManifestHolder{
			Type:     imgpull.V1ociIndex,
			Digest:   strconv.Itoa(i),
			ImageUrl: pr.Url(),
		}
		addToCache(pr, mh, "")
	}
	comparer := func(mh imgpull.ManifestHolder) bool {
		return strings.Contains(mh.ImageUrl, "2")
	}
	toPrune := GetManifestsCompare(comparer, -1)
	for _, mh := range toPrune {
		if !strings.Contains(mh.ImageUrl, "2") {
			t.Fail()
		}
	}
}

// Test creating comparers from various prune configurations
func TestParseCriteria(t *testing.T) {
	type parseTest struct {
		cfg        config.PruneConfig
		shouldPass bool
	}
	parseTests := []parseTest{
		{config.PruneConfig{Duration: "invalid", Type: "created"}, false},
		{config.PruneConfig{Duration: "1d", Type: "created"}, true},
		{config.PruneConfig{Duration: "1d", Type: "accessed"}, true},
		{config.PruneConfig{Duration: "12h", Type: "accessed"}, true},
		{config.PruneConfig{Duration: "", Type: "accessed"}, false},
		{config.PruneConfig{Duration: "1d", Type: ""}, false},
		{config.PruneConfig{Duration: "1d", Type: "invalid"}, false},
	}

	for _, parseTest := range parseTests {
		_, err := ParseCriteria(parseTest.cfg)
		if (err != nil && parseTest.shouldPass) || (err == nil && !parseTest.shouldPass) {
			t.FailNow()
		}
	}
}

// Create comparers from various prune configuration and test that they find images
// correctly.
func TestComparer(t *testing.T) {
	timeStr := func(dur string) string {
		t := time.Now()
		d, _ := time.ParseDuration(dur)
		t = t.Add(d)
		return t.Format(dateFormat)
	}
	type parseTest struct {
		cfg           config.PruneConfig
		mh            imgpull.ManifestHolder
		shouldPrune   bool
		failureReason string
	}
	threeDaysAgo := timeStr("-72h")
	oneDayAgo := timeStr("-24h")
	present := timeStr("0h")

	parseTests := []parseTest{
		{config.PruneConfig{Duration: "2d", Type: "created"}, imgpull.ManifestHolder{Created: present}, false, "not earlier"},
		{config.PruneConfig{Duration: "2d", Type: "created"}, imgpull.ManifestHolder{Created: threeDaysAgo}, true, ""},
		{config.PruneConfig{Duration: "2d", Type: "created"}, imgpull.ManifestHolder{Created: oneDayAgo}, false, "not earlier"},
		{config.PruneConfig{Duration: "2d", Type: "accessed"}, imgpull.ManifestHolder{Pulled: present}, false, "not earlier"},
		{config.PruneConfig{Duration: "2d", Type: "accessed"}, imgpull.ManifestHolder{Pulled: threeDaysAgo}, true, ""},
		{config.PruneConfig{Duration: "2d", Type: "accessed"}, imgpull.ManifestHolder{Pulled: oneDayAgo}, false, "not earlier"},
		{config.PruneConfig{Duration: "2d", Type: "created"}, imgpull.ManifestHolder{}, false, "no date to compare"},
		{config.PruneConfig{Duration: "2d", Type: "accessed"}, imgpull.ManifestHolder{}, false, "no date to compare"},
		{config.PruneConfig{Duration: "2d", Type: "created"}, imgpull.ManifestHolder{Created: "foobar"}, false, "un-parseable date"},
		{config.PruneConfig{Duration: "2d", Type: "accessed"}, imgpull.ManifestHolder{Pulled: "foobar"}, false, "un-parseable date"},
	}
	for _, parseTest := range parseTests {
		comparer, err := ParseCriteria(parseTest.cfg)
		if err != nil {
			t.FailNow()
		}
		willPrune := comparer(parseTest.mh)
		if willPrune != parseTest.shouldPrune {
			t.FailNow()
		}
	}
}
