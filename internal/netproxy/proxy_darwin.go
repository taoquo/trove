//go:build darwin

package netproxy

import "context"

// readSystemProxy asks SystemConfiguration through `scutil --proxy`.
func readSystemProxy(ctx context.Context, runner Runner) (Config, error) {
	output, err := runner.Run(ctx, ScutilBinary, []string{"--proxy"})
	if err != nil {
		return Config{}, err
	}
	return ParseScutil(output), nil
}
