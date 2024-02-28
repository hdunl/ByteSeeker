package main

import (
	"flag"
	"fmt"
	"time"
)

func main() {
	baseURL := flag.String("url", "", "Base URL to scan")
	pathsFile := flag.String("paths", "", "File containing paths to scan")
	concurrent := flag.Int("concurrent", 10, "Number of concurrent goroutines for scanning")
	timeout := flag.Duration("timeout", 10*time.Second, "HTTP request timeout")
	adaptiveDelay := flag.Duration("adaptiveDelay", 100*time.Millisecond, "Initial adaptive delay between requests")
	outputFormat := flag.String("o", "text", "Output format: text, json, xml")
	outputFilename := flag.String("f", "", "Filename to save the output results")
	flag.Parse()

	if *baseURL == "" || *pathsFile == "" || *outputFilename == "" {
		fmt.Println("URL, paths, and output filename flags are required")
		flag.Usage()
		return
	}

	config := ScannerConfig{
		baseURL:       *baseURL,
		pathsFile:     *pathsFile,
		concurrent:    *concurrent,
		timeout:       *timeout,
		userAgents:    defaultUserAgents,
		adaptiveDelay: *adaptiveDelay,
		outputFormat:  *outputFormat,
		outputFile:    *outputFilename,
	}

	runScanner(config)
}
