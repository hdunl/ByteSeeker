package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type ScanResult struct {
	URL          string `json:"url" xml:"url"`
	StatusCode   int    `json:"status_code" xml:"status_code"`
	Status       string `json:"status" xml:"status"`
	ErrorMessage string `json:"error_message,omitempty" xml:"error_message,omitempty"`
}

func scanTarget(client *http.Client, baseURL, path string, wg *sync.WaitGroup, results chan<- ScanResult, userAgents []string, adaptiveDelay *time.Duration) {
	defer wg.Done()

	userAgent := userAgents[rand.Intn(len(userAgents))]

	var resp *http.Response
	var err error
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), client.Timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
		if err != nil {
			cancel()
			if !strings.Contains(err.Error(), "404") {
				results <- ScanResult{URL: baseURL + path, ErrorMessage: fmt.Sprintf("Error creating request for %s: %s\n", path, err)}
			}
			return
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err = client.Do(req)
		cancel()
		if err == nil && resp != nil {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				break
			}
			continue
		}
		time.Sleep(*adaptiveDelay)
		*adaptiveDelay *= 2
	}

	if err != nil {
		// Only send non-404 errors to the results channel
		if !strings.Contains(err.Error(), "404") {
			results <- ScanResult{URL: baseURL + path, ErrorMessage: fmt.Sprintf("Error scanning %s: %s\n", path, err)}
		}
		return
	}

	if resp != nil && resp.StatusCode != http.StatusNotFound {
		status := "Unknown"
		switch resp.StatusCode {
		case http.StatusOK:
			status = "Found"
		case http.StatusTooManyRequests:
			status = "Rate limited"
		}
		results <- ScanResult{URL: baseURL + path, StatusCode: resp.StatusCode, Status: status}
	}
}

func runScanner(config ScannerConfig) {
	paths, err := loadPaths(config.pathsFile)
	if err != nil {
		fmt.Printf("Failed to load paths: %s\n", err)
		return
	}

	dnsCache := newDNSCache()
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		DialContext:         dnsCache.cachedDialContext,
	}
	client := &http.Client{
		Timeout:   config.timeout,
		Transport: transport,
	}

	var wg sync.WaitGroup
	tasksChan := make(chan string, config.concurrent)
	resultsChan := make(chan ScanResult, config.concurrent)

	for i := 0; i < config.concurrent; i++ {
		go func() {
			for path := range tasksChan {
				wg.Add(1)
				scanTarget(client, config.baseURL, path, &wg, resultsChan, config.userAgents, &config.adaptiveDelay)
			}
		}()
	}

	var results []ScanResult
	go func() {
		for result := range resultsChan {
			results = append(results, result)
		}
	}()

	startTime := time.Now()

	for _, path := range paths {
		tasksChan <- path
	}
	close(tasksChan)

	wg.Wait()
	close(resultsChan)

	duration := time.Since(startTime)
	fmt.Printf("Checked %d URLs in %s using %d goroutines\n", len(paths), duration, config.concurrent)

	var outputFile *os.File
	if config.outputFile != "" {
		outputFile, err = os.Create(config.outputFile)
		if err != nil {
			log.Fatalf("Failed to create output file: %v", err)
		}
		defer outputFile.Close()
	} else {
		log.Fatal("Output filename is required when specifying an output format")
	}

	switch config.outputFormat {
	case "json":
		output, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			log.Fatalf("Failed to generate JSON output: %v", err)
		}
		outputFile.WriteString(string(output))
	case "xml":
		output, err := xml.MarshalIndent(results, "", "  ")
		if err != nil {
			log.Fatalf("Failed to generate XML output: %v", err)
		}
		outputFile.WriteString(xml.Header + string(output))
	default: // text
		for _, result := range results {
			if result.ErrorMessage != "" {
				outputFile.WriteString(result.ErrorMessage + "\n")
			} else {
				outputFile.WriteString(fmt.Sprintf("URL: %s, Status: %s, HTTP Status Code: %d\n", result.URL, result.Status, result.StatusCode))
			}
		}
	}

	fmt.Printf("Saved results to %s\n", config.outputFile)
}
