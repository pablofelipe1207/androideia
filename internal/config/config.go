package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Model       string `yaml:"model"`
	OllamaURL   string `yaml:"ollama_url"`
	OllamaModel string `yaml:"ollama_model"` // modelo usado para clasificación+embeddings en Ollama
	Provider    string `yaml:"provider"`
	Approval    string `yaml:"approval"`
	Timeout     int    `yaml:"timeout"` // seconds; 0 means use default
	GlobalPath  string `yaml:"-"`
	ProjectPath string `yaml:"-"`
}

func DefaultConfig() *Config {
	return &Config{
		Model:     "qwen3-coder-64k-32k:latest",
		OllamaURL: "http://localhost:11434",
		Provider:  "ollama",
		Approval:  "ask",
		Timeout:   300, // 5 minutes per LLM call
	}
}

// EffectiveOllamaModel devuelve el modelo que se debe usar para las
// operaciones de Ollama (clasificación semántica de archivos,
// generación de embeddings).
//
// Si `OllamaModel` está configurado, se usa ese. Si no, cae al
// `Model` general (preservando el comportamiento histórico donde el
// mismo modelo servía para chat y para clasificación).
//
// Esto es relevante cuando se usa `provider: opencode_zen` (o
// cualquier provider no-Ollama): el agente habla con Zen, pero las
// operaciones de Ollama siguen siendo locales, y necesitan un modelo
// que sí esté instalado en Ollama (p. ej. `qwen2.5-coder:7b` para
// clasificación, `nomic-embed-text` para embeddings).
func (c *Config) EffectiveOllamaModel() string {
	if c.OllamaModel != "" {
		return c.OllamaModel
	}
	return c.Model
}

// EffectiveTimeout returns the configured timeout or the default if not set.
func (c *Config) EffectiveTimeout() int {
	if c.Timeout <= 0 {
		return 300
	}
	return c.Timeout
}

func LoadConfig() (*Config, error) {
	config := DefaultConfig()

	// Cargar config global
	globalDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("error getting home directory: %w", err)
	}
	globalPath := filepath.Join(globalDir, ".androideai", "config.yml")
	config.GlobalPath = globalPath
	if _, err := os.Stat(globalPath); err == nil {
		if err := config.loadFromFile(globalPath); err != nil {
			return nil, fmt.Errorf("error loading global config: %w", err)
		}
	}

	// Cargar config de proyecto (sobreescribe global)
	projectPath := filepath.Join(".androideai", "config.yml")
	config.ProjectPath = projectPath
	if _, err := os.Stat(projectPath); err == nil {
		if err := config.loadFromFile(projectPath); err != nil {
			return nil, fmt.Errorf("error loading project config: %w", err)
		}
	}

	return config, nil
}

// LoadConfigFromFile carga defaults + un único archivo de configuración.
// Si el archivo no existe, retorna los defaults. Útil para editar la
// configuración global o de proyecto de forma aislada.
func LoadConfigFromFile(path string) (*Config, error) {
	cfg := DefaultConfig()
	if _, err := os.Stat(path); err == nil {
		if err := cfg.loadFromFile(path); err != nil {
			return nil, fmt.Errorf("error loading config from %s: %w", path, err)
		}
	}
	return cfg, nil
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
