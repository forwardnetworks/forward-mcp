//go:build race

package bloom

// raceEnabled reports whether the race detector is active, so tests can relax
// wall-clock performance assertions that are skewed by race instrumentation.
const raceEnabled = true
