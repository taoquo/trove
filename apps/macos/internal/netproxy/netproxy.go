// Package netproxy gives the app one HTTP transport that honours the proxy the
// machine is actually configured with.
//
// Why this exists: Go's http.Client defaults to http.ProxyFromEnvironment, which
// reads HTTP_PROXY, HTTPS_PROXY and NO_PROXY from the environment and nothing
// else. A .app launched from Finder inherits no such variables, so on a machine
// where x.com is only reachable through a local proxy every request went direct
// and died at the TLS handshake -- which the sync reported as a network failure
// while the cookies were perfectly good. macOS keeps the proxy in
// SystemConfiguration, reachable through `scutil --proxy`, and that is what this
// package reads.
//
// The environment still wins when it is set: a user who exports HTTPS_PROXY
// means it, and it is also how the test suite points the transport at a local
// server without touching the machine's settings.
package netproxy

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// ScutilBinary is the tool that prints the system proxy configuration.
const ScutilBinary = "/usr/sbin/scutil"

// Config is the proxy the machine is configured with.
type Config struct {
	// HTTP and HTTPS are the proxy URLs, empty when that scheme goes direct.
	HTTP  string
	HTTPS string
	// Exceptions are the hosts that bypass the proxy, in scutil's form: exact
	// host names, `*.domain`, or CIDR blocks.
	Exceptions []string
}

// Empty reports whether no proxy is configured at all.
func (c Config) Empty() bool { return c.HTTP == "" && c.HTTPS == "" }

// For returns the proxy for a request URL, or nil to go direct.
func (c Config) For(target *url.URL) *url.URL {
	if target == nil || c.Empty() {
		return nil
	}
	if c.bypasses(target.Hostname()) {
		return nil
	}

	raw := c.HTTP
	if target.Scheme == "https" && c.HTTPS != "" {
		raw = c.HTTPS
	}
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	return parsed
}

// bypasses reports whether the host is in the exception list.
//
// The match is on the host alone: the exceptions macOS stores are hosts and
// networks, never schemes, and a proxy that ignored them would break every
// localhost or LAN endpoint a user deliberately excluded.
func (c Config) bypasses(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	ip := net.ParseIP(host)

	for _, raw := range c.Exceptions {
		pattern := strings.ToLower(strings.TrimSpace(raw))
		if pattern == "" {
			continue
		}
		// `<local>` is macOS's shorthand for "no dots": mDNS names and simple
		// hostnames, which is what a local service is called.
		if pattern == "<local>" {
			if !strings.Contains(host, ".") {
				return true
			}
			continue
		}
		if strings.HasPrefix(pattern, "*.") {
			suffix := pattern[1:] // ".example.com"
			if strings.HasSuffix(host, suffix) {
				return true
			}
			continue
		}
		if ip != nil {
			if _, network, err := net.ParseCIDR(pattern); err == nil && network.Contains(ip) {
				return true
			}
		}
		if host == pattern {
			return true
		}
	}
	return false
}

// envConfigured reports whether the environment names a proxy.
//
// The lowercase forms are what curl and Go accept on unix; the uppercase ones are
// the conventional spelling. HTTP_PROXY is deliberately not consulted here in its
// lowercase form by Go's own resolver for security reasons (CGI), so this follows
// the same rule rather than inventing a different one.
func envConfigured() bool {
	for _, name := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "ALL_PROXY", "all_proxy"} {
		if strings.TrimSpace(os.Getenv(name)) != "" {
			return true
		}
	}
	return false
}

// system reads the machine's proxy configuration.
func system(ctx context.Context) Config {
	runner := ScutilRunner{}
	output, err := runner.Run(ctx, ScutilBinary, []string{"--proxy"})
	if err != nil {
		return Config{}
	}
	return ParseScutil(output)
}

// ParseScutil reads the dictionary `scutil --proxy` prints.
//
// The output is a plist-style block:
//
//	<dictionary> {
//	  ExceptionsList : <array> {
//	    0 : localhost
//	    1 : *.local
//	  }
//	  HTTPEnable : 1
//	  HTTPPort : 7890
//	  HTTPProxy : 127.0.0.1
//	  HTTPSEnable : 1
//	  HTTPSPort : 7890
//	  HTTPSProxy : 127.0.0.1
//	}
//
// Only the enabled schemes are read: a disabled entry is a remembered setting,
// not a live one, and using it would send traffic to a proxy the user turned off.
func ParseScutil(output string) Config {
	values := map[string]string{}
	exceptions := []string{}
	inExceptions := false
	depth := 0

	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "ExceptionsList") {
			inExceptions = true
			depth = strings.Count(line, "{") - strings.Count(line, "}")
			continue
		}
		if inExceptions {
			if strings.HasPrefix(line, "}") {
				depth--
				if depth <= 0 {
					inExceptions = false
				}
				continue
			}
			if _, value, found := strings.Cut(line, ":"); found {
				exceptions = append(exceptions, strings.TrimSpace(value))
			}
			continue
		}

		// A compact rendition puts the opening brace on the first entry's line.
		line = strings.TrimSpace(strings.TrimPrefix(line, "<dictionary>"))
		line = strings.TrimSpace(strings.TrimPrefix(line, "{"))
		if line == "" || line == "}" {
			continue
		}

		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		// The closing brace of the dictionary shares a line with the last entry
		// in a compact rendition, and it is not part of the value.
		values[strings.TrimSpace(key)] = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "}"))
	}

	config := Config{Exceptions: exceptions}
	if values["HTTPEnable"] == "1" {
		config.HTTP = proxyURL(values["HTTPProxy"], values["HTTPPort"])
	}
	if values["HTTPSEnable"] == "1" {
		config.HTTPS = proxyURL(values["HTTPSProxy"], values["HTTPSPort"])
	}
	// A SOCKS-only configuration is not expressible through http.Transport's
	// Proxy field, which takes an HTTP proxy URL. Leaving it empty is honest:
	// the request goes direct and fails visibly rather than silently pretending
	// to use a proxy that was never contacted.
	return config
}

// proxyURL builds the proxy URL from scutil's separate host and port entries.
//
// The host may carry credentials ("user:pass@10.0.0.5"), so it is split before
// JoinHostPort is applied: joining the whole thing brackets the user info as
// though it were an IPv6 address, which produces a URL no request can use.
func proxyURL(host, port string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if strings.Contains(host, "://") {
		return host
	}

	user := ""
	if at := strings.LastIndex(host, "@"); at >= 0 {
		user, host = host[:at], host[at+1:]
	}

	authority := host
	if port != "" {
		authority = net.JoinHostPort(host, strings.TrimSpace(port))
	}
	if user != "" {
		return "http://" + user + "@" + authority
	}
	return "http://" + authority
}

// Runner runs the system tool. It is an interface so the parser and the
// environment rules can be tested without the machine's real settings.
type Runner interface {
	Run(ctx context.Context, name string, args []string) (string, error)
}

// ScutilRunner runs /usr/sbin/scutil.
type ScutilRunner struct{}

// Run implements Runner.
func (ScutilRunner) Run(ctx context.Context, name string, args []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	output, err := runCommand(ctx, name, args)
	if err != nil {
		return "", err
	}
	return output, nil
}

// Resolver caches the system configuration.
//
// The lookup spawns a process, and every request of a sync would pay for it, so
// the answer is kept for a short while. A short window rather than forever
// because a user who fixes their proxy should not have to relaunch the app to
// have it noticed.
type Resolver struct {
	// Runner reads the system configuration; nil uses ScutilRunner.
	Runner Runner
	// TTL is how long an answer is reused. Zero uses DefaultTTL.
	TTL time.Duration
	// Now is the clock, for tests.
	Now func() time.Time

	mu     sync.Mutex
	config Config
	readAt time.Time
	loaded bool
}

// DefaultTTL is how long a system proxy answer is reused.
const DefaultTTL = 30 * time.Second

// Config returns the proxy for this request.
func (r *Resolver) Config(ctx context.Context) Config {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	ttl := r.TTL
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if r.loaded && now.Sub(r.readAt) < ttl {
		return r.config
	}

	runner := r.Runner
	if runner == nil {
		runner = ScutilRunner{}
	}
	output, err := runner.Run(ctx, ScutilBinary, []string{"--proxy"})
	config := Config{}
	if err == nil {
		config = ParseScutil(output)
	}

	r.config, r.readAt, r.loaded = config, now, true
	return config
}

// Invalidate forgets the cached answer, so the next request reads the system
// again. It is what the settings panel calls before a connection test: the
// answer the user is checking is often "I just turned the proxy on".
func (r *Resolver) Invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loaded = false
}

// DefaultResolver is the process-wide resolver.
var DefaultResolver = &Resolver{}

// ProxyFunc returns the Proxy hook for an http.Transport.
//
// The environment wins when it names a proxy, because a caller that exports one
// means it; otherwise the system configuration is used. NO_PROXY is honoured by
// http.ProxyFromEnvironment on the first path, and by Config.For on the second.
func ProxyFunc(resolver *Resolver) func(*http.Request) (*url.URL, error) {
	if resolver == nil {
		resolver = DefaultResolver
	}
	return func(request *http.Request) (*url.URL, error) {
		if envConfigured() {
			return http.ProxyFromEnvironment(request)
		}
		return resolver.Config(request.Context()).For(request.URL), nil
	}
}

// Transport returns the shared transport every client in the app uses.
//
// It is shared rather than built per client so the connection pool is reused:
// a sync fetches page after page from one host, and a fresh transport per client
// would open a new TLS connection every time.
var (
	transportOnce sync.Once
	transport     *http.Transport
)

// SharedTransport returns the process-wide transport, with the proxy hook set.
func SharedTransport() *http.Transport {
	transportOnce.Do(func() {
		transport = &http.Transport{
			Proxy: ProxyFunc(DefaultResolver),
			// The defaults http.DefaultTransport uses, restated because a
			// hand-built Transport has none of them.
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          16,
			MaxIdleConnsPerHost:   8,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
			ExpectContinueTimeout: time.Second,
		}
	})
	return transport
}

// NewClient builds a client with the given timeout over the shared transport.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: SharedTransport()}
}

// NewStrictClient is NewClient that refuses a redirect to another host.
//
// The API clients attach their credential as a header (or, for Google's
// generative endpoint, in the query string), and Go's default redirect policy
// only strips Authorization and Cookie. A provider — or a custom gateway the
// user typed — that answers 302 could therefore carry the key to a host the
// user never configured. Media fetches keep NewClient: their redirects are
// ordinary CDN hops and carry no secret.
func NewStrictClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: SharedTransport(),
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) == 0 {
				return nil
			}
			if request.URL.Host != via[0].URL.Host {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

// Describe renders the proxy in use, for a diagnostics line. It never includes
// credentials embedded in a proxy URL.
func (r *Resolver) Describe(ctx context.Context) string {
	if envConfigured() {
		return "environment"
	}
	config := r.Config(ctx)
	switch {
	case config.HTTPS != "":
		return redact(config.HTTPS)
	case config.HTTP != "":
		return redact(config.HTTP)
	default:
		return ""
	}
}

func redact(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	parsed.User = nil
	return parsed.String()
}
