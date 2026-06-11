package log_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lunarhue/libs-go/log"
)

// These tests guard against the re-entrant logFileMu deadlock: CloseFile and
// InitFileLogging used to call back into the logger while holding the mutex, which
// re-locks the non-reentrant mutex and hangs the process. They run in the external
// log_test package to exercise the real public API. Run with `go test -timeout 10s`:
// on the buggy code the calls hang and the timeout fails the test.

const closingMarker = "Closing log file"

func countMarker(t *testing.T, path, marker string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	return strings.Count(string(data), marker)
}

func fileContains(t *testing.T, path, want string) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	return strings.Contains(string(data), want)
}

func TestCloseAndReinitDoNotDeadlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.log")
	if err := log.InitFileLogging(path); err != nil {
		t.Fatalf("InitFileLogging: %v", err)
	}
	// Leave file logging disabled for any later tests; InitFileLogging("") is also a
	// deadlock site, so this defer doubles as light coverage that it returns.
	// ORDERING NOTE: InitFileLogging("") now writes its own "Closing log file"
	// terminator into the live file. That is harmless here only because this defer runs
	// AFTER both marker-count assertions below — do not move a count assertion past this
	// point or into a later defer, or it will see the extra marker.
	defer log.InitFileLogging("")

	log.Errorf("before-close")

	// Must not hang: CloseFile previously called logInternal under logFileMu.
	log.CloseFile()

	// Assert BEFORE any re-init: the closing marker is in the file because CloseFile
	// wrote it directly (write-only), not because replay put it there — replay has not
	// run yet. Exactly one marker; CloseFile is the only producer in this binary.
	if got := countMarker(t, path, closingMarker); got != 1 {
		t.Fatalf("after CloseFile: got %d %q markers, want 1 (direct write)", got, closingMarker)
	}
	markersAfterClose := 1

	// File is closed; this must reach the buffer without hanging.
	log.Errorf("after-close")

	// Re-init replays the buffer into the re-opened file.
	if err := log.InitFileLogging(path); err != nil {
		t.Fatalf("re-init InitFileLogging: %v", err)
	}

	if !fileContains(t, path, "after-close") {
		t.Errorf("after-close was not replayed into the re-opened file")
	}

	// The regression guard: a closing marker that was wrongly buffered would now be
	// replayed into the new file, raising the count. The write-only path keeps it at 1.
	if got := countMarker(t, path, closingMarker); got != markersAfterClose {
		t.Errorf("closing marker count changed after replay: got %d, want %d (marker must not be buffered/replayed)", got, markersAfterClose)
	}
}

// hasLineWithSuffix reports whether any line ends with suffix. Used for the fixed-width
// ringline-NNNN markers, where an exact suffix match is unambiguous (ringline-0099 cannot
// match ringline-1099, etc.) — the formatted file line ends with the message.
func hasLineWithSuffix(lines []string, suffix string) bool {
	for _, l := range lines {
		if strings.HasSuffix(l, suffix) {
			return true
		}
	}
	return false
}

// TestPreInitBufferKeepsOnlyLastN proves the pre-init ring is bounded: lines logged before a
// file is opened are capped at PreInitBufferSize, and InitFileLogging replays exactly that
// many (the most recent), dropping the oldest. It first establishes a clean state
// (InitFileLogging+CloseFile drains and clears any buffer earlier tests left, then closes so
// the ring is empty and no file is open) so the result is independent of test ordering.
func TestPreInitBufferKeepsOnlyLastN(t *testing.T) {
	dir := t.TempDir()

	if err := log.InitFileLogging(filepath.Join(dir, "drain.log")); err != nil {
		t.Fatalf("drain InitFileLogging: %v", err)
	}
	log.CloseFile() // ring now empty, no file open

	// Log N+100 lines with no file open -> they buffer; only the last N survive the ring.
	total := log.PreInitBufferSize + 100
	for i := 0; i < total; i++ {
		log.Infof("ringline-%04d", i)
	}
	if got := log.PreInitBufferLen(); got != log.PreInitBufferSize {
		t.Fatalf("ring length = %d, want %d (must be capped, not grown)", got, log.PreInitBufferSize)
	}

	// Open the real file -> replay the last N, clear the ring.
	path := filepath.Join(dir, "ring.log")
	if err := log.InitFileLogging(path); err != nil {
		t.Fatalf("InitFileLogging: %v", err)
	}
	defer log.InitFileLogging("") // leave file logging disabled for later tests

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	lines := strings.Split(string(data), "\n")

	// Exactly N of our lines were replayed (not N+100).
	count := 0
	for _, l := range lines {
		if strings.Contains(l, "ringline-") {
			count++
		}
	}
	if count != log.PreInitBufferSize {
		t.Errorf("replayed %d ringline entries, want %d", count, log.PreInitBufferSize)
	}

	// The first 100 (ringline-0000 .. ringline-0099) were dropped: absent from the file.
	for i := 0; i < 100; i++ {
		marker := fmt.Sprintf("ringline-%04d", i)
		if hasLineWithSuffix(lines, marker) {
			t.Errorf("dropped line %q is present; the first 100 should be absent", marker)
		}
	}
	// The kept window is present: first survivor (i=100) and last logged (i=total-1).
	for _, i := range []int{100, total - 1} {
		marker := fmt.Sprintf("ringline-%04d", i)
		if !hasLineWithSuffix(lines, marker) {
			t.Errorf("kept line %q is absent; the last N should be present", marker)
		}
	}
}

// TestFileOpenDoesNotBuffer proves that while a file is open, logging writes directly and
// never touches the pre-init ring. The routing is deterministic, so a single post-condition
// after the lines is the whole claim.
func TestFileOpenDoesNotBuffer(t *testing.T) {
	dir := t.TempDir()
	if err := log.InitFileLogging(filepath.Join(dir, "open.log")); err != nil {
		t.Fatalf("InitFileLogging: %v", err)
	}
	defer log.InitFileLogging("") // leave file logging disabled for later tests

	for i := 0; i < 10000; i++ {
		log.Infof("open-%d", i)
	}
	if got := log.PreInitBufferLen(); got != 0 {
		t.Errorf("ring length = %d while a file is open, want 0 (direct write must not buffer)", got)
	}
}

func TestInitFileLoggingEmptyPathDoesNotHang(t *testing.T) {
	// Covers the third re-entry site: the empty-path branch logged "File logging
	// disabled" via logInternal while holding logFileMu. Must simply return.
	// (Asserting the message reaches the console becomes possible once Phase 2's
	// consoleOut seam lands; for now we only require no deadlock.)
	if err := log.InitFileLogging(""); err != nil {
		t.Fatalf("InitFileLogging(\"\"): %v", err)
	}
}
