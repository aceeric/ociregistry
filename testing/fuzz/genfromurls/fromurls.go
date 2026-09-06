// Package genfromurls implements --generate-from-urls: reading a file of
// known-good, working URLs and producing subtle alterations of them
// (character-level, structural, and size mutations), appended to the
// generate file for later replay by --run-endpoints.
package genfromurls

import (
	"bufio"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"strings"

	"ociregistry-fuzz/fuzzcorpus"
	"ociregistry-fuzz/genendpoints"
)

type Line = genendpoints.Line

var injectTokens = []string{"..", "%00", "'", "<script>", " ", "%2e%2e%2f", ";", "|"}

// Generate reads inPath (one URL per line; blank lines and lines starting
// with '#' are ignored), and returns a set of mutated Lines derived from
// each. Both http and https input URLs are accepted; every mutated line
// always keeps the exact scheme of its source URL, since the server
// serves only one scheme at a time and a scheme-flipped request would
// just be a guaranteed connection failure (see the note at the bottom of
// mutate() for why this is deliberate, not an oversight).
func Generate(inPath string, stress int, seed int64) ([]Line, error) {
	f, err := os.Open(inPath)
	if err != nil {
		return nil, fmt.Errorf("opening urls file: %w", err)
	}
	defer f.Close()

	rng := rand.New(rand.NewSource(seed))
	stress = clamp(stress)

	var lines []Line
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			fmt.Fprintf(os.Stderr, "warning: skipping unparseable URL on line %d: %s\n", lineNo, raw)
			continue
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			fmt.Fprintf(os.Stderr, "warning: skipping non-http(s) URL on line %d: %s\n", lineNo, raw)
			continue
		}
		lines = append(lines, mutate(u, raw, rng, stress)...)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading urls file: %w", err)
	}
	return lines, nil
}

func mutate(u *url.URL, orig string, rng *rand.Rand, stress int) []Line {
	var out []Line
	segs := splitPath(u.Path)
	if len(segs) == 0 {
		return out
	}

	// How many "one-at-a-time" mutation passes to run, scaled by stress.
	passes := 1 + stress/10 // 1..11
	if passes > len(segs)*3 {
		passes = len(segs) * 3
	}
	if passes < 1 {
		passes = 1
	}

	add := func(label, path, query string) {
		nu := *u
		nu.Path = "/" + strings.Join(splitPath(path), "/")
		if strings.HasSuffix(path, "/") {
			nu.Path += "/"
		}
		query, forced := forcePruneDryRun(nu.Path, query)
		nu.RawQuery = query
		if forced {
			label += " [dryRun forced true: /cmd/prune deletes by default]"
		}
		out = append(out, Line{
			Comment: fmt.Sprintf("from-url mutation=%s orig=%s", label, orig),
			Method:  "GET",
			URL:     nu.String(),
		})
	}

	// 1. Per-segment character mutations, cycling through segments.
	for i := 0; i < passes; i++ {
		idx := i % len(segs)
		seg := segs[idx]
		if seg == "" {
			continue
		}
		mutated := append([]string{}, segs...)

		switch i % 6 {
		case 0: // flip case of first alpha char
			mutated[idx] = flipCase(seg)
			add("flip-case", strings.Join(mutated, "/"), u.RawQuery)
		case 1: // truncate last char
			if len(seg) > 1 {
				mutated[idx] = seg[:len(seg)-1]
				add("truncate", strings.Join(mutated, "/"), u.RawQuery)
			}
		case 2: // append a char
			mutated[idx] = seg + "X"
			add("append-char", strings.Join(mutated, "/"), u.RawQuery)
		case 3: // inject a special token into the middle
			tok := injectTokens[rng.Intn(len(injectTokens))]
			mid := len(seg) / 2
			mutated[idx] = seg[:mid] + tok + seg[mid:]
			add("inject-token", strings.Join(mutated, "/"), u.RawQuery)
		case 4: // replace whole segment with a generic fuzz corpus value
			cases := fuzzcorpus.GenericCases(stress)
			c := cases[rng.Intn(len(cases))]
			mutated[idx] = c.Value
			add("replace-with-"+c.Label, strings.Join(mutated, "/"), u.RawQuery)
		case 5: // lengthen the segment substantially
			n := 1000
			if stress >= 40 {
				n = 50_000
			}
			mutated[idx] = seg + strings.Repeat("Z", n)
			add("lengthen-segment", strings.Join(mutated, "/"), u.RawQuery)
		}
	}

	// 2. Structural: drop a segment, duplicate a segment, swap two segments.
	if len(segs) >= 1 {
		dropIdx := rng.Intn(len(segs))
		dropped := append(append([]string{}, segs[:dropIdx]...), segs[dropIdx+1:]...)
		add("drop-segment", strings.Join(dropped, "/"), u.RawQuery)

		dupIdx := rng.Intn(len(segs))
		dup := append(append([]string{}, segs[:dupIdx+1]...), segs[dupIdx:]...)
		add("duplicate-segment", strings.Join(dup, "/"), u.RawQuery)
	}
	if len(segs) >= 2 {
		i, j := rng.Intn(len(segs)), rng.Intn(len(segs))
		if i != j {
			swapped := append([]string{}, segs...)
			swapped[i], swapped[j] = swapped[j], swapped[i]
			add("swap-segments", strings.Join(swapped, "/"), u.RawQuery)
		}
	}

	// 3. Tag/digest-in-last-segment mutation: many pull-style URLs encode
	//    a ":tag" or "@sha256:..." suffix in the final segment.
	last := segs[len(segs)-1]
	if strings.Contains(last, ":") || strings.Contains(last, "@") {
		sep := ":"
		if strings.Contains(last, "@") && !strings.Contains(last, ":") {
			sep = "@"
		}
		parts := strings.SplitN(last, sep, 2)
		if len(parts) == 2 {
			mutatedLast := parts[0] + sep + mutateSuffix(parts[1], rng)
			mutated := append([]string{}, segs...)
			mutated[len(mutated)-1] = mutatedLast
			add("mutate-tag-or-digest-suffix", strings.Join(mutated, "/"), u.RawQuery)
		}
	}

	// 4. Query-string mutations.
	add("extra-query-param", strings.Join(segs, "/"), addQuery(u.RawQuery, "unexpected_extra_param=1"))
	if stress >= 10 {
		add("traversal-query-param", strings.Join(segs, "/"), addQuery(u.RawQuery, "ns=../../../etc/passwd"))
	}

	// 5. Trailing slash toggling.
	if strings.HasSuffix(u.Path, "/") {
		add("remove-trailing-slash", strings.TrimSuffix(strings.Join(segs, "/"), "/"), u.RawQuery)
	} else {
		add("add-trailing-slash", strings.Join(segs, "/")+"/", u.RawQuery)
	}

	// Note: we deliberately do NOT generate a scheme-toggled variant here.
	// The server serves exactly one scheme at a time (whichever the
	// operator configured), so flipping http<->https would just produce a
	// guaranteed connection failure - wasted effort at best, and at worst
	// it could spuriously trip --run-endpoints' consecutive-failure halt
	// logic. Every mutated line always keeps the exact scheme given in
	// the input file. To test the other scheme, run the server itself
	// with that scheme and pass it a urls file using it.

	return out
}

// forcePruneDryRun overrides (or adds) dryRun=true in rawQuery whenever
// path resolves exactly to /cmd/prune. The server treats a missing or
// malformed dryRun as false (i.e. "prune for real"), and this mode has no
// per-parameter targeting the way --generate-endpoints does, so the only
// safe policy here is: any mutated URL that could still reach the real
// prune route always gets dryRun forced to true, unconditionally. It
// returns the (possibly unchanged) query and whether it forced anything,
// purely so callers can annotate the line for transparency.
func forcePruneDryRun(path, rawQuery string) (string, bool) {
	norm := "/" + strings.Trim(path, "/")
	if !strings.EqualFold(norm, "/cmd/prune") {
		return rawQuery, false
	}
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		q = url.Values{}
	}
	already := q.Get("dryRun") == "true" && len(q["dryRun"]) == 1
	q.Set("dryRun", "true")
	return q.Encode(), !already
}

func mutateSuffix(s string, rng *rand.Rand) string {
	if s == "" {
		return "X"
	}
	choices := []func(string) string{
		func(s string) string { return s + "X" },
		func(s string) string { return s[:len(s)-1] },
		func(s string) string { return flipCase(s) },
		func(s string) string { return strings.Repeat("z", 64) },
		func(s string) string { return "0" },
	}
	return choices[rng.Intn(len(choices))](s)
}

func flipCase(s string) string {
	for i, r := range s {
		if r >= 'a' && r <= 'z' {
			return s[:i] + strings.ToUpper(string(r)) + s[i+1:]
		}
		if r >= 'A' && r <= 'Z' {
			return s[:i] + strings.ToLower(string(r)) + s[i+1:]
		}
	}
	return s
}

func addQuery(existing, add string) string {
	if existing == "" {
		return add
	}
	return existing + "&" + add
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func clamp(stress int) int {
	if stress < 1 {
		return 1
	}
	if stress > 100 {
		return 100
	}
	return stress
}

// Append appends lines to path (creating it if it doesn't exist), per the
// requirement that --generate-from-urls appends to the generate file.
func Append(lines []Line, path string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening output file for append: %w", err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()
	for _, l := range lines {
		if l.Comment != "" {
			fmt.Fprintf(w, "# %s\n", l.Comment)
		}
		fmt.Fprintf(w, "%s\t%s\n", l.Method, l.URL)
	}
	return nil
}
