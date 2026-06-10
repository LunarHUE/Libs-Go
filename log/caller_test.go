package log_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/lunarhue/libs-go/log"
)

// These tests live in package log_test (external) on purpose: an in-package test
// function would be skipped by findCaller's own-package filter and so could never
// stand in for a genuine external caller. They drive the real public API and assert
// against the file sink, which always records file:line (FILE destination) regardless
// of log level — deterministic, no stdout capture.

// lineFor returns the first line in the log file containing marker.
func lineFor(t *testing.T, path, marker string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	for _, l := range strings.Split(string(data), "\n") {
		if strings.Contains(l, marker) {
			return l
		}
	}
	t.Fatalf("no log line containing %q in:\n%s", marker, data)
	return ""
}

func TestFindCallerReportsExternalCaller(t *testing.T) {
	// Initialize file logging once while the internal buffer is still empty (a
	// second InitFileLogging with buffered entries is a separate pre-existing
	// concern). We deliberately do not call CloseFile: writes are unbuffered, so
	// the file is readable immediately.
	path := filepath.Join(t.TempDir(), "test.log")
	if err := log.InitFileLogging(path); err != nil {
		t.Fatalf("InitFileLogging: %v", err)
	}

	t.Run("Errorf reports exact caller location", func(t *testing.T) {
		_, _, want, _ := runtime.Caller(0)
		log.Errorf("caller-probe-basic") // must stay one line below runtime.Caller

		line := lineFor(t, path, "caller-probe-basic")
		wantLoc := fmt.Sprintf("caller_test.go:%d", want+1)
		if !strings.Contains(line, wantLoc) {
			t.Errorf("caller location: got %q, want it to contain %q", line, wantLoc)
		}
		if strings.Contains(line, "log.go:") {
			t.Errorf("caller incorrectly reported inside the log package: %q", line)
		}
	})

	t.Run("Debugf reports exact caller location", func(t *testing.T) {
		log.SetLevel(log.DEBUG) // Debugf is a no-op below DEBUG
		_, _, want, _ := runtime.Caller(0)
		log.Debugf("caller-probe-debug") // must stay one line below runtime.Caller

		line := lineFor(t, path, "caller-probe-debug")
		wantLoc := fmt.Sprintf("caller_test.go:%d", want+1)
		if !strings.Contains(line, wantLoc) {
			t.Errorf("caller location: got %q, want it to contain %q", line, wantLoc)
		}
	})

	t.Run("reports caller from a goroutine", func(t *testing.T) {
		var wg sync.WaitGroup
		var want int
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, l, _ := runtime.Caller(0)
			log.Errorf("caller-probe-goroutine") // must stay one line below runtime.Caller
			want = l + 1
		}()
		wg.Wait()

		line := lineFor(t, path, "caller-probe-goroutine")
		wantLoc := fmt.Sprintf("caller_test.go:%d", want)
		if !strings.Contains(line, wantLoc) {
			t.Errorf("goroutine caller location: got %q, want it to contain %q", line, wantLoc)
		}
		if strings.Contains(line, "log.go:") {
			t.Errorf("goroutine caller incorrectly reported inside the log package: %q", line)
		}
	})
}
