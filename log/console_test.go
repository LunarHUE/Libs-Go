package log_test

import (
	"bytes"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/lunarhue/libs-go/log"
)

// withConsole pins the level to INFO (so dest == STDOUT, not STDOUT_DEBUG — caller_test
// leaves the package at DEBUG and tests share one binary), swaps the console sink for a
// buffer, and forces the TTY flag. Returns the buffer and a combined restore func.
func withConsole(t *testing.T, isTTY bool) (*bytes.Buffer, func()) {
	t.Helper()
	restoreLevel := log.ForceLevel(log.INFO)
	var buf bytes.Buffer
	restoreConsole := log.SwapConsole(&buf, isTTY)
	return &buf, func() {
		restoreConsole()
		restoreLevel()
	}
}

// TestNonTTYIsPlain: with stdout treated as a pipe, output carries no ANSI escapes and
// repeats are NOT collapsed — each occurrence is its own plain line (the documented
// non-TTY contract, consistent with the always-on file path).
func TestNonTTYIsPlain(t *testing.T) {
	buf, restore := withConsole(t, false)
	defer restore()

	for i := 0; i < 3; i++ {
		log.Infof("repeated-message")
	}

	out := buf.String()
	if strings.ContainsRune(out, '\033') {
		t.Errorf("non-TTY output contains an ANSI escape (\\033):\n%q", out)
	}
	if got := strings.Count(out, "repeated-message"); got != 3 {
		t.Errorf("non-TTY: got %d occurrences of the message, want 3 (no dedup off-TTY)", got)
	}
}

// lineFormat matches a non-color STDOUT line. Tokens verified against level.go (REQUEST
// renders as REQ) and formatLog's "2006-01-02 15:04:05" layout. If this is wrong it fails
// every line — do NOT loosen it to make the suite pass; that would void the interleaving
// guard below.
var lineFormat = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} (PANIC|ERROR|WARN|INFO|DEBUG|REQ): `)

// TestConcurrentConsoleNoInterleave: 100 goroutines logging a mix of repeated and distinct
// messages. With consoleMu held across the whole render, no line may be spliced or carry a
// stray escape — every non-empty line must match the line-format regex. Run with -race;
// this fails on the pre-Phase-2 split-mutex code.
func TestConcurrentConsoleNoInterleave(t *testing.T) {
	buf, restore := withConsole(t, false)
	defer restore()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			log.Infof("shared-repeated-line")
			log.Warnf("distinct-%d", g)
			log.Infof("shared-repeated-line")
		}(g)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != goroutines*3 {
		t.Errorf("got %d lines, want %d (no line lost or split)", len(lines), goroutines*3)
	}
	for i, line := range lines {
		if line == "" {
			continue
		}
		if !lineFormat.MatchString(line) {
			t.Fatalf("line %d does not match the log format (interleaving/corruption?):\n%q", i, line)
		}
	}
}

// TestTTYRewriteBranch exercises the branch real terminals run. The escapes are just bytes
// in a buffer, so this is fully deterministic: logging the same line twice must produce one
// cursor-up rewrite, exactly one (2x) marker, and the message text twice. The timestamp
// value is intentionally NOT asserted (seconds can tick between the two calls); that the
// rewrite carries the latest ts is guaranteed by construction (&ts is the call's own time).
func TestTTYRewriteBranch(t *testing.T) {
	buf, restore := withConsole(t, true)
	defer restore()

	log.Infof("rewrite-me")
	log.Infof("rewrite-me")

	out := buf.String()
	if !strings.Contains(out, "\033[F") {
		t.Errorf("TTY rewrite did not emit the cursor-up escape (\\033[F):\n%q", out)
	}
	if got := strings.Count(out, "(2x)"); got != 1 {
		t.Errorf("got %d (2x) markers, want exactly 1:\n%q", got, out)
	}
	if got := strings.Count(out, "rewrite-me"); got != 2 {
		t.Errorf("got %d occurrences of the message, want 2 (original + rewrite):\n%q", got, out)
	}
}
