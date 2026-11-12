//go:build race

package workspace_test

// raceEnabled reports whether the binary was built with the race detector.
//
// The scaling report builds databases of fifty thousand rows, which under -race
// takes minutes. It is a measurement harness, not a correctness test, so it skips
// there: otherwise the per-commit gate the project requires would be unusable.
const raceEnabled = true
