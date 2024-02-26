package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type ScannerConfig struct {
	baseURL       string
	pathsFile     string
	concurrent    int
	timeout       time.Duration
	userAgents    []string
	adaptiveDelay time.Duration
}

type dnsCacheEntry struct {
	ip        string
	timestamp time.Time
}

type dnsCache struct {
	entries map[string]dnsCacheEntry
	mutex   sync.RWMutex
}

func newDNSCache() *dnsCache {
	return &dnsCache{
		entries: make(map[string]dnsCacheEntry),
	}
}

func (c *dnsCache) lookup(host string) (string, error) {
	c.mutex.RLock()
	if entry, found := c.entries[host]; found && time.Since(entry.timestamp) < 5*time.Minute {
		c.mutex.RUnlock()
		return entry.ip, nil
	}
	c.mutex.RUnlock()

	ips, err := net.LookupHost(host)
	if err != nil {
		return "", err
	}
	ip := ips[0]

	c.mutex.Lock()
	c.entries[host] = dnsCacheEntry{ip: ip, timestamp: time.Now()}
	c.mutex.Unlock()

	return ip, nil
}

func (c *dnsCache) cachedDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	separator := strings.LastIndex(addr, ":")
	host, port := addr[:separator], addr[separator:]

	ip, err := c.lookup(host)
	if err != nil {
		return nil, err
	}
	addr = ip + port

	dialer := net.Dialer{}
	return dialer.DialContext(ctx, network, addr)
}

func scanTarget(client *http.Client, baseURL, path string, wg *sync.WaitGroup, results chan<- string, userAgents []string, adaptiveDelay *time.Duration) {
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
			results <- fmt.Sprintf("Error creating request for %s: %s\n", path, err)
			return
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err = client.Do(req)
		cancel()
		if err == nil && resp != nil {
			defer resp.Body.Close()
			break
		}
		time.Sleep(*adaptiveDelay)
		*adaptiveDelay *= 2
	}

	if err != nil {
		results <- fmt.Sprintf("Error scanning %s: %s\n", path, err)
		return
	}

	if resp != nil {
		switch resp.StatusCode {
		case http.StatusOK:
			results <- fmt.Sprintf("Found: %s\n", baseURL+path)
		case http.StatusTooManyRequests:
			results <- fmt.Sprintf("Rate limited on %s\n", baseURL+path)
		default:
		}
	}
}

func loadPaths(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var paths []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		paths = append(paths, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return paths, nil
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
	resultsChan := make(chan string, config.concurrent)

	for i := 0; i < config.concurrent; i++ {
		go func() {
			for path := range tasksChan {
				wg.Add(1)
				scanTarget(client, config.baseURL, path, &wg, resultsChan, config.userAgents, &config.adaptiveDelay)
			}
		}()
	}

	go func() {
		for result := range resultsChan {
			fmt.Print(result)
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
}

func main() {
	baseURL := flag.String("url", "", "Base URL to scan")
	pathsFile := flag.String("paths", "", "File containing paths to scan")
	concurrent := flag.Int("concurrent", 10, "Number of concurrent goroutines for scanning")
	timeout := flag.Duration("timeout", 10*time.Second, "HTTP request timeout")
	adaptiveDelay := flag.Duration("adaptiveDelay", 100*time.Millisecond, "Initial adaptive delay between requests")
	flag.Parse()

	if *baseURL == "" || *pathsFile == "" {
		fmt.Println("url and paths flags are required")
		flag.Usage()
		return
	}

	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0.3 Safari/605.1.15",
		"Mozilla/5.0 (iPad; CPU OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Linux; Android 10; SM-G975F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3865.92 Mobile Safari/537.36",
		"Mozilla/5.0 (Linux; Android 10; SM-A505FN) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3865.92 Mobile Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3865.90 Safari/537.36",
		"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:68.0) Gecko/20100101 Firefox/68.0",
		"Mozilla/5.0 (Windows NT 6.1; WOW64; rv:54.0) Gecko/20100101 Firefox/54.0",
		"Mozilla/5.0 (Windows NT 10.0; WOW64; Trident/7.0; rv:11.0) like Gecko",
		"Mozilla/5.0 (compatible; MSIE 10.0; Windows NT 6.2)",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:78.0) Gecko/20100101 Firefox/78.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.97 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.150 Safari/537.36 Edge/88.0.705.50",
		"Mozilla/5.0 (Linux; Android 9; SM-G960F Build/PPR1.180610.011) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/70.0.3538.110 Mobile Safari/537.36",
		"Mozilla/5.0 (Linux; U; Android 4.4; en-us; Nexus 5 Build/KRT16M) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.4 Mobile Safari/537.36",
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Mozilla/5.0 (compatible; Bingbot/2.0; +http://www.bing.com/bingbot.htm)",
		"Mozilla/5.0 (compatible; Yahoo! Slurp; http://help.yahoo.com/help/us/ysearch/slurp)",
		"Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)",
	}

	config := ScannerConfig{
		baseURL:       *baseURL,
		pathsFile:     *pathsFile,
		concurrent:    *concurrent,
		timeout:       *timeout,
		userAgents:    userAgents,
		adaptiveDelay: *adaptiveDelay,
	}

	runScanner(config)
}
