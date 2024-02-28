package main

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"
)

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
