//go:build windows

package netproxy

import (
	"context"

	"golang.org/x/sys/windows/registry"
)

// internetSettingsKey is where the current user's proxy lives.
const internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

// readSystemProxy reads the current user's Internet Settings.
//
// A missing key or a missing value is Windows's own default -- no proxy -- so it is not
// an error. The context is accepted for symmetry with the macOS reader; there is no
// process to stop.
func readSystemProxy(ctx context.Context, _ Runner) (Config, error) {
	_ = ctx
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return Config{}, nil
	}
	defer key.Close()

	settings := WindowsProxySettings{}
	if value, _, err := key.GetIntegerValue("ProxyEnable"); err == nil {
		settings.ProxyEnable = value != 0
	}
	settings.ProxyServer, _, _ = key.GetStringValue("ProxyServer")
	settings.ProxyOverride, _, _ = key.GetStringValue("ProxyOverride")
	settings.AutoConfigURL, _, _ = key.GetStringValue("AutoConfigURL")
	return ParseWindowsProxy(settings), nil
}
