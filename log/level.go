package log

import (
	"fmt"
	"strings"
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

// SetLevel sets the minimum log level. The equal-level short-circuit is a best-effort dedup,
// NOT atomic: two concurrent SetLevel calls can both pass the check and both store/log. That
// is harmless — last-write-wins on the value, at worst one duplicate "Log level set" line —
// and is intentionally left as-is. Do NOT "fix" it with a mutex or CAS loop: that would
// reintroduce the locking this design removed, just to suppress a benign duplicate line.
func SetLevel(level LogLevel) {
	if level == GetLevel() {
		return
	}
	currentLevel.Store(int32(level))
	logInternal(INFO, nil, "Log level set to %s", levelNames[level])
}

// GetLevel returns the current minimum log level.
func GetLevel() LogLevel {
	return LogLevel(currentLevel.Load())
}
