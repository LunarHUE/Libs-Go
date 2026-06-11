package log_test

import (
	"sync"
	"testing"

	"github.com/lunarhue/libs-go/log"
)

// TestConcurrentLevelToggle drives the exact race Phase 3 removes: one goroutine flips the
// level while many goroutines call Debugf. On the pre-Phase-3 code this is an unsynchronized
// read/write of the Debugf function value (SetLevel -> updateLogFunctions reassigns it) and
// fails under -race. After Phase 3 the level is an atomic and Debugf is a fixed function, so
// it must be clean. Console is swapped to a buffer so the toggling doesn't spam stdout.
func TestConcurrentLevelToggle(t *testing.T) {
	restoreLevel := log.ForceLevel(log.INFO)
	defer restoreLevel()
	restoreConsole := log.SwapConsole(&nopWriter{}, false)
	defer restoreConsole()

	const (
		callers = 50
		iters   = 200
	)
	var wg sync.WaitGroup

	// Level flipper.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iters; i++ {
			if i%2 == 0 {
				log.SetLevel(log.DEBUG)
			} else {
				log.SetLevel(log.INFO)
			}
		}
	}()

	// Concurrent Debugf callers.
	wg.Add(callers)
	for c := 0; c < callers; c++ {
		go func(c int) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				log.Debugf("debug-from-%d-%d", c, i)
			}
		}(c)
	}

	wg.Wait()
}

// BenchmarkDebugfDisabled measures the disabled-debug path (level pinned below DEBUG so
// Debugf early-returns). Expect parity with the old no-op closure — this is a no-regression
// guard, not a speedup claim (see plan Context).
func BenchmarkDebugfDisabled(b *testing.B) {
	restoreLevel := log.ForceLevel(log.INFO) // below DEBUG -> Debugf returns early
	defer restoreLevel()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Debugf("disabled %d", i)
	}
}

// nopWriter discards console output for the concurrency test.
type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
