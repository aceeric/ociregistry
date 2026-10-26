package serialize

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/pullrequest"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

const (
	// MigrationMarker is created in the manifests directory when a migration completes. Its
	// presence means the legacy directories are never read again, so a manifest pruned after
	// migration is not copied back on the next start.
	MigrationMarker = "migration-complete"
	// MigrationReportFile is written to the manifests directory by every migration that is
	// not a dry run, complete or not.
	MigrationReportFile = "migration-report.txt"
)

// MigrationOutcome is what happened to one legacy manifest file.
type MigrationOutcome string

const (
	// Migrated means the manifest was written to the new layout under its own url.
	Migrated MigrationOutcome = "migrated"
	// Relocated means the manifest was written to the new layout under its digest url,
	// because another legacy manifest keeps the tag url (for example an image list and one
	// of its image manifests both recorded with the tag url).
	Relocated MigrationOutcome = "relocated"
	// AlreadyPresent means the manifest is already in the new layout, or is identical to
	// another legacy manifest that was migrated.
	AlreadyPresent MigrationOutcome = "already present"
	// Skipped means the legacy file can't be migrated because of a problem with the file
	// itself. The reason is in the entry's Detail. Skips don't make a migration incomplete.
	Skipped MigrationOutcome = "skipped"
	// Failed means the manifest could not be written. Any failure makes the migration
	// incomplete: the marker is not created, so the next run retries.
	Failed MigrationOutcome = "failed"
)

var allOutcomes = []MigrationOutcome{Migrated, Relocated, AlreadyPresent, Skipped, Failed}

// MigrationEntry records the outcome for one legacy manifest file.
type MigrationEntry struct {
	// File is the legacy file, relative to the image path, e.g. img/3163eabb...
	File string
	// Url is the canonical url the manifest is (or would be) stored under. Empty when the
	// file was skipped before a url could be determined.
	Url string
	// Outcome is what happened to the file.
	Outcome MigrationOutcome
	// Detail is the reason for a skip or failure, or a note such as a normalized url.
	Detail string
}

// MigrationReport describes a migration of the legacy cache layout (manifests named by
// digest in the lts and img directories) to the current layout (one file per url in the
// manifests directory). Every legacy file found has exactly one entry.
type MigrationReport struct {
	ImagePath       string
	DryRun          bool
	AlreadyMigrated bool
	Elapsed         time.Duration
	Found           int
	Entries         []MigrationEntry
}

// Count returns the number of entries with the passed outcome.
func (r *MigrationReport) Count(outcome MigrationOutcome) int {
	cnt := 0
	for _, e := range r.Entries {
		if e.Outcome == outcome {
			cnt++
		}
	}
	return cnt
}

// Complete returns true if every legacy file was accounted for and none failed. Skipped
// files don't make a migration incomplete.
func (r *MigrationReport) Complete() bool {
	return len(r.Entries) == r.Found && r.Count(Failed) == 0
}

// Summary returns the counts as a few lines of text, suitable for a log or a console.
func (r *MigrationReport) Summary() string {
	var b strings.Builder
	switch {
	case r.AlreadyMigrated:
		fmt.Fprintf(&b, "cache at %s is already migrated (report: %s)\n", r.ImagePath,
			filepath.Join(r.ImagePath, globals.ManifestPath, MigrationReportFile))
		return b.String()
	case r.Found == 0:
		fmt.Fprintf(&b, "cache at %s has no legacy manifests to migrate\n", r.ImagePath)
		return b.String()
	}
	mode := ""
	if r.DryRun {
		mode = " dry run (nothing was written)"
	}
	normalized := 0
	for _, e := range r.Entries {
		if strings.HasPrefix(e.Detail, "url normalized") {
			normalized++
		}
	}
	fmt.Fprintf(&b, "cache migration%s for %s\n", mode, r.ImagePath)
	fmt.Fprintf(&b, "legacy manifest files found: %d, accounted for: %d\n", r.Found, len(r.Entries))
	for _, o := range allOutcomes {
		fmt.Fprintf(&b, "  %-16s %d\n", string(o)+":", r.Count(o))
	}
	fmt.Fprintf(&b, "  (urls normalized: %d)\n", normalized)
	verb := "migrated"
	if r.DryRun {
		verb = "would migrate"
	}
	fmt.Fprintf(&b, "%s %d manifest(s) in %s\n", verb, r.Count(Migrated)+r.Count(Relocated), r.Elapsed)
	switch {
	case r.Complete():
		fmt.Fprintf(&b, "result: complete\n")
	case len(r.Entries) != r.Found:
		fmt.Fprintf(&b, "result: INCOMPLETE - %d legacy file(s) were not accounted for\n", r.Found-len(r.Entries))
	default:
		fmt.Fprintf(&b, "result: INCOMPLETE - %d manifest(s) could not be written; fix the cause and run again\n", r.Count(Failed))
	}
	return b.String()
}

// String returns one line per legacy file followed by the summary, so the summary is what's
// left on the screen.
func (r *MigrationReport) String() string {
	var b strings.Builder
	if len(r.Entries) != 0 {
		tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "OUTCOME\tLEGACY FILE\tURL\tDETAIL")
		for _, e := range r.Entries {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.Outcome, e.File, e.Url, e.Detail)
		}
		tw.Flush()
		b.WriteString("\n")
	}
	b.WriteString(r.Summary())
	return b.String()
}

// NeedsMigration returns true if the cache at the passed path has legacy manifests and has
// not been migrated. A new, empty cache doesn't need migration.
func NeedsMigration(imagePath string) (bool, error) {
	if _, err := os.Stat(filepath.Join(imagePath, globals.ManifestPath, MigrationMarker)); err == nil {
		return false, nil
	}
	files, err := legacyFiles(imagePath)
	if err != nil {
		return false, err
	}
	return len(files) > 0, nil
}

// legacyFiles returns the manifest files in the legacy lts and img directories, relative to
// the image path, sorted. A missing legacy directory has no files.
func legacyFiles(imagePath string) ([]string, error) {
	files := []string{}
	for _, sub := range []string{globals.LtsPath, globals.ImgPath} {
		entries, err := os.ReadDir(filepath.Join(imagePath, sub))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.Type().IsRegular() && !strings.HasSuffix(e.Name(), ".tmp") {
				files = append(files, filepath.Join(sub, e.Name()))
			}
		}
	}
	sort.Strings(files)
	return files, nil
}

// candidate is a legacy manifest that was read and parsed successfully.
type candidate struct {
	file      string
	mh        imgpull.ManifestHolder
	legacyUrl string
	url       string
	pr        pullrequest.PullRequest
}

// Migrate copies the manifests in the legacy layout to the current layout. The legacy
// directories are never modified. If the cache is already migrated, nothing is done. If
// dryRun is true, nothing is written and the report shows what would happen.
//
// Each manifest keeps its ImageUrl (canonicalized), Created and Pulled. When more than one
// legacy manifest has the same url, one keeps the url and each of the others is stored
// under its own digest url (Relocated), so no manifest is dropped. An image list is preferred
// for a tag url, otherwise the most recently pulled manifest. Manifests with the same digest
// are the same manifest, so a second copy is AlreadyPresent.
//
// When the migration is not a dry run, the report is written to the manifests directory, and
// the completion marker is created if the migration is complete. An error is returned if the
// migration could not complete; the report is returned either way.
func Migrate(imagePath string, dryRun bool) (MigrationReport, error) {
	start := time.Now()
	report := MigrationReport{ImagePath: imagePath, DryRun: dryRun}
	if needs, err := NeedsMigration(imagePath); err != nil {
		return report, err
	} else if !needs {
		_, err := os.Stat(filepath.Join(imagePath, globals.ManifestPath, MigrationMarker))
		report.AlreadyMigrated = err == nil
		report.Elapsed = time.Since(start)
		return report, nil
	}
	files, err := legacyFiles(imagePath)
	if err != nil {
		return report, err
	}
	report.Found = len(files)
	if !dryRun {
		// if this fails, every write below fails and is reported
		os.MkdirAll(filepath.Join(imagePath, globals.ManifestPath), 0755)
	}

	// pass 1: read and parse every legacy file
	byUrl := map[string][]candidate{}
	for _, file := range files {
		c, entry, ok := readLegacy(imagePath, file)
		if !ok {
			report.Entries = append(report.Entries, entry)
			continue
		}
		byUrl[c.url] = append(byUrl[c.url], c)
	}

	// pass 2: decide where each manifest goes, then write it
	urls := make([]string, 0, len(byUrl))
	for url := range byUrl {
		urls = append(urls, url)
	}
	sort.Strings(urls)
	claimed := map[string]string{} // target url -> legacy file that claimed it
	type placement struct {
		c       candidate
		target  string
		outcome MigrationOutcome
		detail  string
	}
	placements := []placement{}
	for _, url := range urls {
		group := byUrl[url]
		sort.SliceStable(group, func(i, j int) bool { return keepsUrl(group[i], group[j]) })
		holder := group[0]
		placements = append(placements, placement{c: holder, target: url, outcome: Migrated})
		for _, other := range group[1:] {
			if other.mh.Digest == holder.mh.Digest {
				placements = append(placements, placement{c: other, outcome: AlreadyPresent,
					detail: fmt.Sprintf("same manifest as %s", holder.file)})
				continue
			}
			target := other.pr.UrlWithDigest("sha256:" + other.mh.Digest)
			placements = append(placements, placement{c: other, target: target, outcome: Relocated,
				detail: fmt.Sprintf("%s keeps %s", holder.file, url)})
		}
	}
	for _, p := range placements {
		entry := MigrationEntry{File: p.c.file, Url: p.target, Outcome: p.outcome, Detail: p.detail}
		if p.outcome == AlreadyPresent {
			entry.Url = p.c.url
			report.Entries = append(report.Entries, entry)
			continue
		}
		if p.c.legacyUrl != p.target && p.outcome == Migrated {
			entry.Detail = fmt.Sprintf("url normalized from %s", p.c.legacyUrl)
		}
		if by, exists := claimed[p.target]; exists {
			entry.Outcome = AlreadyPresent
			entry.Detail = fmt.Sprintf("same url as %s", by)
			report.Entries = append(report.Entries, entry)
			continue
		}
		claimed[p.target] = p.c.file
		fname := manifestFile(imagePath, p.target)
		if _, err := os.Stat(fname); err == nil {
			entry.Outcome = AlreadyPresent
			entry.Detail = "already in the manifests directory"
			report.Entries = append(report.Entries, entry)
			continue
		}
		if !dryRun {
			mh := p.c.mh
			mh.ImageUrl = p.target
			if b, err := json.Marshal(mh); err != nil {
				entry.Outcome, entry.Detail = Failed, fmt.Sprintf("can't marshal: %s", err)
			} else if err := writeFileAtomic(fname, b, 0644); err != nil {
				entry.Outcome, entry.Detail = Failed, fmt.Sprintf("can't write: %s", err)
			}
		}
		report.Entries = append(report.Entries, entry)
	}
	report.Elapsed = time.Since(start)
	sort.SliceStable(report.Entries, func(i, j int) bool { return report.Entries[i].File < report.Entries[j].File })

	if dryRun {
		return report, nil
	}
	return report, finishMigration(imagePath, &report)
}

// readLegacy reads and parses one legacy file. If the file can't be migrated, it returns
// false and the Skipped entry that says why.
func readLegacy(imagePath string, file string) (candidate, MigrationEntry, bool) {
	skip := func(format string, args ...any) (candidate, MigrationEntry, bool) {
		return candidate{}, MigrationEntry{File: file, Outcome: Skipped, Detail: fmt.Sprintf(format, args...)}, false
	}
	b, err := os.ReadFile(filepath.Join(imagePath, file))
	if err != nil {
		return skip("unreadable: %s", err)
	}
	mh := imgpull.ManifestHolder{}
	if err := json.Unmarshal(b, &mh); err != nil {
		return skip("invalid JSON: %s", err)
	}
	pr, err := pullrequest.NewPullRequestFromUrl(mh.ImageUrl)
	if err != nil {
		return skip("image url %q can't be parsed", mh.ImageUrl)
	}
	if mh.Digest == "" {
		return skip("manifest for %q has no digest", mh.ImageUrl)
	}
	return candidate{file: file, mh: mh, legacyUrl: mh.ImageUrl, url: pr.Url(), pr: pr}, MigrationEntry{}, true
}

// keepsUrl orders legacy manifests that share a url: the first one keeps the url. An image
// list is preferred (a tag names the list; its image manifests are fetched by digest), then
// the most recently pulled, then the most recently created, then the legacy file name so the
// result doesn't depend on directory order.
func keepsUrl(a, b candidate) bool {
	if a.mh.IsManifestList() != b.mh.IsManifestList() {
		return a.mh.IsManifestList()
	}
	if a.mh.Pulled != b.mh.Pulled {
		return a.mh.Pulled > b.mh.Pulled
	}
	if a.mh.Created != b.mh.Created {
		return a.mh.Created > b.mh.Created
	}
	return a.file < b.file
}

// finishMigration writes the report and, if the migration is complete, the marker.
func finishMigration(imagePath string, report *MigrationReport) error {
	dir := filepath.Join(imagePath, globals.ManifestPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(dir, MigrationReportFile), []byte(report.String()), 0644); err != nil {
		return fmt.Errorf("unable to write the migration report: %w", err)
	}
	if !report.Complete() {
		return fmt.Errorf("cache migration incomplete: %d manifest(s) could not be written, see %s",
			report.Count(Failed), filepath.Join(dir, MigrationReportFile))
	}
	marker := fmt.Sprintf("migrated %d legacy manifest file(s) on %s\n", report.Found, globals.CurTime())
	if err := writeFileAtomic(filepath.Join(dir, MigrationMarker), []byte(marker), 0644); err != nil {
		return fmt.Errorf("unable to write the migration marker: %w", err)
	}
	return nil
}
