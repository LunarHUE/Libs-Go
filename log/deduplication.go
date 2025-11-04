package log

import (
	"sync"
)

var (
	latestLogLevel   LogLevel
	latestLogMessage string
	latestCounter    int

	consoleDedupMu sync.Mutex
)

func dedupConsoleLog(
	dest Destination,
	level LogLevel,
	message string,
) bool {
	consoleDedupMu.Lock()
	defer consoleDedupMu.Unlock()

	if dest == FILE {
		return false
	}

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
