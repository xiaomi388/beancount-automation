package config

import (
	"fmt"
	"os"

	"github.com/plaid/plaid-go/plaid"
	"gopkg.in/yaml.v3"
)

var ConfigPath string

type Institution struct {
	Name        string         `yaml:"name"`
	AccessToken string         `yaml:"accessToken"`
	Cursor      string         `yaml:"cursor"`
	Type        plaid.Products `yaml:"type"`
}

type Owner struct {
	Name         string        `yaml:"name"`
	Institutions []Institution `yaml:"institutions"`
}

type Config struct {
	ClientID          string  `yaml:"clientID"`
	Secret            string  `yaml:"secret"`
	Environment       string  `yaml:"environment"`
	TransactionDBPath string  `yaml:"transactionDBPath"`
	HoldingDBPath     string  `yaml:"holdingDBPath"`
	DumpPath          string  `yaml:"dumpPath"`
	Owners            []Owner `yaml:"owners"`
}

func (c *Config) Owner(name string) (Owner, bool) {
	for _, owner := range c.Owners {
		if owner.Name == name {
			return owner, true
		}
	}

	return Owner{}, false
}

func (c *Config) SetOwner(owner Owner) error {
	if owner.Name == "" {
		return fmt.Errorf("owner name can not be empty")
	}

	for i := range c.Owners {
		if c.Owners[i].Name == owner.Name {
			c.Owners[i] = owner
			return nil
		}
	}

	c.Owners = append(c.Owners, owner)
	return nil
}

func (c *Config) Institution(name string, ownerName string) (Institution, bool) {
	owner, ok := c.Owner(ownerName)
	if !ok {
		return Institution{}, false
	}

	for _, institution := range owner.Institutions {
		if institution.Name == name {
			return institution, true
		}
	}

	return Institution{}, false
}

func (c *Config) SetInstitution(inst Institution, ownerName string) error {
	if inst.Name == "" {
		return fmt.Errorf("institution name must not be empty")
	}

	owner, ok := c.Owner(ownerName)
	if !ok {
		owner = Owner{
			Name: ownerName,
		}
	}

	for i := range owner.Institutions {
		if owner.Institutions[i].Name == inst.Name {
			owner.Institutions[i] = inst
			goto setowner
		}
	}
	owner.Institutions = append(owner.Institutions, inst)

setowner:
	if err := c.SetOwner(owner); err != nil {
		return fmt.Errorf("failed to add owner: %w", err)
	}

	return nil
}

func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &config, nil
}

func Dump(configPath string, config *Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}

	return nil
}
