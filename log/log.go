package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"sync/atomic"
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
	// currentLevel is the minimum level; atomic so SetLevel and the per-call level checks
	// are race-free without a mutex. LogLevel is an int; stored/loaded as int32. Its zero
	// value is 0 == PANIC, not the intended INFO default, so init() stores INFO explicitly.
	currentLevel atomic.Int32

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

// The public log functions are declared (not reassignable vars) so the level switch can no
// longer race a concurrent call by rewriting a function value. Non-debug functions always
// call logInternal (which filters the console by level and always writes the file). The
// non-f variants use fmt.Sprint to match the previous closures byte-for-byte, including
// Sprint's quirk of inserting spaces only between non-string operands.
func Info(args ...any)                 { logInternal(INFO, nil, "%s", fmt.Sprint(args...)) }
func Infof(format string, a ...any)    { logInternal(INFO, nil, format, a...) }
func Warn(args ...any)                 { logInternal(WARN, nil, "%s", fmt.Sprint(args...)) }
func Warnf(format string, a ...any)    { logInternal(WARN, nil, format, a...) }
func Error(args ...any)                { logInternal(ERROR, nil, "%s", fmt.Sprint(args...)) }
func Errorf(format string, a ...any)   { logInternal(ERROR, nil, format, a...) }
func Request(args ...any)              { logInternal(REQUEST, nil, "%s", fmt.Sprint(args...)) }
func Requestf(format string, a ...any) { logInternal(REQUEST, nil, format, a...) }

func Panic(args ...any) {
	logInternal(PANIC, func(_ string, a ...any) { log.Panicln(a...) }, "%s", fmt.Sprint(args...))
}
func Panicf(format string, a ...any) { logInternal(PANIC, log.Panicf, format, a...) }

// Debug/Debugf return BEFORE logInternal when the level is below DEBUG. Besides skipping
// the formatting cost, this preserves the prior behavior that a disabled debug message
// never reaches the file: logInternal writes logToFile unconditionally, so the gate must
// live here, not inside logInternal. (The old code achieved this with no-op closures.)
func Debug(args ...any) {
	if GetLevel() < DEBUG {
		return
	}
	logInternal(DEBUG, nil, "%s", fmt.Sprint(args...))
}
func Debugf(format string, a ...any) {
	if GetLevel() < DEBUG {
		return
	}
	logInternal(DEBUG, nil, format, a...)
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
	currentLevel.Store(int32(INFO)) // explicit: the atomic's zero value would be PANIC, not INFO
}
