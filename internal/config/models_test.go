package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultModelsConfig(t *testing.T) {
	c := DefaultModelsConfig()
	if c.Agent.Provider != "opencode_zen" {
		t.Errorf("default agent provider = %q, want opencode_zen", c.Agent.Provider)
	}
	if c.Agent.Model == "" {
		t.Error("default agent model is empty")
	}
	if c.Semantic.Provider != "ollama" {
		t.Errorf("default semantic provider = %q, want ollama", c.Semantic.Provider)
	}
	if c.Semantic.BaseURL == "" {
		t.Error("default semantic base_url is empty")
	}
	if c.Semantic.ChatModel == "" {
		t.Error("default semantic chat_model is empty")
	}
	if c.Semantic.EmbeddingModel == "" {
		t.Error("default semantic embedding_model is empty")
	}
}

func TestModelsConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ModelsConfig)
		wantErr bool
	}{
		{
			name:   "defaults are valid",
			mutate: func(c *ModelsConfig) {},
		},
		{
			name:    "invalid agent provider",
			mutate:  func(c *ModelsConfig) { c.Agent.Provider = "unknown" },
			wantErr: true,
		},
		{
			name:    "empty agent model",
			mutate:  func(c *ModelsConfig) { c.Agent.Model = "" },
			wantErr: true,
		},
		{
			name:    "semantic must be ollama",
			mutate:  func(c *ModelsConfig) { c.Semantic.Provider = "anthropic" },
			wantErr: true,
		},
		{
			name:    "empty semantic base_url",
			mutate:  func(c *ModelsConfig) { c.Semantic.BaseURL = "" },
			wantErr: true,
		},
		{
			name:    "empty semantic chat_model",
			mutate:  func(c *ModelsConfig) { c.Semantic.ChatModel = "" },
			wantErr: true,
		},
		{
			name:    "empty semantic embedding_model",
			mutate:  func(c *ModelsConfig) { c.Semantic.EmbeddingModel = "" },
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := DefaultModelsConfig()
			tc.mutate(c)
			err := c.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestAgentModelConfig_APIKey(t *testing.T) {
	// Sin env var configurada y sin api_key_env: vacío
	a := AgentModelConfig{}
	if got := a.APIKey(); got != "" {
		t.Errorf("APIKey() = %q, want empty", got)
	}

	// Con api_key_env apuntando a un env var que no existe: vacío
	a.APIKeyEnv = "DEFINITELY_NOT_SET_12345"
	if got := a.APIKey(); got != "" {
		t.Errorf("APIKey() = %q, want empty for unset env", got)
	}

	// Con api_key_env apuntando a un env var seteado: lo devuelve
	t.Setenv("TEST_API_KEY_VAR", "secret-xyz")
	a.APIKeyEnv = "TEST_API_KEY_VAR"
	if got := a.APIKey(); got != "secret-xyz" {
		t.Errorf("APIKey() = %q, want secret-xyz", got)
	}
}

func TestLoadModelsConfig_NoFiles(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "models-home-test")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(tmpHome)
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	tmpDir, err := os.MkdirTemp("", "models-proj-test")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	cfg, migrated, err := LoadModelsConfig()
	if err != nil {
		t.Fatalf("LoadModelsConfig: %v", err)
	}
	// Sin config plano ni models.yml, defaults puros sin migración.
	if migrated {
		t.Error("expected no migration, got true")
	}
	if cfg.Agent.Provider != "opencode_zen" {
		t.Errorf("agent.provider = %q, want opencode_zen (default)", cfg.Agent.Provider)
	}
}

func TestLoadModelsConfig_ProjectOverridesGlobal(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "models-home-test")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(tmpHome)
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	tmpDir, err := os.MkdirTemp("", "models-proj-test")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	// Global: agent.provider = ollama
	globalDir := filepath.Join(tmpHome, ".androideai")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "models.yml"), []byte("agent:\n  provider: ollama\n  model: qwen3-coder\nsemantic:\n  provider: ollama\n  base_url: http://localhost:11434\n  chat_model: x\n  embedding_model: y\n"), 0644); err != nil {
		t.Fatalf("write global: %v", err)
	}

	// Project: agent.provider = opencode_zen
	projDir := filepath.Join(tmpDir, ".androideai")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "models.yml"), []byte("agent:\n  provider: opencode_zen\n  model: minimax-m3-free\n"), 0644); err != nil {
		t.Fatalf("write project: %v", err)
	}

	cfg, _, err := LoadModelsConfig()
	if err != nil {
		t.Fatalf("LoadModelsConfig: %v", err)
	}
	// El de proyecto gana.
	if cfg.Agent.Provider != "opencode_zen" {
		t.Errorf("agent.provider = %q, want opencode_zen (project overrides global)", cfg.Agent.Provider)
	}
	if cfg.Agent.Model != "minimax-m3-free" {
		t.Errorf("agent.model = %q, want minimax-m3-free", cfg.Agent.Model)
	}
}

func TestLoadModelsConfig_MigratesFromFlatConfig(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "models-home-test")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(tmpHome)
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	tmpDir, err := os.MkdirTemp("", "models-proj-test")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	// Solo config.yml plano (formato viejo), sin models.yml
	projDir := filepath.Join(tmpDir, ".androideai")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "config.yml"), []byte(
		"provider: opencode_zen\n"+
			"model: minimax-m3-free\n"+
			"ollama_url: http://remote:11434\n"+
			"ollama_model: qwen2.5-coder:7b\n",
	), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, migrated, err := LoadModelsConfig()
	if err != nil {
		t.Fatalf("LoadModelsConfig: %v", err)
	}
	if !migrated {
		t.Error("expected migration flag to be true")
	}
	if cfg.Agent.Provider != "opencode_zen" {
		t.Errorf("agent.provider = %q, want opencode_zen", cfg.Agent.Provider)
	}
	if cfg.Agent.Model != "minimax-m3-free" {
		t.Errorf("agent.model = %q, want minimax-m3-free", cfg.Agent.Model)
	}
	if cfg.Agent.APIKeyEnv != "OPENCODE_ZEN_API_KEY" {
		t.Errorf("agent.api_key_env = %q, want OPENCODE_ZEN_API_KEY", cfg.Agent.APIKeyEnv)
	}
	if cfg.Semantic.BaseURL != "http://remote:11434" {
		t.Errorf("semantic.base_url = %q, want http://remote:11434", cfg.Semantic.BaseURL)
	}
	if cfg.Semantic.ChatModel != "qwen2.5-coder:7b" {
		t.Errorf("semantic.chat_model = %q, want qwen2.5-coder:7b", cfg.Semantic.ChatModel)
	}
	// EmbeddingModel debe caer al OllamaModel (que es lo único que
	// teníamos en el formato viejo).
	if cfg.Semantic.EmbeddingModel != "qwen2.5-coder:7b" {
		t.Errorf("semantic.embedding_model = %q, want qwen2.5-coder:7b", cfg.Semantic.EmbeddingModel)
	}
}

func TestModelsConfig_SaveLoadRoundTrip(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "models-rt-home")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(tmpHome)
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	tmpDir, err := os.MkdirTemp("", "models-roundtrip")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	original := DefaultModelsConfig()
	original.Agent.Provider = "anthropic"
	original.Agent.Model = "claude-sonnet-4-5"
	original.Agent.APIKeyEnv = "ANTHROPIC_API_KEY"
	original.Semantic.BaseURL = "http://custom:11434"
	original.Semantic.ChatModel = "llama3.2:3b"
	original.Semantic.EmbeddingModel = "mxbai-embed-large"

	path := filepath.Join(tmpDir, ".androideai", "models.yml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := original.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Recargar desde el archivo (LoadModelsConfig encuentra el
	// projectPath).
	loaded, _, err := LoadModelsConfig()
	if err != nil {
		t.Fatalf("LoadModelsConfig: %v", err)
	}
	if loaded.Agent.Provider != "anthropic" {
		t.Errorf("agent.provider = %q, want anthropic", loaded.Agent.Provider)
	}
	if loaded.Agent.Model != "claude-sonnet-4-5" {
		t.Errorf("agent.model = %q, want claude-sonnet-4-5", loaded.Agent.Model)
	}
	if loaded.Agent.APIKeyEnv != "ANTHROPIC_API_KEY" {
		t.Errorf("agent.api_key_env = %q, want ANTHROPIC_API_KEY", loaded.Agent.APIKeyEnv)
	}
	if loaded.Semantic.BaseURL != "http://custom:11434" {
		t.Errorf("semantic.base_url = %q, want http://custom:11434", loaded.Semantic.BaseURL)
	}
	if loaded.Semantic.ChatModel != "llama3.2:3b" {
		t.Errorf("semantic.chat_model = %q, want llama3.2:3b", loaded.Semantic.ChatModel)
	}
	if loaded.Semantic.EmbeddingModel != "mxbai-embed-large" {
		t.Errorf("semantic.embedding_model = %q, want mxbai-embed-large", loaded.Semantic.EmbeddingModel)
	}
}
