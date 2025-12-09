package log

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// LogCommand attaches the logger to the command's stdout/stderr.
// It returns a 'wait' function that you must call after cmd.Wait() returns.
// valid loggers: log.Infof, log.Warnf, etc.
func LogCommand(cmd *exec.Cmd, name string) (func(), error) {
	// Create pipes
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to pipe stdout: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to pipe stderr: %w", err)
	}

	// WaitGroup to ensure we print every last line before exiting
	var wg sync.WaitGroup
	wg.Add(2)

	// Helper to scan a pipe and log it
	scan := func(pipe io.Reader, logFunc func(string, ...interface{}), level string) {
		defer wg.Done()
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			// Call your existing global logger here
			// Assuming your logger is accessible as 'Log' or passed in.
			// For this example, I'll assume you pass the log function.
			logFunc("[%s] %s", name, scanner.Text())
		}
	}

	// Start scanning in background
	// You can swap 'Infof' / 'Warnf' with whatever your actual logger uses
	go scan(stdoutPipe, Infof, "INFO")
	go scan(stderrPipe, Warnf, "WARN")

	// Return a closure that waits for the logs to finish
	waitForLogs := func() {
		wg.Wait()
	}

	return waitForLogs, nil
}
