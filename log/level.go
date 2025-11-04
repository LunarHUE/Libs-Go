package log

import (
	"fmt"
	"strings"
	"sync"
)

type LogLevel int

const (
	PANIC LogLevel = iota
	ERROR
	WARN
	INFO
	DEBUG
	REQUEST
)

var levelNames = map[LogLevel]string{
	PANIC:   "PANIC",
	ERROR:   "ERROR",
	WARN:    "WARN",
	INFO:    "INFO",
	DEBUG:   "DEBUG",
	REQUEST: "REQ",
}

var levelColors = map[LogLevel]string{
	PANIC:   colorRed,
	ERROR:   colorRed,
	WARN:    colorYellow,
	INFO:    colorBlue,
	DEBUG:   colorYellow, // Keep debug yellow? Or choose another?
	REQUEST: colorDarkGrey,
}

var levelMutex sync.RWMutex

// SetLevelFromString sets the minimum log level based on a string identifier.
// Valid levels: "debug", "info", "warn", "error", "panic". Case-insensitive.
func SetLevelFromString(levelStr string) error {
	levelStr = strings.ToLower(levelStr)
	var level LogLevel
	switch levelStr {
	case "debug":
		level = DEBUG
	case "info":
		level = INFO
	case "warn":
		level = WARN
	case "error":
		level = ERROR
	case "panic":
		level = PANIC
	default:
		return fmt.Errorf("invalid log level: %s", levelStr)
	}
	SetLevel(level)
	return nil
}

func SetLevel(level LogLevel) {
	if level == currentLevel {
		return
	}

	levelMutex.Lock()
	currentLevel = level
	logInternal(INFO, nil, "Log level set to %s", levelNames[level])
	levelMutex.Unlock()
	updateLogFunctions()
}

// GetLevel returns the current minimum log level.
func GetLevel() LogLevel {
	// levelMutex.RLock()
	// defer levelMutex.RUnlock()
	return currentLevel
}
