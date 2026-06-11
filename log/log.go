package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
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

	// consoleMu guards the entire console-render sequence — dedup decision, the
	// cursor-rewrite-or-fresh-print, and the dedup-state update — as one atomic unit. It
	// replaces the old split between outputMutex and consoleDedupMu, which let a second
	// goroutine interleave its escape codes between another goroutine's dedup check and print.
	consoleMu sync.Mutex

	// consoleOut is the console sink; a var so tests can swap a bytes.Buffer (export_test.go).
	consoleOut io.Writer = os.Stdout

	// stdoutIsTTY gates ANSI color and the cursor-rewrite dedup. Computed once at init:
	// false when stdout isn't a terminal or NO_COLOR is set, so piped/CI/journald output
	// stays plain. A var (not const) so tests can force it.
	//
	// NO_COLOR semantics (no-color.org): disable only when the var is PRESENT and NON-EMPTY.
	// os.Getenv returns "" for both unset and empty, i.e. exactly the "keep color" cases,
	// so `== ""` is the correct test — do not "simplify" to LookupEnv-presence.
	stdoutIsTTY = term.IsTerminal(int(os.Stdout.Fd())) && os.Getenv("NO_COLOR") == ""

	// stderrIsTTY gates color on the stderr diagnostic path (file.go write failures).
	stderrIsTTY = term.IsTerminal(int(os.Stderr.Fd())) && os.Getenv("NO_COLOR") == ""
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

type Destination int

const (
	STDOUT Destination = iota
	STDOUT_DEBUG
	STDERR
	FILE
)

// formatLog renders one log line. useColor controls ANSI color on the STDOUT/STDOUT_DEBUG
// destinations only (FILE/STDERR are always colorless); making color an explicit parameter
// keeps the "color only when the TTY flag says so, read under consoleMu" invariant
// structural rather than a hidden read of a global.
func formatLog(
	dest Destination,
	level LogLevel,
	time *time.Time,
	message string,
	useColor bool,
) string {
	timestamp := time.Local().Format("2006-01-02 15:04:05")
	levelStr := levelNames[level]
	levelColor := levelColors[level]

	switch dest {
	case STDOUT:
		dg, lc, rst := "", "", ""
		if useColor {
			dg, lc, rst = colorDarkGrey, levelColor, colorReset
		}
		return fmt.Sprintf(
			"%s%s %s%s%s: %s",
			dg, timestamp,
			lc, levelStr, rst,
			message,
		)
	case STDOUT_DEBUG:
		file, line := findCaller()

		dg, lc, rst := "", "", ""
		if useColor {
			dg, lc, rst = colorDarkGrey, levelColor, colorReset
		}
		return fmt.Sprintf(
			"%s%s %s:%d %s%s%s: %s",
			dg, timestamp,
			file, line,
			lc, levelStr, rst,
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

// consoleWrite renders one line to the console. It holds consoleMu across the whole
// sequence (dedup decision -> rewrite/print -> dedup-state update) so concurrent callers
// cannot interleave escape sequences. ts is the call's timestamp (always the latest).
func consoleWrite(dest Destination, level LogLevel, msg string, ts time.Time) {
	consoleMu.Lock()
	defer consoleMu.Unlock()

	tty := stdoutIsTTY // read once under the lock; used for both branch and color

	// Non-TTY: no color, no cursor rewrite. Plain line per occurrence (no dedup),
	// matching the always-on file path.
	if !tty {
		fmt.Fprintln(consoleOut, formatLog(dest, level, &ts, msg, false))
		return
	}

	// TTY: collapse a run of identical lines by rewriting the previous line in place,
	// stamped with the latest ts and the running count.
	if isRepeatLocked(level, msg) { // caller holds consoleMu
		fmt.Fprintf(consoleOut, "\033[F\r\033[K%s %s(%dx)%s\n",
			formatLog(dest, level, &ts, msg, true), colorYellow, latestCounter, colorReset)
		return
	}
	fmt.Fprintln(consoleOut, formatLog(dest, level, &ts, msg, true))
}

// logInternal is the central function that handles formatting and output.
func logInternal(level LogLevel, panicFunc func(string, ...interface{}), format string, args ...interface{}) {
	currTime := time.Now().Local()
	message := fmt.Sprintf(format, args...)

	// --- File Logging (Always, No Deduplication) ---
	fileLog := formatLog(FILE, level, &currTime, message, false)
	logToFile(fileLog)

	// --- Console Logging (Level Filtered + Deduplication) ---
	consoleLevel := GetLevel() // single level read drives BOTH filtering and dest
	effectiveLevel := level
	if level == REQUEST {
		effectiveLevel = INFO
	}
	if effectiveLevel <= consoleLevel {
		dest := STDOUT
		if consoleLevel == DEBUG {
			dest = STDOUT_DEBUG
		}
		consoleWrite(dest, level, message, currTime)
	}

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
