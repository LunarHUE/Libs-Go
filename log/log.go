package log

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	colorReset    = "\033[0m"
	colorRed      = "\033[31m"
	colorYellow   = "\033[33m"
	colorBlue     = "\033[34m"
	colorDarkGrey = "\033[90m"
)

var (
	currentLevel LogLevel = INFO
	outputMutex  sync.Mutex
)

var Info func(...any)
var Infof func(string, ...any)
var Request func(...any)
var Requestf func(string, ...any)
var Warn func(...any)
var Warnf func(string, ...any)
var Error func(...any)
var Errorf func(string, ...any)
var Debug func(...any)
var Debugf func(string, ...any)
var Panic func(...any)
var Panicf func(string, ...any)

// updateLogFunctions re-assigns the public log functions based on the current level.
// This is primarily to make Debug/Debugf no-ops if the level is higher.
func updateLogFunctions() {
	levelMutex.RLock()
	lvl := currentLevel
	levelMutex.RUnlock()

	paniclnWrapper := func(format string, args ...any) {
		log.Panicln(args...)
	}

	// --- Corrected Calls for non-f functions ---
	Info = func(args ...any) { logInternal(INFO, nil, "%s", fmt.Sprint(args...)) }
	Warn = func(args ...any) { logInternal(WARN, nil, "%s", fmt.Sprint(args...)) }
	Error = func(args ...any) { logInternal(ERROR, nil, "%s", fmt.Sprint(args...)) }
	Panic = func(args ...any) { logInternal(PANIC, paniclnWrapper, "%s", fmt.Sprint(args...)) }
	Request = func(args ...any) { logInternal(REQUEST, nil, "%s", fmt.Sprint(args...)) }

	// --- Calls for -f functions remain the same ---
	Infof = func(format string, args ...any) { logInternal(INFO, nil, format, args...) }
	Warnf = func(format string, args ...any) { logInternal(WARN, nil, format, args...) }
	Errorf = func(format string, args ...any) { logInternal(ERROR, nil, format, args...) }
	Panicf = func(format string, args ...any) { logInternal(PANIC, log.Panicf, format, args...) }
	Requestf = func(format string, args ...any) { logInternal(REQUEST, nil, format, args...) }

	if lvl >= DEBUG {
		Debug = func(args ...any) { logInternal(DEBUG, nil, "%s", fmt.Sprint(args...)) }
		Debugf = func(format string, args ...any) { logInternal(DEBUG, nil, format, args...) }
	} else {
		Debug = func(...any) {}
		Debugf = func(string, ...any) {}
	}
}

func findCaller() (string, int) {
	var file string
	var line int
	var ok bool
	const maxStackDepth = 10
	loggerPackagePath := "github.com/lunarhue/go-stack/libs/log"

	for skip := 3; skip < maxStackDepth; skip++ {
		var pc uintptr
		pc, file, line, ok = runtime.Caller(skip)
		if !ok {
			return "???", 0
		}

		fn := runtime.FuncForPC(pc)
		if fn != nil {
			funcName := fn.Name()
			if !strings.HasPrefix(funcName, loggerPackagePath) {
				return filepath.Base(file), line
			}
		} else {
			if !strings.Contains(file, loggerPackagePath) {
				return filepath.Base(file), line
			}
		}
	}
	return "???", 0
}

type Destination int

const (
	STDOUT Destination = iota
	STDOUT_DEBUG
	STDERR
	FILE
)

func formatLog(
	dest Destination,
	level LogLevel,
	time *time.Time,
	message string,
) string {
	timestamp := time.Local().Format("2006-01-02 15:04:05")
	levelStr := levelNames[level]
	levelColor := levelColors[level]

	switch dest {
	case STDOUT:
		return fmt.Sprintf(
			"%s%s %s%s%s: %s",
			colorDarkGrey, timestamp,
			levelColor, levelStr, colorReset,
			message,
		)
	case STDOUT_DEBUG:
		file, line := findCaller()

		return fmt.Sprintf(
			"%s%s %s:%d %s%s%s: %s",
			colorDarkGrey, timestamp,
			file, line,
			levelColor, levelStr, colorReset,
			message,
		)
	case FILE:
		timestamp := time.UTC().Format("2006-01-02 15:04:05")
		file, line := findCaller()

		return fmt.Sprintf(
			"%s %s:%d %s: %s",
			timestamp,
			file, line,
			levelStr,
			message,
		)
	case STDERR:
		file, line := findCaller()

		return fmt.Sprintf(
			"%s %s:%d %s: %s",
			timestamp,
			file, line,
			levelStr,
			message,
		)
	default:
		panic(fmt.Sprintf("Unknown destination: %d", dest))
	}
}

// logInternal is the central function that handles formatting and output.
func logInternal(level LogLevel, panicFunc func(string, ...interface{}), format string, args ...interface{}) {
	currTime := time.Now().Local()
	message := fmt.Sprintf(format, args...)

	// --- File Logging (Always, No Deduplication) ---
	fileLog := formatLog(FILE, level, &currTime, message)
	logToFile(fileLog)

	// --- Console Logging (Level Filtered + Deduplication) ---
	consoleLevel := GetLevel()
	effectiveLevel := level
	if level == REQUEST {
		effectiveLevel = INFO
	}

	// Filter out levels higher than the current level
	if effectiveLevel > consoleLevel {
		goto HandlePanic
	}

	// --- Deduplication Logic ---
	{
		var dest Destination
		if currentLevel == DEBUG {
			dest = STDOUT_DEBUG
		} else {
			dest = STDOUT
		}

		if dedupConsoleLog(dest, level, message) {
			fmt.Print("\033[F") // Move cursor up one line
			fmt.Print("\r")     // Move to the beginning of that line
			fmt.Print("\033[K") // Clear that line
			fmt.Printf("%s %s(%dx)%s\n", formatLog(dest, level, &currTime, message), colorYellow, latestCounter, colorReset)

			goto HandlePanic
		}

		// --- Output the Console Log Lines ---
		outputMutex.Lock()
		fmt.Println(formatLog(dest, level, &currTime, message))
		defer outputMutex.Unlock()
	}

HandlePanic:
	// --- Handle Panic (After Logging) ---
	if level == PANIC && panicFunc != nil {
		panicFunc(format, args...)
	} else if level == PANIC {
		panic(message)
	}
}

func init() {
	updateLogFunctions()
}
