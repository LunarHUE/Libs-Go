package config

import (
	"embed"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

func loadConfigFile(filePath string, embedFile *embed.FS) error {
	ext := strings.TrimPrefix(filepath.Ext(filePath), ".")
	if ext == "" {
		return fmt.Errorf("cannot determine config type for default config: %s (missing extension)", filePath)
	}
	v.SetConfigType(ext)

	if embedFile != nil {
		file, err := embedFile.Open(filePath)
		if err != nil {
			return fmt.Errorf("error reading embedded default config %s: %w", filePath, err)
		}
		defer file.Close()

		configBytes, err := io.ReadAll(file)
		if err != nil {
			return fmt.Errorf("error reading embedded default config %s: %w", filePath, err)
		}

		err = v.ReadConfig(strings.NewReader(string(configBytes)))
		if err != nil {
			return fmt.Errorf("fatal error reading config: %w", err)
		}
	} else {
		v.SetConfigFile(filePath)

		err := v.MergeInConfig()
		if err != nil {
			return fmt.Errorf("fatal error reading config: %w", err)
		}
	}

	return nil
}
