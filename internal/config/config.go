package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Model      string `yaml:"model"`
	OllamaURL  string `yaml:"ollama_url"`
	Provider   string `yaml:"provider"`
	Approval   string `yaml:"approval"`
	GlobalPath string `yaml:"-"`
	ProjectPath string `yaml:"-"`
}

func DefaultConfig() *Config {
	return &Config{
		Model:     "qwen3-coder-64k-32k:latest",
		OllamaURL: "http://localhost:11434",
		Provider:  "ollama",
		Approval:  "ask",
	}
}

func LoadConfig() (*Config, error) {
	config := DefaultConfig()

	// Cargar config global
	globalDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("error getting home directory: %w", err)
	}
	globalPath := filepath.Join(globalDir, ".androideai", "config.yml")
	if _, err := os.Stat(globalPath); err == nil {
		if err := config.loadFromFile(globalPath); err != nil {
			return nil, fmt.Errorf("error loading global config: %w", err)
		}
	}

	// Cargar config de proyecto (sobreescribe global)
	projectPath := filepath.Join(".androideai", "config.yml")
	if _, err := os.Stat(projectPath); err == nil {
		if err := config.loadFromFile(projectPath); err != nil {
			return nil, fmt.Errorf("error loading project config: %w", err)
		}
	}

	return config, nil
}

func (c *Config) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		return err
	}

	return nil
}

func (c *Config) Save(projectPath string) error {
	if projectPath == "" {
		projectPath = filepath.Join(".androideai", "config.yml")
	}

	dir := filepath.Dir(projectPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("error marshaling config: %w", err)
	}

	if err := os.WriteFile(projectPath, data, 0644); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}
