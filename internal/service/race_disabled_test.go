//go:build !race

package service

// raceEnabled reports whether the race detector is active, so tests can relax
// wall-clock performance assertions that are skewed by race instrumentation.
const raceEnabled = false
