package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ModelsConfig es la configuración dedicada a modelos. Separa
// claramente dos responsabilidades:
//
//   - Agent: provider + modelo que usa el agente para chat (tool
//     calling incluido).
//   - Semantic: provider + modelos que usa el flujo de `semantic
//     index` (clasificación LLM de archivos + generación de
//     embeddings).
//
// Esta separación importa porque las dos responsabilidades son
// independientes: el agente puede hablar con OpenCode Zen (free,
// rápido, sin GPU) mientras que el semantic index corre contra
// Ollama local (porque es el único que tiene embeddings
// implementados a la fecha).
//
// El archivo se guarda en `~/.androideai/models.yml` (global) y se
// puede sobrescribir por proyecto en `./.androideai/models.yml`.
//
// Formato:
//
//	agent:
//	  provider: opencode_zen
//	  model: minimax-m3-free
//	  base_url: ""                 # opcional
//	  api_key_env: ""              # opcional (nombre de var de entorno)
//
//	semantic:
//	  provider: ollama
//	  base_url: http://localhost:11434
//	  chat_model: qwen2.5-coder:7b
//	  embedding_model: nomic-embed-text
type ModelsConfig struct {
	Agent    AgentModelConfig    `yaml:"agent"`
	Semantic SemanticModelConfig `yaml:"semantic"`

	// Paths internos (no se serializan).
	GlobalPath  string `yaml:"-"`
	ProjectPath string `yaml:"-"`
}

// AgentModelConfig define el provider y modelo que usa el agente
// (chat). Soporta los mismos providers que Config.Provider:
//   - ollama
//   - anthropic
//   - openai
//   - opencode_zen
type AgentModelConfig struct {
	Provider   string `yaml:"provider"`
	Model      string `yaml:"model"`
	BaseURL    string `yaml:"base_url,omitempty"`
	APIKeyEnv  string `yaml:"api_key_env,omitempty"`
}

// SemanticModelConfig define el provider y modelos que usa el
// semantic index. `ChatModel` se usa para clasificación LLM de
// archivos; `EmbeddingModel` se usa para generar embeddings de
// símbolos. `BaseURL` apunta a la API del provider (típicamente
// Ollama local). `APIKeyEnv` es el nombre de la variable de entorno
// que contiene la API key (para providers como Google Gemini).
type SemanticModelConfig struct {
	Provider       string `yaml:"provider"`
	BaseURL        string `yaml:"base_url"`
	ChatModel      string `yaml:"chat_model"`
	EmbeddingModel string `yaml:"embedding_model"`
	APIKeyEnv      string `yaml:"api_key_env,omitempty"`
}

// DefaultModelsConfig devuelve los defaults sensatos:
//
//   - Agent: OpenCode Zen con Mimo V2.5 Free (no requiere key, no
//     requiere GPU).
//   - Semantic: OpenCode Zen con Mimo V2.5 Free para clasificación
//     (sin embeddings locales, usa FTS para búsqueda).
//
// Si el usuario quiere embeddings locales, puede cambiar semantic
// a provider "ollama" y configurar los modelos de embeddings.
func DefaultModelsConfig() *ModelsConfig {
	return &ModelsConfig{
		Agent: AgentModelConfig{
			Provider: "opencode_zen",
			Model:    "mimo-v2.5-free",
		},
		Semantic: SemanticModelConfig{
			Provider:       "opencode_zen",
			BaseURL:        "",
			ChatModel:      "mimo-v2.5-free",
			EmbeddingModel: "",
		},
	}
}

// LoadModelsConfig carga el `models.yml` con prioridad al del
// proyecto sobre el global, igual que LoadConfig.
//
// Si no existe ningún `models.yml` (instalación fresca o proyecto
// nuevo), intenta migrar desde el `config.yml` plano. Si tampoco
// existe `config.yml`, devuelve los defaults.
//
// Si existe `config.yml` plano PERO no `models.yml`, hace la
// migración automáticamente y devuelve el resultado. El flag
// `Migrated` se setea en el wrapper de retorno para que el caller
// pueda avisar al usuario que corra `androideai init` o
// `androideai models init` para persistir el nuevo formato.
func LoadModelsConfig() (*ModelsConfig, bool, error) {
	cfg := DefaultModelsConfig()

	// 1. Intentar cargar `models.yml` global.
	globalPath := modelsGlobalPath()
	cfg.GlobalPath = globalPath
	if _, err := os.Stat(globalPath); err == nil {
		if err := cfg.loadFromFile(globalPath); err != nil {
			return nil, false, fmt.Errorf("error loading global models.yml: %w", err)
		}
	}

	// 2. Sobreescribir con `models.yml` de proyecto.
	projectPath := modelsProjectPath()
	cfg.ProjectPath = projectPath
	if _, err := os.Stat(projectPath); err == nil {
		if err := cfg.loadFromFile(projectPath); err != nil {
			return nil, false, fmt.Errorf("error loading project models.yml: %w", err)
		}
	}

	// 3. Si no había `models.yml` en ningún lado, intentar migrar
	//    desde `config.yml` plano.
	globalExists := fileExists(globalPath)
	projectExists := fileExists(projectPath)
	if !globalExists && !projectExists {
		migrated, err := tryMigrateFromFlatConfig(cfg)
		if err != nil {
			return nil, false, err
		}
		if migrated {
			return cfg, true, nil
		}
	}

	return cfg, false, nil
}

// tryMigrateFromFlatConfig intenta poblar el ModelsConfig con los
// valores del config.yml plano. Devuelve true si encontró un
// config.yml REAL en disco (no defaults) y migró.
func tryMigrateFromFlatConfig(cfg *ModelsConfig) (bool, error) {
	// Sólo intentamos migrar si el config.yml plano existe REALMENTE
	// en disco. Si no existe, los defaults de Config tienen
	// `Provider: "ollama"` que haría que cualquier "migración"
	// parezca haber encontrado un config cuando en realidad no.
	flat, err := LoadConfig()
	if err != nil {
		return false, nil
	}
	// LoadConfig carga global + project; ver si alguno existe.
	if !fileExists(flat.GlobalPath) && !fileExists(flat.ProjectPath) {
		return false, nil
	}

	cfg.Agent.Provider = flat.Provider
	cfg.Agent.Model = flat.Model
	cfg.Semantic.Provider = "ollama" // forzado por ahora
	cfg.Semantic.BaseURL = flat.OllamaURL
	if flat.OllamaModel != "" {
		cfg.Semantic.ChatModel = flat.OllamaModel
		cfg.Semantic.EmbeddingModel = flat.OllamaModel
	}

	// Para OpenAI/Anthropic/Zen intentamos inferir el env var de la key.
	switch flat.Provider {
	case "anthropic":
		cfg.Agent.APIKeyEnv = "ANTHROPIC_API_KEY"
	case "openai":
		cfg.Agent.APIKeyEnv = "OPENAI_API_KEY"
	case "opencode_zen":
		cfg.Agent.APIKeyEnv = "OPENCODE_ZEN_API_KEY"
	}

	return true, nil
}

func (c *ModelsConfig) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, c)
}

// Save escribe el config al path indicado. Si path está vacío,
// escribe al projectPath si existe, si no al globalPath.
func (c *ModelsConfig) Save(path string) error {
	if path == "" {
		if c.ProjectPath != "" {
			path = c.ProjectPath
		} else {
			path = c.GlobalPath
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating models config dir: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("error marshaling models config: %w", err)
	}

	// Escribir con un header explicativo.
	header := []byte("# androideai-core — model configuration\n" +
		"#\n" +
		"# Este archivo controla qué provider/modelo usa el agente\n" +
		"# (chat) y qué modelos usa el flujo de semantic index\n" +
		"# (clasificación LLM de archivos + generación de embeddings).\n" +
		"#\n" +
		"# Editar a mano es la forma más rápida. También se puede usar\n" +
		"# `androideai models show` para ver el efectivo y\n" +
		"# `androideai models set <seccion.campo> <valor>` para modificar.\n" +
		"#\n" +
		"# docs: https://github.com/pablofelipe1207/androideai-core\n\n")
	final := append(header, data...)

	if err := os.WriteFile(path, final, 0644); err != nil {
		return fmt.Errorf("error writing models config: %w", err)
	}
	return nil
}

// APIKey devuelve la API key del agente leyendo la variable de
// entorno indicada en APIKeyEnv. Si APIKeyEnv está vacío, devuelve
// string vacío (típicamente, el tier free de OpenCode Zen).
func (a *AgentModelConfig) APIKey() string {
	if a.APIKeyEnv == "" {
		return ""
	}
	return os.Getenv(a.APIKeyEnv)
}

// SemanticAPIKey devuelve la API key del semantic leyendo la variable
// de entorno indicada en APIKeyEnv. Útil para providers como
// Google Gemini que requieren API key.
func (s *SemanticModelConfig) APIKey() string {
	if s.APIKeyEnv == "" {
		return ""
	}
	return os.Getenv(s.APIKeyEnv)
}

// Validate revisa que los campos requeridos estén bien. Útil para
// fallar rápido en vez de obtener errores crípticos del provider.
func (c *ModelsConfig) Validate() error {
	validProviders := map[string]bool{
		"ollama": true, "anthropic": true, "openai": true, "opencode_zen": true,
	}
	if !validProviders[c.Agent.Provider] {
		return fmt.Errorf("agent.provider %q inválido (usa: ollama, anthropic, openai, opencode_zen)", c.Agent.Provider)
	}
	if c.Agent.Model == "" {
		return fmt.Errorf("agent.model no puede estar vacío")
	}
	validSemanticProviders := map[string]bool{
		"ollama": true, "opencode_zen": true, "openai": true, "google_gemini": true,
	}
	if !validSemanticProviders[c.Semantic.Provider] {
		return fmt.Errorf("semantic.provider debe ser 'ollama', 'opencode_zen', 'openai' o 'google_gemini'")
	}
	if c.Semantic.ChatModel == "" {
		return fmt.Errorf("semantic.chat_model no puede estar vacío")
	}
	// For google_gemini, api_key_env is required
	if c.Semantic.Provider == "google_gemini" && c.Semantic.APIKeyEnv == "" {
		return fmt.Errorf("semantic.api_key_env es requerido para google_gemini (ej: GEMINI_API_KEY)")
	}
	// embedding_model es opcional (solo necesario si se usan embeddings)
	return nil
}

func modelsGlobalPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".androideai", "models.yml")
}

func modelsProjectPath() string {
	return filepath.Join(".androideai", "models.yml")
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
