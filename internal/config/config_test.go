package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Model != "qwen3-coder-64k-32k:latest" {
		t.Errorf("Expected model 'qwen3-coder-64k-32k:latest', got '%s'", cfg.Model)
	}
	if cfg.OllamaURL != "http://localhost:11434" {
		t.Errorf("Expected ollama_url 'http://localhost:11434', got '%s'", cfg.OllamaURL)
	}
	if cfg.Provider != "ollama" {
		t.Errorf("Expected provider 'ollama', got '%s'", cfg.Provider)
	}
	if cfg.Approval != "ask" {
		t.Errorf("Expected approval 'ask', got '%s'", cfg.Approval)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Crear directorio temporal
	tmpDir, err := os.MkdirTemp("", "config-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Crear config
	cfg := DefaultConfig()
	cfg.Model = "test-model"
	cfg.Approval = "auto"

	// Guardar config
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Error saving config: %v", err)
	}

	// Verificar que el archivo existe
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Cargar config
	loadedCfg := DefaultConfig()
	if err := loadedCfg.loadFromFile(configPath); err != nil {
		t.Fatalf("Error loading config: %v", err)
	}

	// Verificar valores
	if loadedCfg.Model != "test-model" {
		t.Errorf("Expected model 'test-model', got '%s'", loadedCfg.Model)
	}
	if loadedCfg.Approval != "auto" {
		t.Errorf("Expected approval 'auto', got '%s'", loadedCfg.Approval)
	}
}

func TestLoadConfigFromProject(t *testing.T) {
	// Crear directorio temporal
	tmpDir, err := os.MkdirTemp("", "config-project-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Cambiar al directorio temporal
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting current directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Error changing directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Crear directorio .androideai
	if err := os.MkdirAll(".androideai", 0755); err != nil {
		t.Fatalf("Error creating .androideai directory: %v", err)
	}

	// Crear config de proyecto
	cfg := DefaultConfig()
	cfg.Model = "project-model"
	configPath := filepath.Join(".androideai", "config.yml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Error saving project config: %v", err)
	}

	// Cargar config
	loadedCfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Error loading config: %v", err)
	}

	// Verificar que se cargó la config del proyecto
	if loadedCfg.Model != "project-model" {
		t.Errorf("Expected model 'project-model', got '%s'", loadedCfg.Model)
	}
}
