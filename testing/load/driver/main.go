package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// var re = regexp.MustCompile(`^[a-zA-Z0-9]{10}-[a-zA-Z0-9]{10}-[0-9]{4}:v[0-9]{1,3}\.[0-9]{1,3}`)
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
	}

	// Print parsed configuration
	fmt.Println("Parsed Configuration:")
	fmt.Printf("  Prune:            %v\n", config.Prune)
	fmt.Printf("  IterationSeconds: %d\n", config.IterationSeconds)
	fmt.Printf("  Patterns:         %v\n", config.Patterns)
	fmt.Printf("  MetricsFile:      %s\n", config.MetricsFile)
	fmt.Printf("  LogFile:          %s\n", config.LogFile)
	fmt.Printf("  RegistryURL:      %s\n", config.RegistryURL)
	fmt.Printf("  Filter:           %s\n", config.Filter)

	if config.Filter != "" {
		re = regexp.MustCompile(config.Filter)
	}

	// get all images
	images := getImages(config.RegistryURL, re)

	if err := createFiles(config.MetricsFile, config.LogFile); err != nil {
		os.Exit(1)
	}

	// run tests
	runTests(testRun{
		filters:     config.Patterns,
		images:      images,
		registry:    config.RegistryURL,
		metricsFile: config.MetricsFile,
		logFile:     config.LogFile,
		prune:       config.Prune,
	})
}

func createFiles(metricsFile, logFile string) error {
	for _, filePath := range []string{metricsFile, logFile} {
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directories: %w", err)
		}
		if _, err := os.Create(filePath); err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
	}
	return nil
}
