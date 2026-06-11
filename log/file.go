package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const preInitBufferSize = 1000 // pre-init lines kept; older ones are dropped (see InitFileLogging)

var (
	logFile     *os.File
	logFileMu   sync.Mutex
	logFilePath string = ""

	// preInitBuf is a ring of the last preInitBufferSize lines logged BEFORE a file was
	// opened. Its only job is to bridge startup; once InitFileLogging opens a file the ring
	// is replayed and cleared, and never appended to again while a file is open. A fixed
	// array makes the bound structural — it is impossible to store more than N lines.
	preInitBuf   [preInitBufferSize]string
	preInitStart int // index of the oldest buffered line
	preInitLen   int // number of valid lines, capped at preInitBufferSize
)

// writeFileLineLocked writes one line to the open log file. Caller must hold logFileMu.
// Does NOT touch the pre-init ring — used by replay (lines already buffered) and by
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

// logToFileLocked routes one line. Caller must hold logFileMu. While a file is open it writes
// directly and never buffers; before any file is opened it goes to the bounded pre-init ring
// (most recent preInitBufferSize lines; older ones are dropped).
func logToFileLocked(line string) {
	if logFile != nil {
		writeFileLineLocked(line)
		return
	}
	bufferPreInitLocked(line)
}

// bufferPreInitLocked records one line in the bounded pre-init ring. Caller holds logFileMu.
// Only called while logFile == nil. When the ring is full the oldest line is overwritten and
// dropped — those lines will not reach the file (documented loss; see InitFileLogging). O(1)
// per call: a disabled/never-initialized logger makes this the steady-state hot path, so it
// must not copy the buffer down on each line.
func bufferPreInitLocked(line string) {
	if preInitLen < preInitBufferSize {
		preInitBuf[(preInitStart+preInitLen)%preInitBufferSize] = line
		preInitLen++
		return
	}
	preInitBuf[preInitStart] = line
	preInitStart = (preInitStart + 1) % preInitBufferSize
}

// logToFile is the normal entry point from logInternal.
func logToFile(message string) {
	logFileMu.Lock()
	defer logFileMu.Unlock()
	logToFileLocked(message)
}

// InitFileLogging opens filePath for logging, replaying the pre-init ring (lines logged
// before any file existed) into it and then clearing the ring. Only the most recent
// preInitBufferSize pre-init lines are replayed; earlier ones were dropped and do not appear
// in the file. Passing "" instead closes any open file and disables file logging.
//
// While disabled, the "File logging disabled" notice and any further lines flow back into the
// pre-init ring, so a later InitFileLogging(path) replays them into the new file as accurate
// context (this is also why a freshly opened file may begin with a prior session's tail).
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

	// Replay the pre-init ring oldest-first, writing each line straight to the file with
	// writeFileLineLocked. Replay's contract is "put these into THIS file, full stop", so it
	// names the write primitive directly rather than routing through logToFileLocked — replay
	// correctness must not hinge on logToFileLocked's logFile-based branch happening to take
	// the direct-write path right now. Then clear the ring: it exists only to bridge the gap
	// before a file was opened, so it must not survive to grow or replay again on a later
	// re-init. Pre-init lines beyond preInitBufferSize were already dropped and are absent
	// from the file by design.
	for i := 0; i < preInitLen; i++ {
		writeFileLineLocked(preInitBuf[(preInitStart+i)%preInitBufferSize])
	}
	preInitBuf = [preInitBufferSize]string{} // drop string refs so the buffered lines can be GC'd
	preInitStart, preInitLen = 0, 0

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
