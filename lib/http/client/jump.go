package httpclient

/**
Jump Service Client
-------------------

Resolves human-readable .i2p hostnames to full I2P destinations via HTTP
jump services. In I2P, hostnames like "forum.i2p" are not DNS-resolvable;
they require an address book or jump service to translate them into base32
or base64 destinations that the SAM bridge can connect to.

```
[Browser] -> http://forum.i2p/page
                    |
             [Jump Service Check]
                    |
         Is it *.b32.i2p? ── Yes ──> Direct dial (SAM resolves it)
                    |
                   No
                    |
         Query jump service: http://stats.i2p/cgi-bin/jump.cgi?q=forum.i2p
                    |
         Parse base64 destination from response
                    |
         Cache result with TTL
                    |
         Rewrite host to base32 and dial
```

This mirrors the Java I2P HTTP client tunnel's jump service behavior.
The jump service URL is configurable. Default: http://stats.i2p/cgi-bin/jump.cgi
**/

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DefaultJumpServiceURL is the standard I2P jump service endpoint.
// stats.i2p is a well-known and reliable I2P service that provides
// hostname-to-destination resolution.
const DefaultJumpServiceURL = "http://stats.i2p/cgi-bin/jump.cgi"

// defaultCacheTTL is how long resolved addresses stay cached before
// requiring a fresh lookup. 1 hour balances freshness with performance.
const defaultCacheTTL = 1 * time.Hour

// maxCacheEntries prevents unbounded memory growth from cached lookups.
const maxCacheEntries = 1000

// jumpCacheEntry stores a resolved address with its expiration time.
type jumpCacheEntry struct {
	// destination is the resolved base64 or base32 I2P destination.
	destination string
	// expiresAt is when this cache entry should be considered stale.
	expiresAt time.Time
}

// JumpService resolves human-readable .i2p hostnames by querying a
// configurable jump service over HTTP. Results are cached with a TTL
// to avoid redundant network lookups.
//
// Thread-safe: all methods are safe for concurrent use.
type JumpService struct {
	// url is the jump service endpoint (e.g., "http://stats.i2p/cgi-bin/jump.cgi").
	url string
	// client performs HTTP requests to the jump service.
	// This is the I2P-routed HTTP client, NOT a clearnet client.
	client *http.Client
	// cacheTTL controls how long resolved addresses remain valid.
	cacheTTL time.Duration
	// mu protects the cache map from concurrent access.
	mu sync.RWMutex
	// cache maps hostnames to their resolved destinations.
	cache map[string]jumpCacheEntry
}

// NewJumpService creates a jump service client with the given HTTP client
// and endpoint URL. The HTTP client should be configured to route through
// I2P (via the SAM bridge), since the jump service itself lives on I2P.
//
// If url is empty, DefaultJumpServiceURL is used.
// If client is nil, http.DefaultClient is used (typically wrong for I2P;
// the caller should provide an I2P-routed client).
func NewJumpService(client *http.Client, url string) *JumpService {
	if url == "" {
		url = DefaultJumpServiceURL
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &JumpService{
		url:      url,
		client:   client,
		cacheTTL: defaultCacheTTL,
		cache:    make(map[string]jumpCacheEntry),
	}
}

// SetCacheTTL changes how long resolved addresses stay cached.
// Zero or negative values disable caching.
func (j *JumpService) SetCacheTTL(ttl time.Duration) {
	j.mu.Lock()
	j.cacheTTL = ttl
	j.mu.Unlock()
}

// NeedsJump reports whether the given hostname requires a jump service
// lookup. Base32 addresses (*.b32.i2p) are directly resolvable by SAM
// and don't need jump service resolution. Non-I2P addresses and empty
// strings also return false.
func NeedsJump(host string) bool {
	host = stripPort(host)
	if host == "" {
		return false
	}
	lower := strings.ToLower(host)
	// Must be an .i2p address
	if !strings.HasSuffix(lower, ".i2p") {
		return false
	}
	// Base32 addresses are directly resolvable — no jump needed
	if strings.HasSuffix(lower, ".b32.i2p") {
		return false
	}
	return true
}

// Lookup resolves a human-readable .i2p hostname to its destination.
// Returns the cached result if available and not expired.
// On cache miss, queries the configured jump service.
//
// Returns the destination string (base32 or base64) and any error.
// The caller should use the returned destination to connect via SAM.
func (j *JumpService) Lookup(hostname string) (string, error) {
	hostname = stripPort(hostname)
	hostname = strings.ToLower(hostname)

	// Check cache first (fast path)
	if dest, ok := j.getCached(hostname); ok {
		return dest, nil
	}

	// Query jump service (slow path)
	dest, err := j.query(hostname)
	if err != nil {
		return "", fmt.Errorf("jump service lookup failed for %s: %w", hostname, err)
	}

	// Cache the result
	j.putCache(hostname, dest)
	return dest, nil
}

// ClearCache removes all cached entries.
func (j *JumpService) ClearCache() {
	j.mu.Lock()
	j.cache = make(map[string]jumpCacheEntry)
	j.mu.Unlock()
}

// CacheSize returns the number of entries currently in the cache.
func (j *JumpService) CacheSize() int {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return len(j.cache)
}

// URL returns the configured jump service endpoint URL.
func (j *JumpService) URL() string {
	return j.url
}

// getCached returns the cached destination for a hostname if it exists
// and hasn't expired.
func (j *JumpService) getCached(hostname string) (string, bool) {
	j.mu.RLock()
	entry, ok := j.cache[hostname]
	j.mu.RUnlock()
	if !ok {
		return "", false
	}
	if time.Now().After(entry.expiresAt) {
		// Expired — remove lazily
		j.mu.Lock()
		delete(j.cache, hostname)
		j.mu.Unlock()
		return "", false
	}
	return entry.destination, true
}

// putCache stores a resolved destination in the cache.
// Evicts oldest entries if the cache exceeds maxCacheEntries.
func (j *JumpService) putCache(hostname, destination string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	// Skip caching if TTL is non-positive
	if j.cacheTTL <= 0 {
		return
	}

	// Simple eviction: if at capacity, clear the entire cache.
	// A more sophisticated LRU isn't warranted for typical usage
	// (hundreds of unique hostnames at most).
	if len(j.cache) >= maxCacheEntries {
		j.cache = make(map[string]jumpCacheEntry)
	}

	j.cache[hostname] = jumpCacheEntry{
		destination: destination,
		expiresAt:   time.Now().Add(j.cacheTTL),
	}
}

// maxResponseBody limits how much data we read from the jump service
// to prevent abuse or accidental large responses.
const maxResponseBody = 4096

// query sends an HTTP GET to the jump service and parses the response.
//
// Jump services typically respond with either:
//   - A redirect (302) to the resolved base32 address
//   - A plain-text body containing the base64 destination
//   - An error page if the hostname is unknown
//
// This implementation handles all three cases.
func (j *JumpService) query(hostname string) (string, error) {
	reqURL := j.url + "?q=" + hostname

	// Build request manually to avoid following redirects automatically.
	// Jump services often respond with a 302 redirect to the resolved
	// address, which we want to capture rather than follow.
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Use a client that doesn't follow redirects so we can inspect
	// the Location header for the resolved address.
	noRedirectClient := *j.client
	noRedirectClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := noRedirectClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("jump service request failed: %w", err)
	}
	defer resp.Body.Close()

	return parseJumpResponse(resp, hostname)
}

// parseJumpResponse extracts the resolved destination from a jump
// service HTTP response. Handles redirects, plain text bodies,
// and error responses.
func parseJumpResponse(resp *http.Response, hostname string) (string, error) {
	// Case 1: Redirect — Location header contains the resolved URL.
	// The new host in the redirect URL is the base32 address.
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		location := resp.Header.Get("Location")
		if location == "" {
			return "", fmt.Errorf("jump service returned redirect without Location header")
		}
		// Extract the host from the redirect URL
		dest := extractHostFromURL(location)
		if dest == "" {
			return "", fmt.Errorf("could not extract destination from redirect: %s", location)
		}
		return dest, nil
	}

	// Case 2: Successful response with body containing the destination
	if resp.StatusCode == http.StatusOK {
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
		if err != nil {
			return "", fmt.Errorf("failed to read jump service response: %w", err)
		}
		dest := parseDestinationFromBody(string(body), hostname)
		if dest == "" {
			return "", fmt.Errorf("jump service returned no destination for %s", hostname)
		}
		return dest, nil
	}

	// Case 3: Error response
	return "", fmt.Errorf("jump service returned status %d for %s", resp.StatusCode, hostname)
}

// parseDestinationFromBody tries to extract a base64 I2P destination
// from the jump service response body. Jump services may return:
//   - Just the base64 destination string
//   - "hostname=base64destination" format
//   - An HTML page with the destination embedded
func parseDestinationFromBody(body, hostname string) string {
	body = strings.TrimSpace(body)

	// Format: "hostname=base64destination"
	if strings.Contains(body, "=") {
		parts := strings.SplitN(body, "=", 2)
		if len(parts) == 2 {
			dest := strings.TrimSpace(parts[1])
			if isI2PDestination(dest) {
				return dest
			}
		}
	}

	// Format: plain base64 destination
	if isI2PDestination(body) {
		return body
	}

	// Format: look for base64 destination in HTML or mixed content
	// Search for a long base64-like string (I2P destinations are 516+ chars)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if isI2PDestination(line) {
			return line
		}
	}

	return ""
}

// isI2PDestination checks if a string looks like a valid I2P base64
// destination. I2P destinations are 516+ character base64 strings
// ending with "AAAA" (the null certificate).
func isI2PDestination(s string) bool {
	s = strings.TrimSpace(s)
	// I2P destinations are at least 516 characters of base64
	if len(s) < 516 {
		return false
	}
	// Must end with the I2P destination suffix
	if !strings.HasSuffix(s, "AAAA") {
		return false
	}
	// Basic base64 character check
	for _, c := range s {
		if !isBase64Char(c) {
			return false
		}
	}
	return true
}

// isBase64Char reports whether c is a valid base64 character.
func isBase64Char(c rune) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '+' || c == '/' || c == '=' ||
		c == '-' || c == '~' // I2P uses modified base64
}

// extractHostFromURL extracts the hostname from a URL string.
// Used to parse redirect Location headers from jump services.
func extractHostFromURL(rawURL string) string {
	// Remove scheme
	if idx := strings.Index(rawURL, "://"); idx >= 0 {
		rawURL = rawURL[idx+3:]
	}
	// Remove path
	if idx := strings.Index(rawURL, "/"); idx >= 0 {
		rawURL = rawURL[:idx]
	}
	// Remove port
	return stripPort(rawURL)
}

// stripPort removes the :port suffix from a host string if present.
func stripPort(host string) string {
	if idx := strings.LastIndex(host, ":"); idx >= 0 {
		return host[:idx]
	}
	return host
}
