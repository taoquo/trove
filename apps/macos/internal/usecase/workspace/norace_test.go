//go:build !race

package workspace_test

// raceEnabled is false in a normal build, so the scaling report runs.
const raceEnabled = false
