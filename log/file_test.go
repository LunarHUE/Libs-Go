package log_test

import (
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

func TestInitFileLoggingEmptyPathDoesNotHang(t *testing.T) {
	// Covers the third re-entry site: the empty-path branch logged "File logging
	// disabled" via logInternal while holding logFileMu. Must simply return.
	// (Asserting the message reaches the console becomes possible once Phase 2's
	// consoleOut seam lands; for now we only require no deadlock.)
	if err := log.InitFileLogging(""); err != nil {
		t.Fatalf("InitFileLogging(\"\"): %v", err)
	}
}
