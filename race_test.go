//go:build race

package protocol

// raceEnabled is set when the tests run with the race detector, which
// allocates on its own: allocation counts are not checked then.
const raceEnabled = true
