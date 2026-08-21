package config

import (
	"embed"
	"fmt"

	"github.com/lunarhue/libs-go/log"
	"github.com/spf13/viper"
)

func LoadConfig[T any](
	embeddedFS *embed.FS,
	defaultConfigPath string,
	overrideFilePath string,
	envPrefix string,
	opts ...Option,
) (*T, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	v := viper.New()

	var config T

	// --- Default Config ---
	if defaultConfigPath != "" {
		err := loadConfigFile(v, defaultConfigPath, embeddedFS)
		if err != nil {
			return nil, fmt.Errorf("error reading default config file %s: %w", defaultConfigPath, err)
		}
		if err := v.Unmarshal(&config); err != nil {
			return nil, fmt.Errorf("unable to decode default config: %w", err)
		}
	} else {
		log.Debugf("No default config file provided, using embedded config")
	}

	// --- Override Config File ---
	if overrideFilePath != "" {
		err := loadConfigFile(v, overrideFilePath, nil)
		if err != nil {
			return nil, fmt.Errorf("error reading override config file %s: %w", overrideFilePath, err)
		}
		if err := v.Unmarshal(&config); err != nil {
			return nil, fmt.Errorf("unable to decode override config: %w", err)
		}
	} else {
		log.Debugf("No override config file provided")
	}

	// --- Etcd ---
	if !o.skipEtcd {
		err := loadConfigEtcd(v)
		if err != nil {
			log.Warnf("Error reading etcd: %s", err)
		}
		if err := v.Unmarshal(&config); err != nil {
			return nil, fmt.Errorf("unable to decode into struct, %v", err)
		}
	}

	// --- Environment Variables ---
	err := loadConfigEnv(v, envPrefix, overrideFilePath)
	if err != nil {
		return nil, fmt.Errorf("error reading environment variables: %w", err)
	}

	for _, binding := range o.envBindings {
		args := append([]string{binding.key}, binding.envVars...)
		if err := v.BindEnv(args...); err != nil {
			return nil, fmt.Errorf("error binding env for key %s: %w", binding.key, err)
		}
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
