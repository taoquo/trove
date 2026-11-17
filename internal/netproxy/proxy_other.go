//go:build !darwin && !windows

package netproxy

import "context"

// readSystemProxy is the no-op on the platforms whose proxy setting this app does not
// read: there the environment is the only source, which ProxyFunc already honours.
func readSystemProxy(context.Context, Runner) (Config, error) {
	return Config{}, nil
}
