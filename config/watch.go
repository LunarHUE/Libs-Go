package config

import (
	"fmt"

	"github.com/fsnotify/fsnotify"
)

func WatchFile(fileName string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	err = watcher.Add(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to watch file: %w", err)
	}

	return watcher, nil
}
