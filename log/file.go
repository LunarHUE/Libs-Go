package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	logFile     *os.File
	logFileMu   sync.Mutex
	logFilePath string = ""

	currentFileLogs []string = []string{}
)

func logToFile(message string) {
	currentFileLogs = append(currentFileLogs, message)
	logFileMu.Lock()
	defer logFileMu.Unlock()

	if logFile == nil {
		return
	}
	if _, err := fmt.Fprintf(logFile, "%s\n", message); err != nil {
		fmt.Fprintf(os.Stderr, "%s ERROR: Failed to write internal message to log file %s: %v%s\n", colorRed, logFilePath, err, colorReset)
	}
}

func InitFileLogging(filePath string) error {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	if filePath == "" {
		logInternal(INFO, nil, "File logging disabled (no path provided)")
		logFilePath = ""
		if logFile != nil {
			logFile.Close()
			logFile = nil
		}
		return nil
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0750); err != nil { // Changed permissions slightly
		return fmt.Errorf("failed to create log directory '%s': %w", dir, err)
	}

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640) // Changed permissions slightly
	if err != nil {
		return fmt.Errorf("failed to open log file '%s': %w", filePath, err)
	}

	if logFile != nil {
		logFile.Close()
	}

	logFile = file
	logFilePath = filePath

	if len(currentFileLogs) > 0 {
		for _, logEntry := range currentFileLogs {
			logToFile(logEntry)
		}
	}

	return nil
}

func CloseFile() {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	if logFile != nil {
		logInternal(INFO, nil, "Closing log file: %s", logFilePath)
		logFile.Close()
		logFile = nil
		logFilePath = ""
	}
}
