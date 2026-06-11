package log

var (
	latestLogLevel   LogLevel
	latestLogMessage string
	latestCounter    int
)

// isRepeatLocked reports whether (level, message) repeats the last console line, updating
// the dedup state. Caller MUST hold consoleMu — the latest* vars live under that lock now
// (previously a separate consoleDedupMu, which let the rewrite race the print). On a match
// it bumps latestCounter and returns true; otherwise it records the new line, resets the
// counter to 1, and returns false. DEBUG lines never collapse.
func isRepeatLocked(level LogLevel, message string) bool {
	if level != DEBUG &&
		level == latestLogLevel &&
		message == latestLogMessage {

		latestCounter++
		return true
	}

	latestLogLevel = level
	latestLogMessage = message
	latestCounter = 1

	return false
}
