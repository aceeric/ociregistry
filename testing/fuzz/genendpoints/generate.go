// Package genendpoints implements --generate-endpoints: turning the parsed
// OpenAPI spec into a large set of test request lines covering defined
// endpoints (with baseline + one-at-a-time malicious mutations of every
// path/query parameter, plus a few structural path mutations) and a random
// sampling of undefined endpoints/probe paths.
package genendpoints

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strings"

	"ociregistry-fuzz/fuzzcorpus"
	"ociregistry-fuzz/spec"
)

// Options controls generation.
type Options struct {
	Host        string // host:port, no scheme
	Scheme      string // "http" only, enforced by caller
	StressLevel int    // 1-100
	Seed        int64
	OutPath     string // truncated/created fresh

	// Exclude lists operations to skip entirely (no baseline, no
	// mutations at all - not even a request with a bogus query string,
	// since for a parameter-less endpoint that would still hit the real
	// route). Each entry is either a bare path template, e.g.
	// "/cmd/stop" (excludes the operation regardless of HTTP method), or
	// "METHOD:/path/template", e.g. "GET:/cmd/stop" (excludes only that
	// method). Matching is exact against the path template as it appears
	// in the spec (including the {placeholder} braces).
	Exclude []string
}

// DefaultExcludes are operations excluded from --generate-endpoints
// unless the caller overrides them. GET /cmd/stop actually shuts the
// server down, so hitting it for real (even accidentally, e.g. via a
// generated "add an unknown query param" case) would kill the server
// mid-run - it must never be generated as a live, unmutated-path request.
var DefaultExcludes = []string{
	"/cmd/stop",
}

func buildExcludeSet(exclude []string) map[string]bool {
	set := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		set[e] = true
	}
	return set
}

// excluded reports whether the given operation should be skipped, per
// opt.Exclude. Entries may be "METHOD:/path" or just "/path" (all
// methods on that path).
func excluded(set map[string]bool, method, path string) bool {
	if set[path] {
		return true
	}
	if set[strings.ToUpper(method)+":"+path] {
		return true
	}
	return false
}

// Line is one emitted test case.
type Line struct {
	Comment string // human-readable, written as a "# ..." line before URL (may be empty)
	Method  string
	URL     string
}

// Generate builds the full set of Lines for the given spec and options.
func Generate(sp *spec.Spec, opt Options) []Line {
	rng := rand.New(rand.NewSource(opt.Seed))
	stress := clamp(opt.StressLevel)
	base := opt.Scheme + "://" + opt.Host
	excludeSet := buildExcludeSet(opt.Exclude)

	var lines []Line

	for _, pi := range sp.Paths {
		pathParamNames := pi.TemplateParamNames()
		for _, op := range pi.Operations {
			if excluded(excludeSet, op.Method, pi.Path) {
				lines = append(lines, Line{
					Comment: fmt.Sprintf("SKIPPED (excluded): %s %s", op.Method, pi.Path),
				})
				continue
			}
			lines = append(lines, defineOperationCases(base, pi, op, pathParamNames, stress)...)
		}
	}

	lines = append(lines, undefinedEndpointSamples(base, rng, stress)...)

	return lines
}

// baselineValue returns a plausible, well-formed placeholder value for a
// named path parameter, based on common OCI naming conventions.
func baselineValue(name string) string {
	switch name {
	case "digest":
		return "sha256:" + strings.Repeat("a", 64)
	case "reference":
		return "latest"
	case "name":
		return "test-repo"
	default:
		// s1, s2, s3, s4, or anything else.
		return "test-" + name
	}
}

func substitute(template string, values map[string]string) string {
	out := template
	for k, v := range values {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	return out
}

func buildURL(base, path, query string) string {
	if query == "" {
		return base + path
	}
	return base + path + "?" + query
}

func defineOperationCases(base string, pi spec.PathItem, op spec.Operation, pathParamNames []string, stress int) []Line {
	if pi.Path == "/cmd/prune" {
		// /cmd/prune deletes images by default (dryRun defaults to
		// false server-side), so it gets fully separate, safety-first
		// handling rather than the generic query-fuzzing path below.
		return pruneOperationCases(base, pi, op, stress)
	}

	var lines []Line

	baseline := map[string]string{}
	for _, n := range pathParamNames {
		baseline[n] = baselineValue(n)
	}
	queryParams := op.QueryParams()
	requiredQuery := ""
	for _, qp := range queryParams {
		if qp.Required {
			if requiredQuery != "" {
				requiredQuery += "&"
			}
			requiredQuery += qp.Name + "=" + baselineValue(qp.Name)
		}
	}

	tag := fmt.Sprintf("%s %s", op.Method, pi.Path)

	// 1. Baseline "happy path" request.
	lines = append(lines, Line{
		Comment: "baseline: " + tag,
		Method:  op.Method,
		URL:     buildURL(base, substitute(pi.Path, baseline), requiredQuery),
	})

	// 2. One-at-a-time malicious mutation of each path parameter.
	for _, pname := range pathParamNames {
		cases := fuzzcorpus.GenericCases(stress)
		if pname == "digest" {
			cases = append(cases, fuzzcorpus.DigestCases(stress)...)
		}
		for _, c := range cases {
			values := cloneMap(baseline)
			values[pname] = c.Value
			lines = append(lines, Line{
				Comment: fmt.Sprintf("%s param=%s case=%s", tag, pname, c.Label),
				Method:  op.Method,
				URL:     buildURL(base, substitute(pi.Path, values), requiredQuery),
			})
		}
	}

	// 3. Structural path mutations: drop a segment (changes route depth),
	//    duplicate a segment, insert an extra bogus segment.
	if len(pathParamNames) > 0 {
		lines = append(lines, structuralMutations(base, pi, baseline, requiredQuery, tag)...)
	}

	// 4. One-at-a-time query parameter fuzzing.
	for _, qp := range queryParams {
		cases := queryCasesFor(pi.Path, qp, stress)
		for _, c := range cases {
			q := buildQueryOneAtATime(queryParams, qp.Name, c.Value)
			lines = append(lines, Line{
				Comment: fmt.Sprintf("%s query=%s case=%s", tag, qp.Name, c.Label),
				Method:  op.Method,
				URL:     buildURL(base, substitute(pi.Path, baseline), q),
			})
		}
	}

	// 5. Missing required query parameter(s), if any.
	for _, qp := range queryParams {
		if !qp.Required {
			continue
		}
		q := buildQueryOmitting(queryParams, qp.Name)
		lines = append(lines, Line{
			Comment: fmt.Sprintf("%s query=%s case=missing-required", tag, qp.Name),
			Method:  op.Method,
			URL:     buildURL(base, substitute(pi.Path, baseline), q),
		})
	}

	// 6. Extra/unknown query parameters and duplicate query parameters.
	extraQ := requiredQuery
	if extraQ != "" {
		extraQ += "&"
	}
	extraQ += "unexpected_extra_param=1&admin=true"
	lines = append(lines, Line{
		Comment: tag + " query=<extra-unknown>",
		Method:  op.Method,
		URL:     buildURL(base, substitute(pi.Path, baseline), extraQ),
	})
	if len(queryParams) > 0 {
		dup := queryParams[0].Name + "=" + baselineValue(queryParams[0].Name) + "&" +
			queryParams[0].Name + "=" + "second-value-" + queryParams[0].Name
		lines = append(lines, Line{
			Comment: tag + " query=<duplicate-param>",
			Method:  op.Method,
			URL:     buildURL(base, substitute(pi.Path, baseline), dup),
		})
	}

	return lines
}

// queryCasesFor selects the fuzz corpus for a given query parameter,
// taking into account the documented semantics of specific /cmd/*
// parameters (regex-compiled "pattern"/"expr" fields get ReDoS-style
// cases, digest/substring fields get malformed-digest cases, etc.) in
// addition to the generic type-based defaults.
func queryCasesFor(pathTemplate string, qp spec.Param, stress int) []fuzzcorpus.Case {
	switch {
	case (pathTemplate == "/cmd/image/list" || pathTemplate == "/cmd/manifest/list") && qp.Name == "pattern":
		return fuzzcorpus.RegexPatternCases(stress)
	case pathTemplate == "/cmd/image/list" && qp.Name == "digest":
		return append(fuzzcorpus.GenericCases(stress), fuzzcorpus.DigestCases(stress)...)
	case pathTemplate == "/cmd/blob/list" && qp.Name == "substr":
		return append(fuzzcorpus.GenericCases(stress), fuzzcorpus.DigestCases(stress)...)
	case qp.Type == "integer":
		return fuzzcorpus.IntegerCases(stress)
	default:
		cases := fuzzcorpus.GenericCases(stress)
		if strings.EqualFold(qp.Name, "dryRun") {
			cases = append(cases, fuzzcorpus.BooleanishCases(stress)...)
		}
		return cases
	}
}

// pruneOperationCases generates the test lines for /cmd/prune. Unlike
// every other operation, this one deletes data by default (the server
// treats a missing/malformed "dryRun" as false, i.e. "prune for real"),
// so every generated request - no matter which parameter is under test -
// forces dryRun=true, except for the cases that specifically fuzz the
// dryRun parameter's own value, which instead rely on a secondary safety
// net: type=pattern with an expr guaranteed to match no real manifest, so
// that even a misparsed dryRun can't delete anything.
func pruneOperationCases(base string, pi spec.PathItem, op spec.Operation, stress int) []Line {
	var lines []Line
	tag := fmt.Sprintf("%s %s", op.Method, pi.Path)

	// defaults() is called fresh each time to avoid aliasing a shared map
	// across iterations.
	defaults := func() map[string]string {
		return map[string]string{
			"type":   "pattern",
			"expr":   fuzzcorpus.SafeNonMatchExpr,
			"dryRun": "true",
		}
	}
	build := func(vals map[string]string) string {
		order := []string{"type", "dur", "expr", "count", "dryRun"}
		var parts []string
		for _, k := range order {
			if v, ok := vals[k]; ok {
				parts = append(parts, k+"="+v)
			}
		}
		return strings.Join(parts, "&")
	}

	note := "dryRun forced true - /cmd/prune deletes by default"

	// 1. Baseline.
	lines = append(lines, Line{
		Comment: fmt.Sprintf("baseline: %s (%s)", tag, note),
		Method:  op.Method,
		URL:     buildURL(base, pi.Path, build(defaults())),
	})

	// 2. Fuzz "type". dryRun stays true no matter what type resolves to.
	for _, c := range fuzzcorpus.EnumCases([]string{"accessed", "created", "pattern"}, stress) {
		vals := defaults()
		vals["type"] = c.Value
		lines = append(lines, Line{
			Comment: fmt.Sprintf("%s query=type case=%s (%s)", tag, c.Label, note),
			Method:  op.Method,
			URL:     buildURL(base, pi.Path, build(vals)),
		})
	}

	// 3. Fuzz "dur". Forced to type=created (not "pattern") so dur is
	//    actually parsed/exercised server-side, per the documented
	//    "dur is ignored when type=pattern" behavior. dryRun stays true.
	for _, c := range fuzzcorpus.DurationCases(stress) {
		vals := defaults()
		vals["type"] = "created"
		delete(vals, "expr")
		vals["dur"] = c.Value
		lines = append(lines, Line{
			Comment: fmt.Sprintf("%s query=dur case=%s (%s)", tag, c.Label, note),
			Method:  op.Method,
			URL:     buildURL(base, pi.Path, build(vals)),
		})
	}

	// 4. Fuzz "expr" (includes ReDoS shapes and "match everything").
	//    dryRun stays true, so even a pattern matching every manifest
	//    can't actually delete anything.
	for _, c := range fuzzcorpus.RegexPatternCases(stress) {
		vals := defaults()
		vals["expr"] = c.Value
		lines = append(lines, Line{
			Comment: fmt.Sprintf("%s query=expr case=%s (%s)", tag, c.Label, note),
			Method:  op.Method,
			URL:     buildURL(base, pi.Path, build(vals)),
		})
	}

	// 5. Fuzz "count". dryRun stays true regardless.
	for _, c := range fuzzcorpus.IntegerCases(stress) {
		vals := defaults()
		vals["count"] = c.Value
		lines = append(lines, Line{
			Comment: fmt.Sprintf("%s query=count case=%s (%s)", tag, c.Label, note),
			Method:  op.Method,
			URL:     buildURL(base, pi.Path, build(vals)),
		})
	}

	// 6. Fuzz "dryRun" itself. This is the one case where the value under
	//    test *is* the safety flag, so the secondary safety net (a
	//    guaranteed-non-matching expr, already in defaults()) is what
	//    protects real data here instead.
	dryRunCases := append(fuzzcorpus.GenericCases(stress), fuzzcorpus.BooleanishCases(stress)...)
	for _, c := range dryRunCases {
		vals := defaults()
		vals["dryRun"] = c.Value
		lines = append(lines, Line{
			Comment: fmt.Sprintf("%s query=dryRun case=%s (secondary safety net: type=pattern + non-matching expr)", tag, c.Label),
			Method:  op.Method,
			URL:     buildURL(base, pi.Path, build(vals)),
		})
	}

	// 7. Missing required "type". dryRun stays true regardless.
	lines = append(lines, Line{
		Comment: fmt.Sprintf("%s query=type case=missing-required (%s)", tag, note),
		Method:  op.Method,
		URL:     buildURL(base, pi.Path, "dryRun=true"),
	})

	// 8. Extra/unknown and duplicate query params. dryRun stays true.
	lines = append(lines, Line{
		Comment: fmt.Sprintf("%s query=<extra-unknown> (%s)", tag, note),
		Method:  op.Method,
		URL:     buildURL(base, pi.Path, build(defaults())+"&unexpected_extra_param=1&admin=true"),
	})
	lines = append(lines, Line{
		Comment: fmt.Sprintf("%s query=<duplicate-dryRun-param> (both true, still safe)", tag),
		Method:  op.Method,
		URL:     buildURL(base, pi.Path, build(defaults())+"&dryRun=true"),
	})

	return lines
}

func buildQueryOneAtATime(all []spec.Param, target, value string) string {
	var parts []string
	for _, p := range all {
		if p.Name == target {
			parts = append(parts, p.Name+"="+value)
		} else if p.Required {
			parts = append(parts, p.Name+"="+baselineValue(p.Name))
		}
	}
	return strings.Join(parts, "&")
}

func buildQueryOmitting(all []spec.Param, omit string) string {
	var parts []string
	for _, p := range all {
		if p.Name == omit {
			continue
		}
		if p.Required {
			parts = append(parts, p.Name+"="+baselineValue(p.Name))
		}
	}
	return strings.Join(parts, "&")
}

// structuralMutations perturbs the *shape* of the path itself: removing a
// templated segment (route becomes shallower), duplicating one (deeper),
// and inserting a bogus extra literal segment near the end. This targets
// exactly the class of bug the tool exists to catch: a server that
// resolves an unexpected segment count/shape to a route it shouldn't.
func structuralMutations(base string, pi spec.PathItem, baseline map[string]string, requiredQuery, tag string) []Line {
	var lines []Line
	full := substitute(pi.Path, baseline)
	segs := strings.Split(strings.TrimPrefix(full, "/"), "/")

	// Drop each segment once (only for segments that came from a path
	// parameter, to preserve at least the literal skeleton words like
	// "v2", "manifests", "blobs" as anchors for comparison).
	for i, s := range segs {
		if !looksLikeParamValue(s, baseline) {
			continue
		}
		reduced := append(append([]string{}, segs[:i]...), segs[i+1:]...)
		p := "/" + strings.Join(reduced, "/")
		lines = append(lines, Line{
			Comment: fmt.Sprintf("%s structural=drop-segment-%d", tag, i),
			Method:  "GET",
			URL:     buildURL(base, p, requiredQuery),
		})
	}

	// Duplicate each param segment once (insert a copy right after it).
	for i, s := range segs {
		if !looksLikeParamValue(s, baseline) {
			continue
		}
		dup := append([]string{}, segs[:i+1]...)
		dup = append(dup, s)
		dup = append(dup, segs[i+1:]...)
		p := "/" + strings.Join(dup, "/")
		lines = append(lines, Line{
			Comment: fmt.Sprintf("%s structural=duplicate-segment-%d", tag, i),
			Method:  "GET",
			URL:     buildURL(base, p, requiredQuery),
		})
	}

	// Insert one bogus extra segment right before the last literal
	// component (e.g. before "manifests"/"blobs"/"tags"/"list"), and one
	// appended at the very end.
	if len(segs) > 0 {
		withExtraEnd := append(append([]string{}, segs...), "extra-injected-segment")
		lines = append(lines, Line{
			Comment: tag + " structural=extra-trailing-segment",
			Method:  "GET",
			URL:     buildURL(base, "/"+strings.Join(withExtraEnd, "/"), requiredQuery),
		})
		mid := len(segs) / 2
		withExtraMid := append(append([]string{}, segs[:mid]...), "extra-injected-segment")
		withExtraMid = append(withExtraMid, segs[mid:]...)
		lines = append(lines, Line{
			Comment: tag + " structural=extra-mid-segment",
			Method:  "GET",
			URL:     buildURL(base, "/"+strings.Join(withExtraMid, "/"), requiredQuery),
		})
	}

	return lines
}

func looksLikeParamValue(seg string, baseline map[string]string) bool {
	for _, v := range baseline {
		if seg == v {
			return true
		}
	}
	return false
}

func cloneMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
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

// --- Undefined endpoint sampling ---

var commonProbePaths = []string{
	"/v1/",
	"/v3/",
	"/v2",
	"/V2/",
	"/v2/..",
	"/v2/../v2/",
	"/v2//",
	"/.git/config",
	"/.env",
	"/admin",
	"/console",
	"/server-status",
	"/actuator/env",
	"/actuator/health",
	"/wp-login.php",
	"/favicon.ico",
	"/robots.txt",
	"/metrics",
	"/debug/pprof/",
	"/../etc/passwd",
	"/v2/%2e%2e/%2e%2e/etc/passwd",
	"/v2/health",
	"/healthz",
}

var randomSegmentAlphabet = []string{
	"pdq", "xyz", "foo", "bar", "abc123", "aBcDe", "nonexistent-repo",
	"..", ".", "%00", "a b", "'; --", "<x>", "0", "-1", "sha256:zz",
}

func undefinedEndpointSamples(base string, rng *rand.Rand, stress int) []Line {
	var lines []Line

	// Static probe list, scaled by stress level.
	n := 5 + stress/5
	if n > len(commonProbePaths) {
		n = len(commonProbePaths)
	}
	for i := 0; i < n; i++ {
		lines = append(lines, Line{
			Comment: "undefined-probe: static",
			Method:  "GET",
			URL:     base + commonProbePaths[i],
		})
	}

	// Randomly generated undefined /v2/... shaped paths and fully random
	// top-level paths, count scaled by stress level.
	count := 5 + stress*2
	if count > 500 {
		count = 500
	}
	methods := []string{"GET", "HEAD", "PUT", "POST", "DELETE", "PATCH"}
	suffixes := []string{"", "/manifests/latest", "/blobs/sha256:" + strings.Repeat("a", 64), "/tags/list"}
	for i := 0; i < count; i++ {
		depth := 1 + rng.Intn(4)
		var segs []string
		underV2 := rng.Intn(2) == 0
		if underV2 {
			segs = append(segs, "v2")
		}
		for d := 0; d < depth; d++ {
			segs = append(segs, randomSegmentAlphabet[rng.Intn(len(randomSegmentAlphabet))])
		}
		p := "/" + strings.Join(segs, "/")
		if underV2 {
			p += suffixes[rng.Intn(len(suffixes))]
		}
		method := methods[rng.Intn(len(methods))]
		lines = append(lines, Line{
			Comment: "undefined-probe: random",
			Method:  method,
			URL:     base + p,
		})
	}

	return lines
}

// Write writes lines to opt.OutPath, truncating/creating it fresh.
func Write(lines []Line, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()
	for _, l := range lines {
		if l.Comment != "" {
			fmt.Fprintf(w, "# %s\n", l.Comment)
		}
		if l.Method == "" && l.URL == "" {
			// Comment-only line (e.g. a SKIPPED marker) - no request to emit.
			continue
		}
		fmt.Fprintf(w, "%s\t%s\n", l.Method, l.URL)
	}
	return nil
}
