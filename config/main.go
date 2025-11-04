package config

import (
	"embed"
	"fmt"

	"github.com/lunarhue/libs-go/log"
	"github.com/spf13/viper"
)

var (
	v = viper.New()
)

func LoadConfig[T any](
	embeddedFS *embed.FS,
	defaultConfigPath string,
	overrideFilePath string,
	envPrefix string,
) (*T, error) {
	var config T

	// --- Default Config ---
	if defaultConfigPath != "" {
		err := loadConfigFile(defaultConfigPath, embeddedFS)
		if err != nil {
			return nil, fmt.Errorf("error reading default config file %s: %w", defaultConfigPath, err)
		}
		if err := v.Unmarshal(&config); err != nil {
			return nil, fmt.Errorf("unable to decode default config: %w", err)
		}
	} else {
		log.Warnf("No default config file provided, using embedded config")
	}

	// --- Override Config File ---
	if overrideFilePath != "" {
		err := loadConfigFile(overrideFilePath, nil)
		if err != nil {
			return nil, fmt.Errorf("error reading override config file %s: %w", overrideFilePath, err)
		}
		if err := v.Unmarshal(&config); err != nil {
			return nil, fmt.Errorf("unable to decode override config: %w", err)
		}
	} else {
		log.Warnf("No override config file provided")
	}

	// --- Etcd ---
	err := loadConfigEtcd()
	if err != nil {
		log.Warnf("Error reading etcd: %s", err)
	}
	err = v.Unmarshal(&config)
	if err != nil {
		return nil, fmt.Errorf("unable to decode into struct, %v", err)
	}

	// --- Environment Variables ---
	err = loadConfigEnv(envPrefix, overrideFilePath)
	if err != nil {
		return nil, fmt.Errorf("error reading environment variables: %w", err)
	}

	if len(overrideFilePath) > 0 {
		err = v.MergeInConfig()
		if err != nil {
			return nil, fmt.Errorf("unable to merge config: %w", err)
		}
	}

	// Re-unmarshal to pick up env vars
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode with environment variables: %v", err)
	}

	return &config, nil
}
