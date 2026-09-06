// Package fuzzcorpus generates malicious/edge-case string values used to
// mutate path segments and query parameter values. The size and
// "extremity" of the corpus scales with a 1-100 stress level: level 1
// produces a small, cheap set; level 100 produces a large set including
// very large payloads and more exotic encodings.
package fuzzcorpus

import "strings"

// Case is one named fuzz value. Label is used only for human-readable
// comments in generated output files.
type Case struct {
	Label string
	Value string
}

func clampStress(stress int) int {
	if stress < 1 {
		return 1
	}
	if stress > 100 {
		return 100
	}
	return stress
}

// scaledLengths returns the set of "large value" byte lengths to test at
// the given stress level. 1000 bytes is always included per the baseline
// requirement; higher stress adds larger and more numerous tiers.
func scaledLengths(stress int) []int {
	lengths := []int{1000}
	switch {
	case stress >= 90:
		lengths = append(lengths, 10_000, 100_000, 500_000)
	case stress >= 70:
		lengths = append(lengths, 10_000, 100_000)
	case stress >= 40:
		lengths = append(lengths, 10_000, 65_536)
	case stress >= 15:
		lengths = append(lengths, 8_000)
	case stress >= 5:
		lengths = append(lengths, 2_000)
	}
	return lengths
}

// GenericCases returns generic string-value fuzz cases suitable for any
// path segment or query parameter value, regardless of declared type.
func GenericCases(stress int) []Case {
	stress = clampStress(stress)
	var out []Case

	out = append(out,
		Case{"empty", ""},
		Case{"single-char", "a"},
		Case{"normal", "test-repo123"},
	)

	for _, n := range scaledLengths(stress) {
		out = append(out, Case{label("long", n), strings.Repeat("A", n)})
	}

	out = append(out,
		Case{"dot", "."},
		Case{"dotdot", ".."},
		Case{"traversal-basic", "../../../etc/passwd"},
		Case{"embedded-slash", "a/b"},
		Case{"uppercase", "UPPERCASE"},
		Case{"leading-dash", "-leadingdash"},
		Case{"trailing-dot", "abc."},
	)

	if stress >= 5 {
		out = append(out,
			Case{"traversal-encoded", "..%2f..%2f..%2fetc%2fpasswd"},
			Case{"null-byte", "file%00.txt"},
			Case{"whitespace", " "},
			Case{"tab", "\t"},
			Case{"newline", "line1\nline2"},
			Case{"leading-trailing-space", "  a  "},
		)
	}
	if stress >= 10 {
		out = append(out,
			Case{"traversal-double-encoded", "..%252f..%252f..%252fetc%252fpasswd"},
			Case{"encoded-slash", "a%2Fb"},
			Case{"encoded-backslash", "a%5Cb"},
			Case{"sql-injection", "' OR '1'='1"},
			Case{"html-injection", "<script>alert(1)</script>"},
			Case{"backslash-traversal", "..\\..\\..\\windows\\system32"},
			Case{"semicolon-injection", "a;b"},
			Case{"pipe-injection", "a|b"},
		)
	}
	if stress >= 15 {
		out = append(out,
			Case{"control-chars", "abc\x01\x02\x03def"},
			Case{"shell-metachar", "$(id);`whoami`|ls"},
			Case{"crlf-injection", "a\r\nX-Injected: true"},
		)
	}
	if stress >= 20 {
		out = append(out,
			Case{"emoji", "\U0001F525\U0001F600\U0001F680"},
			Case{"format-string", "%s%s%s%s%n%x%x"},
			Case{"quotes-mixed", `a"b'c`},
		)
	}
	if stress >= 25 {
		out = append(out,
			Case{"unicode-lookalike-cyrillic-a", "\u0430bc"}, // Cyrillic а
			Case{"overlong-percent", "%c0%af%c0%ae%c0%ae"},
		)
	}
	if stress >= 30 {
		out = append(out,
			Case{"template-injection", "${7*7}#{7*7}{{7*7}}<%=7*7%>"},
		)
	}
	if stress >= 40 {
		out = append(out,
			Case{"traversal-overlong-utf8", "..%c0%af..%c0%afetc%c0%afpasswd"},
			Case{"many-dots", strings.Repeat(".", 5000)},
			Case{"many-slashes-encoded", strings.Repeat("%2f", 500)},
		)
	}
	if stress >= 50 {
		out = append(out,
			Case{"double-url-encoded", "%252e%252e%252f%252e%252e%252f"},
			Case{"raw-percent", "100%rawpercent"},
			Case{"nul-and-unicode", "a\x00\u2028\u2029b"},
		)
	}
	if stress >= 60 {
		out = append(out,
			Case{"rtl-override", "abc\u202edef"},
			Case{"zero-width", "a\u200b\u200c\u200db"},
		)
	}
	return out
}

// DigestCases returns fuzz values specifically targeting the OCI
// "digest" path parameter (expected form "<algo>:<hex>").
func DigestCases(stress int) []Case {
	stress = clampStress(stress)
	out := []Case{
		{"valid-form-fake-hash", "sha256:" + strings.Repeat("0", 64)},
		{"wrong-algo", "sha1:" + strings.Repeat("a", 40)},
		{"no-colon", "sha256" + strings.Repeat("a", 64)},
		{"too-short", "sha256:abc"},
		{"too-long", "sha256:" + strings.Repeat("a", 200)},
		{"non-hex-chars", "sha256:" + strings.Repeat("z", 64)},
		{"empty-hash", "sha256:"},
		{"empty-algo", ":" + strings.Repeat("a", 64)},
	}
	if stress >= 10 {
		out = append(out,
			Case{"multiple-colons", "sha256:aa:bb:cc"},
			Case{"uppercase-hex", "sha256:" + strings.ToUpper(strings.Repeat("a", 64))},
			Case{"unknown-algo", "md5:" + strings.Repeat("a", 32)},
		)
	}
	if stress >= 30 {
		out = append(out,
			Case{"digest-with-traversal", "sha256:../../../etc/passwd"},
			Case{"digest-with-slash", "sha256:aa/bb"},
		)
	}
	return out
}

// IntegerCases returns fuzz values targeting query parameters declared as
// type "integer" (e.g. count, n).
func IntegerCases(stress int) []Case {
	stress = clampStress(stress)
	out := []Case{
		{"negative", "-1"},
		{"zero", "0"},
		{"non-numeric", "abc"},
		{"float", "1.5"},
		{"huge", "99999999999999999999999999999999"},
		{"hex-form", "0x1F"},
		{"plus-prefixed", "+5"},
		{"leading-zero", "007"},
		{"scientific", "1e10"},
		{"empty", ""},
	}
	if stress >= 20 {
		out = append(out,
			Case{"nan", "NaN"},
			Case{"infinity", "Infinity"},
			Case{"whitespace-padded", " 5 "},
		)
	}
	if stress >= 40 {
		out = append(out,
			Case{"overflow-int64", "9223372036854775808"},
			Case{"negative-overflow-int64", "-9223372036854775809"},
		)
	}
	return out
}

// EnumCases returns fuzz values for a query parameter constrained to a
// small set of valid enum-like string values (e.g. /cmd/prune's "type").
// It includes each valid value once (so the "happy path" for each is
// exercised), plus case-mangled and invalid variants.
func EnumCases(valid []string, stress int) []Case {
	stress = clampStress(stress)
	var out []Case
	for _, v := range valid {
		out = append(out, Case{"valid-" + v, v})
	}
	out = append(out,
		Case{"empty", ""},
		Case{"invalid-value", "not-a-real-value"},
		Case{"uppercase-of-first-valid", strings.ToUpper(valid[0])},
		Case{"whitespace-padded", " " + valid[0] + " "},
	)
	if stress >= 10 {
		out = append(out,
			Case{"concatenated-valid", strings.Join(valid, ",")},
			Case{"sql-injection", "' OR '1'='1"},
			Case{"long", strings.Repeat(valid[0], 500)},
		)
	}
	return out
}

// DurationCases returns fuzz values for a duration-string query parameter
// using the documented unit suffixes (d=days, h=hours, m=minutes), e.g.
// /cmd/prune's "dur" (as in "30d").
func DurationCases(stress int) []Case {
	stress = clampStress(stress)
	out := []Case{
		{"valid-days", "30d"},
		{"valid-hours", "12h"},
		{"valid-minutes", "45m"},
		{"empty", ""},
		{"zero", "0d"},
		{"negative", "-5d"},
		{"no-unit", "30"},
		{"unknown-unit", "30x"},
		{"decimal", "5.5d"},
		{"non-numeric", "abcd"},
	}
	if stress >= 15 {
		out = append(out,
			Case{"huge", "99999999999999d"},
			Case{"combined-units", "1d2h3m"}, // undocumented form; not valid per spec
			Case{"double-suffix", "30dd"},
			Case{"whitespace-padded", " 30d "},
			Case{"unit-only", "d"},
		)
	}
	return out
}

// SafeNonMatchExpr is a manifest-URL pattern engineered to match nothing
// in a real registry, used as a safety net when fuzzing other /cmd/prune
// parameters (see genendpoints' prune-specific handling).
const SafeNonMatchExpr = "zz-ociregistry-fuzz-no-such-manifest-marker-zz"

// RegexPatternCases returns fuzz values for a query parameter that is
// compiled server-side as a Go regular expression (e.g. /cmd/prune's
// "expr", /cmd/image/list's and /cmd/manifest/list's "pattern"). It
// includes classic catastrophic-backtracking ("ReDoS") shapes as well as
// invalid-syntax and oversized inputs.
func RegexPatternCases(stress int) []Case {
	stress = clampStress(stress)
	out := []Case{
		{"empty", ""},
		{"literal", "calico"},
		{"wildcard-match-all", ".*"},
		{"comma-separated", "calico,coredns"},
		{"invalid-syntax-unclosed-paren", "(unclosed"},
		{"invalid-syntax-bad-repeat", "*foo"},
	}
	if stress >= 10 {
		out = append(out,
			Case{"redos-nested-quantifiers", "(a+)+$"},
			Case{"redos-alternation", "(a|aa)+$"},
			Case{"redos-nested-quantifiers-2", "(a*)*b"},
		)
	}
	if stress >= 25 {
		out = append(out,
			Case{"many-commas", strings.Repeat("a,", 2000) + "a"},
			Case{"long-pattern", strings.Repeat("a", 20_000)},
			Case{"large-alternation", strings.Repeat("a|", 5000) + "a"},
		)
	}
	return out
}


// behave like booleans (e.g. dryRun) even though the spec types them as
// plain strings.
func BooleanishCases(stress int) []Case {
	out := []Case{
		{"true-lower", "true"},
		{"true-upper", "TRUE"},
		{"one", "1"},
		{"yes", "yes"},
		{"null-word", "null"},
		{"empty", ""},
		{"non-boolean", "maybe"},
	}
	return out
}

func label(prefix string, n int) string {
	switch {
	case n >= 1_000_000:
		return prefix + "-" + itoa(n/1_000_000) + "MB"
	case n >= 1_000:
		return prefix + "-" + itoa(n/1_000) + "KB"
	default:
		return prefix + "-" + itoa(n) + "B"
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
