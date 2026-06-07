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

func TestLoadConfigFromFile_Missing(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "non-existent-config.yml")
	cfg, err := LoadConfigFromFile(tmpFile)
	if err != nil {
		t.Fatalf("Expected no error for missing file, got: %v", err)
	}
	if cfg.Model != "qwen3-coder-64k-32k:latest" {
		t.Errorf("Expected default model, got '%s'", cfg.Model)
	}
}

func TestLoadConfigFromFile_Override(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-loadfile-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.Model = "custom-model"
	cfg.Approval = "auto"
	path := filepath.Join(tmpDir, "config.yml")
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Error saving config: %v", err)
	}

	loaded, err := LoadConfigFromFile(path)
	if err != nil {
		t.Fatalf("Error loading config: %v", err)
	}
	if loaded.Model != "custom-model" {
		t.Errorf("Expected model 'custom-model', got '%s'", loaded.Model)
	}
	if loaded.Approval != "auto" {
		t.Errorf("Expected approval 'auto', got '%s'", loaded.Approval)
	}
}

func TestLoadConfig_GlobalAndProjectMerge(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "config-home-test")
	if err != nil {
		t.Fatalf("Error creating temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)
	os.Setenv("HOME", tmpHome)
	defer os.Unsetenv("HOME")

	tmpDir, err := os.MkdirTemp("", "config-proj-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	// Global config defines model
	globalDir := filepath.Join(tmpHome, ".androideai")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatal(err)
	}
	globalCfg := DefaultConfig()
	globalCfg.Model = "global-model"
	globalCfg.Approval = "auto"
	globalCfg.Provider = "ollama"
	if err := globalCfg.Save(filepath.Join(globalDir, "config.yml")); err != nil {
		t.Fatal(err)
	}

	// Project config overrides model only (write partial YAML manually so
	// the other fields fall through to the global config)
	if err := os.MkdirAll(".androideai", 0755); err != nil {
		t.Fatal(err)
	}
	partialYAML := "model: project-model\n"
	if err := os.WriteFile(filepath.Join(".androideai", "config.yml"), []byte(partialYAML), 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Model != "project-model" {
		t.Errorf("Expected project model 'project-model', got '%s'", loaded.Model)
	}
	if loaded.Approval != "auto" {
		t.Errorf("Expected global approval 'auto', got '%s'", loaded.Approval)
	}
	if loaded.Provider != "ollama" {
		t.Errorf("Expected global provider 'ollama', got '%s'", loaded.Provider)
	}
}

func TestEffectiveOllamaModel(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		ollamaModel string
		want        string
	}{
		{
			name:        "ollama_model configured wins",
			model:       "minimax-m3-free", // agente usa Zen
			ollamaModel: "qwen2.5-coder:7b", // clasificación+embeddings usan Ollama
			want:        "qwen2.5-coder:7b",
		},
		{
			name:        "ollama_model empty falls back to model",
			model:       "qwen3-coder:latest",
			ollamaModel: "",
			want:        "qwen3-coder:latest",
		},
		{
			name:        "both empty falls back to empty model",
			model:       "",
			ollamaModel: "",
			want:        "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{Model: tc.model, OllamaModel: tc.ollamaModel}
			if got := c.EffectiveOllamaModel(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestConfig_LoadsOllamaModel(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-ollama-model-test")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// LoadConfig lee ~/.androideai; apuntamos HOME a un dir vacío
	// para que no se cargue nada global.
	tmpHome, err := os.MkdirTemp("", "config-home-test")
	if err != nil {
		t.Fatalf("home tempdir: %v", err)
	}
	defer os.RemoveAll(tmpHome)
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	// Sin ~/.androideai ni .androideai: solo defaults
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.OllamaModel != "" {
		t.Errorf("default OllamaModel = %q, want empty", cfg.OllamaModel)
	}

	// Config explícito
	cfgPath := filepath.Join(tmpDir, ".androideai", "config.yml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte("model: minimax-m3-free\nollama_model: qwen2.5-coder:7b\nprovider: opencode_zen\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err = LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Model != "minimax-m3-free" {
		t.Errorf("Model = %q, want minimax-m3-free", cfg.Model)
	}
	if cfg.OllamaModel != "qwen2.5-coder:7b" {
		t.Errorf("OllamaModel = %q, want qwen2.5-coder:7b", cfg.OllamaModel)
	}
	if cfg.Provider != "opencode_zen" {
		t.Errorf("Provider = %q, want opencode_zen", cfg.Provider)
	}
	if cfg.EffectiveOllamaModel() != "qwen2.5-coder:7b" {
		t.Errorf("EffectiveOllamaModel = %q, want qwen2.5-coder:7b", cfg.EffectiveOllamaModel())
	}
}
