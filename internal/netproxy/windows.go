package netproxy

import (
	"net/url"
	"strings"
)

// WindowsProxySettings is the shape of the current user's Internet Settings, with the
// types the registry holds. It is a plain struct so the parser below can be tested on
// any machine: only the reading of these values depends on Windows.
type WindowsProxySettings struct {
	// ProxyEnable is the master switch. A remembered proxy with the switch off is not a
	// live proxy, exactly like a disabled scheme in scutil's output.
	ProxyEnable bool
	// ProxyServer is either "host:port" for every scheme, or a semicolon-separated list
	// of "scheme=host:port" entries, which is what the Settings dialog writes when the
	// reader gives different addresses per protocol.
	ProxyServer string
	// ProxyOverride is the semicolon-separated bypass list: host names, `*.domain`,
	// CIDR blocks and `<local>`.
	ProxyOverride string
	// AutoConfigURL is a PAC script's address. This app does not evaluate PAC, and
	// guessing at one would be worse than going direct, so a profile that only has one
	// configures no proxy here.
	AutoConfigURL string
}

// ParseWindowsProxy turns the registry values into the Config the transports use.
//
// The bare "host:port" form applies to both schemes. In the per-scheme form only the
// http and https entries are read: the app speaks those, and a socks or ftp entry says
// nothing about them. A per-scheme list that names only http leaves HTTPS empty, which
// Config.For falls back to HTTP -- a proxy that is configured for one scheme is almost
// always the same local forwarder for both.
func ParseWindowsProxy(settings WindowsProxySettings) Config {
	if !settings.ProxyEnable {
		return Config{}
	}
	server := strings.TrimSpace(settings.ProxyServer)
	if server == "" {
		return Config{}
	}

	config := Config{}
	if strings.Contains(server, "=") {
		for _, entry := range strings.Split(server, ";") {
			name, value, ok := strings.Cut(entry, "=")
			if !ok {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(name)) {
			case "http":
				config.HTTP = windowsProxyURL(value)
			case "https":
				config.HTTPS = windowsProxyURL(value)
			}
		}
	} else {
		config.HTTP = windowsProxyURL(server)
	}
	if config.Empty() {
		return Config{}
	}

	for _, entry := range strings.Split(settings.ProxyOverride, ";") {
		if pattern := strings.TrimSpace(entry); pattern != "" {
			config.Exceptions = append(config.Exceptions, pattern)
		}
	}
	return config
}

// windowsProxyURL normalizes one proxy address to a URL.
//
// The registry stores a bare authority, but a reader who typed the address by hand may
// have included the scheme; both are accepted, and anything unparseable is dropped rather
// than handed to the transport.
func windowsProxyURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.String()
}
