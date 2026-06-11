package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logFile     *os.File
	logFileMu   sync.Mutex
	logFilePath string = ""

	currentFileLogs []string = []string{}
)

// writeFileLineLocked writes one line to the open log file. Caller must hold logFileMu.
// Does NOT touch currentFileLogs — used by replay (lines already buffered) and by
// CloseFile's terminator record (must die with the file, never replay).
func writeFileLineLocked(line string) {
	if logFile == nil {
		return
	}
	if _, err := fmt.Fprintf(logFile, "%s\n", line); err != nil {
		red, reset := "", ""
		if stderrIsTTY {
			red, reset = colorRed, colorReset
		}
		fmt.Fprintf(os.Stderr, "%s ERROR: Failed to write internal message to log file %s: %v%s\n", red, logFilePath, err, reset)
	}
}

// logToFileLocked buffers the line and writes it. Caller must hold logFileMu.
func logToFileLocked(line string) {
	currentFileLogs = append(currentFileLogs, line) // under the lock: protects currentFileLogs
	writeFileLineLocked(line)
}

// logToFile is the normal entry point from logInternal.
func logToFile(message string) {
	logFileMu.Lock()
	defer logFileMu.Unlock()
	logToFileLocked(message)
}

func InitFileLogging(filePath string) error {
	if filePath == "" {
		logFileMu.Lock()
		if logFile != nil {
			// Same "a file dies with its terminator record" rule as CloseFile: close-
			// and-disable must leave a marker, or the file ends mid-stream and is
			// indistinguishable from a crash. Write-only, so it can't replay elsewhere.
			now := time.Now()
			line := formatLog(FILE, INFO, &now, "Closing log file: "+logFilePath, false)
			writeFileLineLocked(line)
			logFile.Close()
			logFile = nil
		}
		logFilePath = ""
		logFileMu.Unlock()
		// Logged on the normal path AFTER releasing the mutex: there is no open file
		// for this message to land in, and logInternal re-enters logFileMu (which
		// would deadlock if called under the lock).
		logInternal(INFO, nil, "File logging disabled (no path provided)")
		return nil
	}

	logFileMu.Lock()
	defer logFileMu.Unlock()

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create log directory '%s': %w", dir, err)
	}

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return fmt.Errorf("failed to open log file '%s': %w", filePath, err)
	}

	if logFile != nil {
		logFile.Close()
	}

	logFile = file
	logFilePath = filePath

	// Replay buffered lines with writeFileLineLocked (write-only): they are already in
	// currentFileLogs, so logToFileLocked here would re-buffer and duplicate them.
	for _, logEntry := range currentFileLogs {
		writeFileLineLocked(logEntry)
	}

	return nil
}

func CloseFile() {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	if logFile != nil {
		now := time.Now()
		// formatLog(FILE, ...) runs findCaller, so the closing record reports the
		// CloseFile *caller's* file:line (who closed it), not file.go. Intentional —
		// don't "fix" this to point at file.go.
		line := formatLog(FILE, INFO, &now, "Closing log file: "+logFilePath, false)
		writeFileLineLocked(line) // write-only: must NOT buffer (would replay into next file)
		logFile.Close()
		logFile = nil
		logFilePath = ""
	}
}
