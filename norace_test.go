//go:build !race

package protocol

// raceEnabled is set when the tests run with the race detector, see
// race_test.go.
const raceEnabled = false
