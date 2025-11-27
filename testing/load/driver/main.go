package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// This regex will pull all images created by the testing/load/maketar
// script: "^[a-zA-Z0-9]{10}-[a-zA-Z0-9]{10}-[0-9]{4}:v[0-9]{1,3}\.[0-9]{1,3}"
var re *regexp.Regexp

func main() {
	config, err := ParseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		PrintUsage()
		os.Exit(1)
	}

	// Validate required arguments
	if err := config.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		PrintUsage()
		os.Exit(1)
	} else {
		fmt.Println("Parsed Configuration:")
		fmt.Printf("%-20s%v\n", "  Prune:", config.prune)
		fmt.Printf("%-20s%v\n", "  DryRun:", config.dryRun)
		fmt.Printf("%-20s%v\n", "  Shuffle:", config.shuffle)
		fmt.Printf("%-20s%d\n", "  IterationSeconds:", config.iterationSeconds)
		fmt.Printf("%-20s%d\n", "  TallySeconds:", config.tallySeconds)
		fmt.Printf("%-20s%v\n", "  Patterns:", config.patterns)
		fmt.Printf("%-20s%s\n", "  MetricsFile:", config.metricsFile)
		fmt.Printf("%-20s%s\n", "  LogFile:", config.logFile)
		fmt.Printf("%-20s%s\n", "  RegistryURL:", config.registryURL)
		fmt.Printf("%-20s%s\n", "  PullthroughURL:", config.pullthroughURL)
		fmt.Printf("%-20s%s\n", "  Filter:", config.filter)
	}

	if config.filter != "" {
		if re, err = regexp.Compile(config.filter); err != nil {
			fmt.Fprintf(os.Stderr, "filter is not a valid regex to go: %s\n", config.filter)
			os.Exit(1)
		}
	}

	// get all images from the upstream registry
	config.images, err = getImages(config.registryURL, re)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting images from registry: %s\n", err)
		os.Exit(1)
	}

	// create log files if specified
	if err := createFiles(config.metricsFile, config.logFile); err != nil {
		fmt.Fprintf(os.Stderr, "error creating logging files: %s\n", err)
		os.Exit(1)
	}

	// run the test driver
	if runTests(config) != nil {
		fmt.Printf("Error running tests: %s\n", err)
	}
}

// createFiles ensures the path and files for metrics and logging exist. If either of the
// passed file paths is the empty string then no file is created for that path. Supports
// logging to stdout.
func createFiles(metricsFile, logFile string) error {
	for _, filePath := range []string{metricsFile, logFile} {
		if filePath != "" {
			dir := filepath.Dir(filePath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directories: %w", err)
			}
			if _, err := os.Create(filePath); err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}
		}
	}
	return nil
}
