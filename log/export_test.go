package log

import "io"

// SwapConsole redirects console output and forces the TTY flag for a test, resetting the
// dedup state so repeat-collapsing tests start clean. Returns a restore func that puts
// the previous sink/flag back and clears the dedup state again.
func SwapConsole(w io.Writer, isTTY bool) (restore func()) {
	consoleMu.Lock()
	prevOut, prevTTY := consoleOut, stdoutIsTTY
	consoleOut, stdoutIsTTY = w, isTTY
	latestLogLevel, latestLogMessage, latestCounter = 0, "", 0
	consoleMu.Unlock()
	return func() {
		consoleMu.Lock()
		consoleOut, stdoutIsTTY = prevOut, prevTTY
		latestLogLevel, latestLogMessage, latestCounter = 0, "", 0
		consoleMu.Unlock()
	}
}

// ForceLevel sets the console level WITHOUT emitting the "Log level set" line that the
// public SetLevel produces — so a test can pin the level (e.g. INFO, to keep dest ==
// STDOUT) without polluting a swapped console buffer. Returns a restore func.
func ForceLevel(l LogLevel) (restore func()) {
	prev := GetLevel()
	currentLevel.Store(int32(l))
	return func() { currentLevel.Store(int32(prev)) }
}
