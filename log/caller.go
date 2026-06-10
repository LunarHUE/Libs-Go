package log

import (
	"path/filepath"
	"runtime"
	"strings"
)

// logPackagePath is this package's import path, discovered at startup instead of
// hardcoded. A previous hardcoded value ("github.com/lunarhue/go-stack/libs/log")
// silently broke caller detection after the module was renamed — discovery makes
// rename/fork/vendor safe.
var logPackagePath = discoverPackagePath()

func discoverPackagePath() string {
	pc, _, _, ok := runtime.Caller(0) // this frame: <pkgpath>.discoverPackagePath
	if !ok {
		// Should be impossible; failing loudly beats silently degrading to the
		// original bug (a "" path makes the prefix "." match nothing).
		panic("log: runtime.Caller failed during package-path discovery")
	}
	name := runtime.FuncForPC(pc).Name() // e.g. github.com/lunarhue/libs-go/log.discoverPackagePath
	// package path = everything before the first '.' that follows the last '/'
	if slash := strings.LastIndex(name, "/"); slash >= 0 {
		if dot := strings.Index(name[slash:], "."); dot >= 0 {
			return name[:slash+dot]
		}
	} else if dot := strings.Index(name, "."); dot >= 0 {
		return name[:dot]
	}
	panic("log: could not parse package path from func name: " + name)
}

func findCaller() (string, int) {
	const maxStackDepth = 32
	var pcs [maxStackDepth]uintptr
	// skip=2 clears runtime.Callers + findCaller; the prefix check below skips the
	// remaining in-package frames (formatLog, logInternal, the public closures), so
	// this offset is self-correcting and won't break if the internal chain is refactored.
	// Note: runtime.Callers skip is one greater than runtime.Caller skip.
	n := runtime.Callers(2, pcs[:])
	if n == 0 {
		return "???", 0
	}
	prefix := logPackagePath + "." // trailing dot so a sibling like logutil/log_test isn't treated as internal
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		// Skip junk frames (empty Function = unresolvable PC) and any in-package
		// frame; the first frame outside this package is the real caller.
		if frame.Function != "" && !strings.HasPrefix(frame.Function, prefix) {
			return filepath.Base(frame.File), frame.Line
		}
		if !more {
			break
		}
	}
	return "???", 0
}
