// Command ociregistry-fuzz is a security test utility for OCI Distribution
// Server REST APIs. It has three modes:
//
//	--generate-endpoints    build test URLs from an OpenAPI spec
//	--generate-from-urls    build subtle mutations of known-good URLs
//	--run-endpoints         replay a generated file against a live server
//
// See README.md for full usage and examples.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"ociregistry-fuzz/genendpoints"
	"ociregistry-fuzz/genfromurls"
	"ociregistry-fuzz/runner"
	"ociregistry-fuzz/spec"
)

// stringListFlag collects repeated -flag values (and/or comma-separated
// values within a single occurrence) into a slice.
type stringListFlag struct {
	values []string
}

func (s *stringListFlag) String() string {
	return strings.Join(s.values, ",")
}

func (s *stringListFlag) Set(v string) error {
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			s.values = append(s.values, part)
		}
	}
	return nil
}

func main() {
	var (
		generateEndpoints = flag.Bool("generate-endpoints", false,
			"Generate test URLs from the OpenAPI spec (defined endpoints + mutations + a random sample of undefined endpoints).")
		generateFromURLs = flag.String("generate-from-urls", "",
			"Path to a file of known-good, working URLs (one per line). Subtle alterations of each are appended to --out.")
		runEndpoints = flag.Bool("run-endpoints", false,
			"Replay the lines in --in against the live server and record results.")

		specPath = flag.String("spec", "ociregistry.yaml",
			"Path to the OpenAPI 3.0.3 spec (used with --generate-endpoints).")
		outPath = flag.String("out", "generated_urls.txt",
			"Output file for generated test lines. Overwritten by --generate-endpoints, appended to by --generate-from-urls.")
		inPath = flag.String("in", "generated_urls.txt",
			"Input file of generated test lines (used with --run-endpoints and defaults to --out's default).")
		resultsPath = flag.String("results", "",
			"Output CSV file for --run-endpoints results. Default: stdout.")
		host = flag.String("host", "localhost:8080",
			"host:port of the target registry (used with --generate-endpoints).")
		scheme = flag.String("scheme", "http",
			"URL scheme to generate. Only 'http' is currently supported for --generate-endpoints.")
		stressLevel = flag.Int("stress-level", 1,
			"1-100. Higher values generate more, larger, and more extreme test-case variations.")
		timeout = flag.Duration("timeout", 10*time.Second,
			"Per-request timeout for --run-endpoints (e.g. 10s, 500ms).")
		seed = flag.Int64("seed", time.Now().UnixNano(),
			"Random seed for reproducible generation. Defaults to current time.")
		maxBodyBytes = flag.Int64("max-body-bytes", 0,
			"Cap on response bytes counted per request for --run-endpoints (0 = unlimited; the full body is always read/discarded to free the connection).")

		excludePath stringListFlag
		noDefaultExcludes = flag.Bool("no-default-excludes", false,
			"Don't exclude the built-in default set of unsafe operations (currently: GET /cmd/stop). Not recommended.")
	)
	flag.Var(&excludePath, "exclude-path", "Operation to exclude entirely from --generate-endpoints "+
		"(no baseline, no mutated requests at all - not even a bogus-query-param request against the exact real path). "+
		"May be repeated, or comma-separated. Form: \"/path/template\" (all methods) or \"METHOD:/path/template\". "+
		"GET /cmd/stop is always excluded by default since it actually shuts the server down; see --no-default-excludes.")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "ociregistry-fuzz: security-oriented URL fuzz tester for OCI distribution servers\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s --generate-endpoints [--spec FILE] [--host HOST:PORT] [--stress-level N] [--out FILE]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --generate-from-urls FILE [--stress-level N] [--out FILE]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --run-endpoints [--in FILE] [--timeout DUR] [--results FILE]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	modeCount := 0
	if *generateEndpoints {
		modeCount++
	}
	if *generateFromURLs != "" {
		modeCount++
	}
	if *runEndpoints {
		modeCount++
	}
	if modeCount != 1 {
		fmt.Fprintln(os.Stderr, "error: specify exactly one of --generate-endpoints, --generate-from-urls, --run-endpoints")
		flag.Usage()
		os.Exit(2)
	}

	if *stressLevel < 1 || *stressLevel > 100 {
		fmt.Fprintln(os.Stderr, "error: --stress-level must be between 1 and 100")
		os.Exit(2)
	}

	switch {
	case *generateEndpoints:
		if *scheme != "http" {
			fmt.Fprintln(os.Stderr, "error: --scheme only supports 'http' at this time for --generate-endpoints")
			os.Exit(2)
		}
		sp, err := spec.ParseFile(*specPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: parsing spec: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "parsed %d path templates from %s\n", len(sp.Paths), *specPath)

		exclude := excludePath.values
		if !*noDefaultExcludes {
			exclude = append(append([]string{}, genendpoints.DefaultExcludes...), exclude...)
		}
		if len(exclude) > 0 {
			fmt.Fprintf(os.Stderr, "excluding operations: %s\n", strings.Join(exclude, ", "))
		}

		lines := genendpoints.Generate(sp, genendpoints.Options{
			Host:        *host,
			Scheme:      *scheme,
			StressLevel: *stressLevel,
			Seed:        *seed,
			Exclude:     exclude,
		})
		if err := genendpoints.Write(lines, *outPath); err != nil {
			fmt.Fprintf(os.Stderr, "error: writing output: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "wrote %d test lines to %s (seed=%d, stress-level=%d)\n", len(lines), *outPath, *seed, *stressLevel)

	case *generateFromURLs != "":
		lines, err := genfromurls.Generate(*generateFromURLs, *stressLevel, *seed)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if err := genfromurls.Append(lines, *outPath); err != nil {
			fmt.Fprintf(os.Stderr, "error: writing output: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "appended %d mutated test lines to %s (seed=%d, stress-level=%d)\n", len(lines), *outPath, *seed, *stressLevel)

	case *runEndpoints:
		results, halted, err := runner.Run(runner.Options{
			InPath:       *inPath,
			ResultsPath:  *resultsPath,
			Timeout:      *timeout,
			MaxBodyBytes: *maxBodyBytes,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "ran %d requests from %s", len(results), *inPath)
		if halted {
			fmt.Fprintf(os.Stderr, " (halted early due to consecutive timeouts)")
		}
		fmt.Fprintln(os.Stderr)
	}
}
