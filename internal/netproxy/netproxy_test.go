package netproxy

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// scutilOutput is a real `scutil --proxy` from the machine this was written on,
// with the shape preserved: a dictionary, an array of exceptions, and one block
// per scheme.
const scutilOutput = `<dictionary> {
  ExceptionsList : <array> {
    0 : ::1
    1 : *.local
    2 : <local>
    3 : localhost
    4 : 127.0.0.1
    5 : 192.168.0.0/16
  }
  ExcludeSimpleHostnames : 0
  FTPPassive : 1
  HTTPEnable : 1
  HTTPPort : 7890
  HTTPProxy : 127.0.0.1
  HTTPSEnable : 1
  HTTPSPort : 7890
  HTTPSProxy : 127.0.0.1
  SOCKSEnable : 1
  SOCKSPort : 7890
  SOCKSProxy : 127.0.0.1
}`

func TestParseScutilReadsBothSchemes(t *testing.T) {
	config := ParseScutil(scutilOutput)

	if config.HTTP != "http://127.0.0.1:7890" {
		t.Errorf("HTTP = %q", config.HTTP)
	}
	if config.HTTPS != "http://127.0.0.1:7890" {
		t.Errorf("HTTPS = %q", config.HTTPS)
	}
	if len(config.Exceptions) != 6 {
		t.Fatalf("exceptions = %v, want six", config.Exceptions)
	}
	if config.Exceptions[2] != "<local>" {
		t.Errorf("the <local> shorthand was mangled: %q", config.Exceptions[2])
	}
}

// TestParseScutilIgnoresDisabledSchemes pins the rule that matters: a disabled
// entry is a remembered setting, not a live proxy. Honouring it would send
// traffic to a proxy the user turned off.
func TestParseScutilIgnoresDisabledSchemes(t *testing.T) {
	config := ParseScutil(`<dictionary> {
  HTTPEnable : 0
  HTTPPort : 8080
  HTTPProxy : 10.0.0.1
  HTTPSEnable : 1
  HTTPSPort : 8443
  HTTPSProxy : 10.0.0.2
}`)

	if config.HTTP != "" {
		t.Errorf("a disabled HTTP proxy was used: %q", config.HTTP)
	}
	if config.HTTPS != "http://10.0.0.2:8443" {
		t.Errorf("HTTPS = %q", config.HTTPS)
	}
}

// TestParseScutilKeepsProxyCredentials covers the shape that used to produce an
// unusable URL: an authenticated proxy, which macOS stores as one host string with
// the user info in front. Joining that whole string with a port brackets the user
// info as though it were an IPv6 address, so the split has to happen first. The
// values are invented; nothing here reads a real configuration.
func TestParseScutilKeepsProxyCredentials(t *testing.T) {
	const want = "http://proxyuser:proxy-pass-for-the-test@10.0.0.5:7890"
	for _, tc := range []struct {
		name   string
		output string
	}{
		{
			name: "an expanded dictionary",
			output: `<dictionary> {
  HTTPEnable : 0
  HTTPPort : 0
  HTTPSEnable : 1
  HTTPSPort : 7890
  HTTPSProxy : proxyuser:proxy-pass-for-the-test@10.0.0.5
}`,
		},
		{
			// scutil joins the last entry and the closing brace when it prints
			// compactly, so the brace must not become part of the port.
			name: "a compact rendition",
			output: `<dictionary> {
  HTTPEnable : 0
  HTTPPort : 0
  HTTPSEnable : 1
  HTTPSPort : 7890
  HTTPSProxy : proxyuser:proxy-pass-for-the-test@10.0.0.5}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := ParseScutil(tc.output)
			if config.HTTPS != want {
				t.Fatalf("HTTPS = %q, want %q", config.HTTPS, want)
			}
			parsed := config.For(mustURL(t, "https://x.com/"))
			if parsed == nil {
				t.Fatal("the proxy was not used")
			}
			if parsed.Host != "10.0.0.5:7890" {
				t.Errorf("host = %q", parsed.Host)
			}
			password, set := parsed.User.Password()
			if parsed.User.Username() != "proxyuser" || !set || password != "proxy-pass-for-the-test" {
				t.Errorf("user info = %v", parsed.User)
			}
		})
	}
}

// TestParseScutilWithoutAProxy is the machine with nothing configured: no
// proxy, and requests must go direct rather than to an empty host.
func TestParseScutilWithoutAProxy(t *testing.T) {
	config := ParseScutil(`<dictionary> {
  ExceptionsList : <array> {
    0 : localhost
  }
  HTTPEnable : 0
  HTTPSEnable : 0
}`)
	if !config.Empty() {
		t.Errorf("config = %+v, want empty", config)
	}
	if got := config.For(mustURL(t, "https://x.com/")); got != nil {
		t.Errorf("For returned %v on an empty configuration", got)
	}
}

func TestExceptionsBypassTheProxy(t *testing.T) {
	config := Config{
		HTTP:  "http://127.0.0.1:7890",
		HTTPS: "http://127.0.0.1:7890",
		Exceptions: []string{
			"localhost", "*.local", "<local>", "127.0.0.1", "192.168.0.0/16",
		},
	}

	for _, target := range []string{
		"http://localhost:8123/",
		"http://printer.local/",
		"http://intranet/",       // <local>: no dot
		"http://127.0.0.1:8799/", // exact host and CIDR
		"http://192.168.1.20/",
	} {
		if got := config.For(mustURL(t, target)); got != nil {
			t.Errorf("%s went through %v, want direct", target, got)
		}
	}

	for _, target := range []string{"https://x.com/", "https://api.openai.com/"} {
		got := config.For(mustURL(t, target))
		if got == nil || got.Host != "127.0.0.1:7890" {
			t.Errorf("%s did not use the proxy: %v", target, got)
		}
	}
}

// TestHTTPSPrefersItsOwnEntry keeps the two schemes apart: macOS lets a user set
// different proxies for each, and using the HTTP one for an HTTPS request would
// be wrong on exactly the machines that configured them differently.
func TestHTTPSPrefersItsOwnEntry(t *testing.T) {
	config := Config{HTTP: "http://10.0.0.1:3128", HTTPS: "http://10.0.0.2:3129"}
	got := config.For(mustURL(t, "https://x.com/"))
	if got == nil || got.Host != "10.0.0.2:3129" {
		t.Errorf("HTTPS request used %v", got)
	}
	got = config.For(mustURL(t, "http://example.com/"))
	if got == nil || got.Host != "10.0.0.1:3128" {
		t.Errorf("HTTP request used %v", got)
	}
}

func TestEnvConfigured(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("ALL_PROXY", "")
	if envConfigured() {
		t.Error("an empty environment reported a proxy")
	}
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:7890")
	if !envConfigured() {
		t.Error("HTTPS_PROXY was not noticed")
	}
}

// TestProxyFuncPrefersTheEnvironment keeps the documented precedence: a caller
// that exported a proxy means it, and it is also how a test points the
// transport somewhere without touching the machine.
func TestProxyFuncPrefersTheEnvironment(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:9999")
	t.Setenv("NO_PROXY", "")

	resolver := &Resolver{
		Runner: staticRunner(`<dictionary> { HTTPSEnable : 1
  HTTPSPort : 7890
  HTTPSProxy : 10.9.9.9 }`),
	}
	request, err := http.NewRequest(http.MethodGet, "https://x.com/", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ProxyFunc(resolver)(request)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Port() != "9999" {
		t.Errorf("the environment did not win: %v", got)
	}
}

// TestResolverCachesAndInvalidates covers the two behaviours the settings panel
// depends on: a sync must not spawn scutil per request, and a user who just
// turned a proxy on must not have to relaunch.
func TestResolverCachesAndInvalidates(t *testing.T) {
	runner := &countingRunner{output: `<dictionary> { HTTPSEnable : 1
  HTTPSPort : 7890
  HTTPSProxy : 10.0.0.5 }`}
	now := time.Unix(0, 0)
	resolver := &Resolver{Runner: runner, TTL: time.Minute, Now: func() time.Time { return now }}

	for i := 0; i < 5; i++ {
		resolver.Config(context.Background())
	}
	if runner.calls != 1 {
		t.Errorf("scutil ran %d times, want once", runner.calls)
	}

	now = now.Add(2 * time.Minute)
	resolver.Config(context.Background())
	if runner.calls != 2 {
		t.Errorf("the cached answer outlived its TTL: %d calls", runner.calls)
	}

	resolver.Invalidate()
	resolver.Config(context.Background())
	if runner.calls != 3 {
		t.Errorf("Invalidate did not force a re-read: %d calls", runner.calls)
	}
}

// TestResolverSurvivesAFailingTool keeps a machine where scutil is missing or
// restricted from breaking every request: no proxy is the honest answer.
func TestResolverSurvivesAFailingTool(t *testing.T) {
	resolver := &Resolver{Runner: failingRunner{}}
	if got := resolver.Config(context.Background()); !got.Empty() {
		t.Errorf("a failing tool produced %+v", got)
	}
}

func TestDescribeRedactsCredentials(t *testing.T) {
	resolver := &Resolver{
		Runner: staticRunner(`<dictionary> { HTTPSEnable : 1
  HTTPSPort : 7890
  HTTPSProxy : user:secret@10.0.0.5 }`),
	}
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("ALL_PROXY", "")

	got := resolver.Describe(context.Background())
	if strings.Contains(got, "secret") || strings.Contains(got, "user") {
		t.Errorf("Describe leaked the proxy credentials: %q", got)
	}
	if !strings.Contains(got, "10.0.0.5:7890") {
		t.Errorf("Describe = %q, want the host", got)
	}
}

func TestSharedTransportIsStable(t *testing.T) {
	if SharedTransport() != SharedTransport() {
		t.Error("the shared transport was rebuilt; the connection pool would be lost")
	}
	if SharedTransport().Proxy == nil {
		t.Error("the shared transport has no proxy hook")
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return parsed
}

type staticRunner string

func (r staticRunner) Run(context.Context, string, []string) (string, error) {
	return string(r), nil
}

type countingRunner struct {
	output string
	calls  int
}

func (r *countingRunner) Run(context.Context, string, []string) (string, error) {
	r.calls++
	return r.output, nil
}

type failingRunner struct{}

func (failingRunner) Run(context.Context, string, []string) (string, error) {
	return "", context.DeadlineExceeded
}
