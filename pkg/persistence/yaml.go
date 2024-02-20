package persistence

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/xiaomi388/beancount-automation/pkg/types"
	"gopkg.in/yaml.v3"
)

func DumpOwners(path string, owners []types.Owner) error {
	data, err := json.MarshalIndent(owners, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal txns: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func LoadOwners(path string) ([]types.Owner, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []types.Owner{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read txn db file: %w", err)
	}

	owners := []types.Owner{}
	if err := json.Unmarshal(data, &owners); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return owners, nil
}

func DumpConfig(path string, config types.Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}

	return nil
}

func LoadConfig(path string) (types.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return types.Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var config types.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to load config: %w", err)
	}

	return config, nil
}
