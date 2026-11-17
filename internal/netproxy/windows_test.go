package netproxy

import (
	"net/url"
	"testing"
)

// TestParseWindowsProxy covers the shapes the registry holds. The samples are the fixed
// values a machine writes; nothing here reads a real registry, so the table runs on
// every platform.
func TestParseWindowsProxy(t *testing.T) {
	for _, tc := range []struct {
		name     string
		settings WindowsProxySettings
		http     string
		https    string
	}{
		{
			name:     "the switch is off",
			settings: WindowsProxySettings{ProxyEnable: false, ProxyServer: "127.0.0.1:7890"},
		},
		{
			name:     "a bare authority serves both schemes",
			settings: WindowsProxySettings{ProxyEnable: true, ProxyServer: "127.0.0.1:7890"},
			http:     "http://127.0.0.1:7890",
		},
		{
			name: "one address per scheme",
			settings: WindowsProxySettings{ProxyEnable: true,
				ProxyServer: "http=10.0.0.2:8080;https=10.0.0.2:8443"},
			http:  "http://10.0.0.2:8080",
			https: "http://10.0.0.2:8443",
		},
		{
			name: "only http named leaves https to the fallback",
			settings: WindowsProxySettings{ProxyEnable: true,
				ProxyServer: "http=127.0.0.1:7890;socks=127.0.0.1:7891"},
			http: "http://127.0.0.1:7890",
		},
		{
			name: "schemes this app does not speak are ignored",
			settings: WindowsProxySettings{ProxyEnable: true,
				ProxyServer: "ftp=10.0.0.2:21;socks=10.0.0.2:1080"},
		},
		{
			name:     "an address written with its scheme is accepted",
			settings: WindowsProxySettings{ProxyEnable: true, ProxyServer: "https=http://127.0.0.1:7890"},
			https:    "http://127.0.0.1:7890",
		},
		{
			name:     "an address with no host is dropped",
			settings: WindowsProxySettings{ProxyEnable: true, ProxyServer: "http://"},
		},
		{
			name:     "a PAC script alone configures nothing",
			settings: WindowsProxySettings{ProxyEnable: true, AutoConfigURL: "http://example.test/proxy.pac"},
		},
		{
			name:     "surrounding spaces are trimmed",
			settings: WindowsProxySettings{ProxyEnable: true, ProxyServer: "  127.0.0.1:7890  "},
			http:     "http://127.0.0.1:7890",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := ParseWindowsProxy(tc.settings)
			if config.HTTP != tc.http || config.HTTPS != tc.https {
				t.Errorf("config = %+v, want http %q and https %q", config, tc.http, tc.https)
			}
		})
	}
}

// TestWindowsOverridesBypassTheProxy pins the bypass list: the settings dialog's hosts,
// wildcards, `<local>` and CIDR blocks mean the same thing there as they do in scutil's
// exception list, because both are read by the same Config.
func TestWindowsOverridesBypassTheProxy(t *testing.T) {
	config := ParseWindowsProxy(WindowsProxySettings{
		ProxyEnable:   true,
		ProxyServer:   "127.0.0.1:7890",
		ProxyOverride: "localhost;*.corp.example;10.0.0.0/8;intranet;<local>",
		AutoConfigURL: "",
	})
	if config.HTTP == "" {
		t.Fatal("the proxy was not read")
	}

	for _, tc := range []struct {
		host   string
		bypass bool
	}{
		{"localhost", true},
		{"api.corp.example", true},
		{"corp.example", false},
		{"intranet", true},
		{"printer", true}, // <local> means a name with no dots
		{"10.2.3.4", true},
		{"11.2.3.4", false},
		{"x.com", false},
	} {
		target := &url.URL{Scheme: "https", Host: tc.host}
		if bypassed := config.For(target) == nil; bypassed != tc.bypass {
			t.Errorf("%s: bypass = %v, want %v", tc.host, bypassed, tc.bypass)
		}
	}
}
