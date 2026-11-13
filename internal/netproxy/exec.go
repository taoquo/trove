package netproxy

import (
	"context"
	"os/exec"
)

// runCommand is the one place this package starts a process.
//
// It is separate from Run so the exec call itself stays trivial to audit: the
// tool is fixed and the arguments are fixed, so nothing here can be steered by
// data.
func runCommand(ctx context.Context, name string, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
