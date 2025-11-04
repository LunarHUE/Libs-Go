package config

import (
	"fmt"
	"strings"

	"github.com/lunarhue/libs-go/log"
)

func loadConfigEnv(prefix string, overrideFilePath string) error {
	replacer := strings.NewReplacer(
		"-", "_",
		".", "_",
	)
	v.SetEnvKeyReplacer(replacer)
	v.SetEnvPrefix(prefix)

	v.AutomaticEnv()

	log.Debugf("Loaded configuration from environment variables with prefix \"%s\"", prefix)

	if len(overrideFilePath) == 0 {
		return nil
	}

	err := v.MergeInConfig()
	if err != nil {
		return fmt.Errorf("unable to merge config: %w", err)
	}

	return nil
}
