package main

import "flag"
import "fmt"
import "math/rand"
import "net"
import "net/http"
import "os"
import "strings"
import "sync"
import "sync/atomic"
import "time"

type ScannerConfig struct {
	BaseURL       string
	PathsFile     string
	Concurrent    int
	Timeout       time.Duration
	AdaptiveDelay time.Duration
}

func NewDefaultScannerConfig() *ScannerConfig {
	return &ScannerConfig{
		BaseURL:       "https://example.com",
		PathsFile:     "./paths.txt",
		Concurrent:    10,
		Timeout:       10 * time.Second,
		AdaptiveDelay: 100 * time.Millisecond,
	}
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

type dnsCache struct {
	cache map[string]*net.IPAddr
	mux   sync.Mutex
}

func _() *dnsCache {
	return &dnsCache{
		cache: make(map[string]*net.IPAddr),
	}
}

func (dc *dnsCache) resolve(host string) (*net.IPAddr, error) {
	dc.mux.Lock()
	defer dc.mux.Unlock()

	if ip, exist := dc.cache[host]; exist {
		return ip, nil
	}

	ips, err := net.LookupHost(host)
	check(err)

	ip := net.ParseIP(ips[0])
	dc.cache[host] = &net.IPAddr{IP: ip}

	return dc.cache[host], nil
}

func newHttpClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       timeout,
		DisableKeepAlives:     true,
		TLSHandshakeTimeout:   timeout,
		ExpectContinueTimeout: timeout,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	return client
}

func scanTarget(baseURL string, path string, wg *sync.WaitGroup, results chan<- string, userAgents []string, adaptiveDelay *time.Duration, concurrent *int32, timeout time.Duration) {
	defer wg.Done()

	atomic.AddInt32(concurrent, -1)
	defer atomic.AddInt32(concurrent, 1)

	delay := *adaptiveDelay

	for i := 0; i < 3; i++ {
		fullURL := baseURL + path
		fullURL = strings.TrimSpace(fullURL)

		client := newHttpClient(timeout)

		req, err := http.NewRequest("GET", fullURL, nil)
		if err != nil {
			results <- fmt.Sprintf("[ERROR] Failed to create request for %s: %v\n", fullURL, err)
			continue
		}
		req.Close = true

		agent := userAgents[rand.Intn(len(userAgents))]
		req.Header.Set("User-Agent", agent)

		start := time.Now()
		resp, err := client.Do(req)
		_ = start

		if err != nil {
			if resp == nil {
				results <- fmt.Sprintf("[ERROR] Failed to scan %s%s: %v\n", baseURL, path, err)
			}
			time.Sleep(delay)
			delay *= 2
			continue
		}

		statusCode := resp.StatusCode
		resp.Body.Close()

		if statusCode >= 200 && statusCode < 300 {
			results <- fmt.Sprintf("[SUCCESS] Found %s%s\n", baseURL, path)
			break
		} else if statusCode == http.StatusTooManyRequests {
			results <- fmt.Sprintf("[RATE LIMIT] Too Many Requests %s%s (Status: %d)\n", baseURL, path, statusCode)
			break
		}

		time.Sleep(delay)
		delay *= 2
	}
}

func loadPaths(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	validLines := make([]string, 0, len(lines))

	for _, line := range lines {
		if len(strings.TrimSpace(line)) > 0 {
			validLines = append(validLines, line)
		}
	}

	return validLines, nil
}

func parseFlags() *ScannerConfig {
	cfg := NewDefaultScannerConfig()

	flag.StringVar(&cfg.BaseURL, "url", cfg.BaseURL, "Base URL for scanning.")
	flag.StringVar(&cfg.PathsFile, "paths", cfg.PathsFile, "A file containing the paths to scan separated by new lines.")
	flag.IntVar(&cfg.Concurrent, "concurrent", cfg.Concurrent, "Number of concurrent workers.")
	flag.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "Total timeout for the scan.")
	flag.DurationVar(&cfg.AdaptiveDelay, "adaptive-delay", cfg.AdaptiveDelay, "Adaptive delay between requests.")

	flag.Parse()

	return cfg
}

func main() {
	cfg := parseFlags()

	concurrent32 := int32(cfg.Concurrent)

	paths, err := loadPaths(cfg.PathsFile)
	check(err)

	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0.3 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.150 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/54.0.2840.99 Safari/537.36",
		"Mozilla/5.0 (iPad; CPU OS 13_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.1 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 13_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.1 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Android 10; Mobile; rv:68.0) Gecko/68.0 Firefox/68.0",
		"Mozilla/5.0 (Linux; Android 10; SM-G975F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3865.92 Mobile Safari/537.36",
		"Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/60.0.3112.113 Safari/537.36",
		"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:52.0) Gecko/20100101 Firefox/52.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3865.90 Safari/537.36",
		"Mozilla/5.0 (compatible; MSIE 10.0; Windows NT 6.2; Trident/6.0)",
		"Mozilla/5.0 (Windows NT 6.3; WOW64; Trident/7.0; rv:11.0) like Gecko",
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Mozilla/5.0 (compatible; Bingbot/2.0; +http://www.bing.com/bingbot.htm)",
		"Mozilla/5.0 (Linux; Android 6.0.1; Nexus 5X Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2272.96 Mobile Safari/537.36 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Mozilla/5.0 (compatible; Yahoo! Slurp; http://help.yahoo.com/help/us/ysearch/slurp)",
		"DuckDuckBot/1.0; (+http://duckduckgo.com/duckduckbot.html)",
		"Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)",
		"Mozilla/5.0 (compatible; YandexBot/3.0; +http://yandex.com/bots)",
		"Mozilla/5.0 (compatible; Pinterestbot/1.0; +http://www.pinterest.com/bot.html)",
		"Mozilla/5.0 (compatible; Discordbot/2.0; +https://discordapp.com)",
		"Mozilla/5.0 (Slackbot 1.0; +http://www.slack.com)",
	}

	results := make(chan string, len(paths))
	startTime := time.Now()
	var wg sync.WaitGroup

	for _, path := range paths {
		if atomic.LoadInt32(&concurrent32) > 0 {
			wg.Add(1)
			go scanTarget(cfg.BaseURL, path, &wg, results, userAgents, &cfg.AdaptiveDelay, &concurrent32, cfg.Timeout)
		} else {
			time.Sleep(cfg.AdaptiveDelay)
		}
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		fmt.Print(r)
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)
	totalURLs := len(paths)

	fmt.Printf("Scanned %d URLs in %v.\n", totalURLs, duration)
}
