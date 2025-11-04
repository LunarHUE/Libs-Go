package config

import (
	"fmt"
	"os"

	"github.com/lunarhue/libs-go/log"
	"gopkg.in/yaml.v3"
)

func SaveConfig(config *any, fileName string) error {
	yamlBytes, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to serialize config to YAML: %w", err)
	}

	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to open file for writing: %w", err)
	}

	_, err = file.Write(yamlBytes)
	if err != nil {
		return fmt.Errorf("failed to write config to file: %w", err)
	}

	log.Infof("Config saved to %s", fileName)

	return nil
}
