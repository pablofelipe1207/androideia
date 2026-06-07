package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/pablofelipe1207/androideia/internal/llm"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Configura y muestra los modelos del agente y del semantic index",
	Long: `Gestiona el archivo models.yml que controla qué provider/modelo
usa el agente (chat) y qué modelos usa el flujo de semantic index
(clasificación LLM de archivos + generación de embeddings).

Subcomandos:
  list   Lista los modelos disponibles en el provider del agente.
  show   Muestra la configuración efectiva (con paths).
  set    Modifica un campo (sección.campo valor).
  init   Crea models.yml con defaults si no existe.
  path   Muestra las rutas de los archivos models.yml.`,
}

var modelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista los modelos disponibles en el provider del agente",
	Long: `Lista los modelos disponibles en el provider configurado
(agent.provider en models.yml).

Soportados:
  - ollama:        GET <base_url>/api/tags
  - opencode_zen:  GET <base_url>/models
  - openai:        no implementado (la API no expone listado de modelos
                    accesibles con la key)
  - anthropic:     no implementado

Para Ollama, además marca con * el modelo que el agente usaría
(auto-selección cuando hay 1 modelo en Ollama).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		mc, _, err := config.LoadModelsConfig()
		if err != nil {
			return fmt.Errorf("error loading models config: %w", err)
		}

		provider := mc.Agent.Provider
		baseURL := mc.Agent.BaseURL
		apiKey := mc.Agent.APIKey()

		var models []string
		switch provider {
		case "ollama":
			if baseURL == "" {
				baseURL = "http://localhost:11434"
			}
			p := llm.NewOllamaProvider(baseURL, mc.Agent.Model)
			models, err = p.ListModels()
			if err != nil {
				return fmt.Errorf("error listando modelos de Ollama: %w", err)
			}
			fmt.Printf("Modelos disponibles en Ollama (%s):\n", baseURL)
		case "opencode_zen":
			if baseURL == "" {
				baseURL = "https://opencode.ai/zen/v1"
			}
			p := llm.NewOpenCodeZenProviderWithOptions(mc.Agent.Model, apiKey, baseURL, 30*time.Second)
			models, err = p.ListModels()
			if err != nil {
				return fmt.Errorf("error listando modelos de OpenCode Zen: %w", err)
			}
			fmt.Printf("Modelos disponibles en OpenCode Zen (%s):\n", baseURL)
		default:
			return fmt.Errorf("provider %q no soporta listado de modelos", provider)
		}

		if len(models) == 0 {
			fmt.Println("  (ninguno)")
			return nil
		}

		for _, m := range models {
			marker := "  "
			if m == mc.Agent.Model {
				marker = "* "
			}
			fmt.Printf("%s%s\n", marker, m)
		}

		fmt.Println()
		fmt.Printf("Configurado: %s (%s)\n", mc.Agent.Model, provider)
		return nil
	},
}

var modelsShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Muestra la configuración efectiva de modelos",
	Long: `Lee models.yml (global + proyecto mergeado) y la muestra
formateada. Si no existe, intenta migrar desde el config.yml plano
antiguo.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, migrated, err := config.LoadModelsConfig()
		if err != nil {
			return err
		}
		if migrated {
			fmt.Println("⚠️  No se encontró models.yml; se migró desde config.yml (formato antiguo).")
			fmt.Println("    Ejecutá 'androideai models init' para persistir el nuevo formato.")
		}
		if err := cfg.Validate(); err != nil {
			fmt.Printf("⚠️  Config inválida: %v\n", err)
		}

		fmt.Println("─── Agent (chat del agente) ───")
		fmt.Printf("  provider:    %s\n", cfg.Agent.Provider)
		fmt.Printf("  model:       %s\n", cfg.Agent.Model)
		if cfg.Agent.BaseURL != "" {
			fmt.Printf("  base_url:    %s\n", cfg.Agent.BaseURL)
		}
		if cfg.Agent.APIKeyEnv != "" {
			fmt.Printf("  api_key_env: %s\n", cfg.Agent.APIKeyEnv)
		}

		fmt.Println()
		fmt.Println("─── Semantic index (clasificación + embeddings) ───")
		fmt.Printf("  provider:        %s\n", cfg.Semantic.Provider)
		fmt.Printf("  base_url:        %s\n", cfg.Semantic.BaseURL)
		fmt.Printf("  chat_model:      %s\n", cfg.Semantic.ChatModel)
		fmt.Printf("  embedding_model: %s\n", cfg.Semantic.EmbeddingModel)

		fmt.Println()
		fmt.Println("─── Paths ───")
		if cfg.GlobalPath != "" {
			marker := ""
			if _, err := os.Stat(cfg.GlobalPath); err == nil {
				marker = " (existe)"
			} else {
				marker = " (no existe)"
			}
			fmt.Printf("  global:  %s%s\n", cfg.GlobalPath, marker)
		}
		if cfg.ProjectPath != "" {
			marker := ""
			if _, err := os.Stat(cfg.ProjectPath); err == nil {
				marker = " (existe)"
			} else {
				marker = " (no existe)"
			}
			fmt.Printf("  project: %s%s\n", cfg.ProjectPath, marker)
		}
		return nil
	},
}

var modelsSetCmd = &cobra.Command{
	Use:   "set <seccion.campo> <valor>",
	Short: "Modifica un campo de models.yml",
	Long: `Modifica un campo del archivo models.yml del proyecto.

Ejemplos:
  androideai models set agent.provider opencode_zen
  androideai models set agent.model minimax-m3-free
  androideai models set semantic.chat_model qwen2.5-coder:7b
  androideai models set semantic.embedding_model nomic-embed-text
  androideai models set agent.api_key_env OPENCODE_ZEN_API_KEY`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		value := args[1]

		cfg, migrated, err := config.LoadModelsConfig()
		if err != nil {
			return err
		}
		if migrated {
			fmt.Println("⚠️  models.yml no existía; se creó con defaults + migración del config.yml antiguo.")
		}

		if err := setModelsField(cfg, path, value); err != nil {
			return err
		}

		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("valor inválido: %w", err)
		}

		// Guardar al archivo de proyecto.
		savePath := cfg.ProjectPath
		if savePath == "" {
			savePath = cfg.GlobalPath
		}
		// Si la ruta es la global y el flag --global no se pasó, igual
		// escribimos al projectPath (que es donde normalmente el usuario
		// espera sus cambios). Si el projectPath no tiene dir, lo creamos.
		if err := cfg.Save(savePath); err != nil {
			return err
		}

		// Sincronizar el `config.yml` plano para que el flujo de
		// semantic index (init, semantic index) vea los cambios
		// sin necesidad de migrar.
		if err := syncFlatConfigFromModels(cfg); err != nil {
			fmt.Printf("⚠️  models.yml guardado, pero no se pudo sincronizar config.yml: %v\n", err)
		}

		fmt.Printf("✅ %s.%s = %s (guardado en %s)\n", sectionName(path), fieldName(path), value, savePath)
		return nil
	},
}

var modelsInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Crea models.yml con defaults si no existe",
	Long: `Crea el archivo models.yml (global o de proyecto, según el flag
--global). Si ya existe un config.yml plano (formato antiguo), los
valores se migran; si no, se usan los defaults sensatos.

Útil para arrancar en un entorno nuevo o para materializar el
formato nuevo desde uno viejo.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		global, _ := cmd.Flags().GetBool("global")
		var path string
		if global {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("no se pudo obtener HOME: %w", err)
			}
			path = filepath.Join(home, ".androideai", "models.yml")
		} else {
			path = filepath.Join(".androideai", "models.yml")
		}
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("• %s ya existe; se conserva.\n", path)
			return nil
		}
		// Crear el directorio padre si no existe.
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("error creando directorio: %w", err)
		}
		// Intentar migrar desde el config.yml plano (si existe);
		// si no, defaults.
		cfg, _, err := config.LoadModelsConfig()
		if err != nil {
			return err
		}
		if err := cfg.Save(path); err != nil {
			return err
		}
		fmt.Printf("✓ Creado %s\n", path)
		if cfg.Agent.Provider == "opencode_zen" && cfg.Semantic.ChatModel == "qwen2.5-coder:7b" {
			fmt.Println("  Con defaults sensatos: agent=OpenCode Zen, semantic=Ollama.")
		} else {
			fmt.Printf("  Migrado desde config.yml: agent=%s/%s, semantic.chat=%s.\n",
				cfg.Agent.Provider, cfg.Agent.Model, cfg.Semantic.ChatModel)
		}
		fmt.Println("  Editá los valores a mano o usá 'androideai models set <campo> <valor>'.")
		return nil
	},
}

var modelsPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Muestra las rutas donde se buscan models.yml",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := config.LoadModelsConfig()
		if err != nil {
			return err
		}
		if cfg.GlobalPath != "" {
			fmt.Println(cfg.GlobalPath)
		}
		if cfg.ProjectPath != "" {
			fmt.Println(cfg.ProjectPath)
		}
		return nil
	},
}

// setModelsField aplica `value` al campo `path` (formato
// "section.field") del cfg. Devuelve error si el path no es válido.
func setModelsField(cfg *config.ModelsConfig, path, value string) error {
	switch path {
	case "agent.provider":
		valid := map[string]bool{
			"ollama": true, "anthropic": true, "openai": true, "opencode_zen": true,
		}
		if !valid[value] {
			return fmt.Errorf("provider %q inválido (usa: ollama, anthropic, openai, opencode_zen)", value)
		}
		cfg.Agent.Provider = value
	case "agent.model":
		if value == "" {
			return fmt.Errorf("agent.model no puede estar vacío")
		}
		cfg.Agent.Model = value
	case "agent.base_url":
		cfg.Agent.BaseURL = value
	case "agent.api_key_env":
		cfg.Agent.APIKeyEnv = value
	case "semantic.provider":
		if value != "ollama" {
			return fmt.Errorf("semantic.provider debe ser 'ollama' (único con embeddings implementados)")
		}
		cfg.Semantic.Provider = value
	case "semantic.base_url":
		if value == "" {
			return fmt.Errorf("semantic.base_url no puede estar vacío")
		}
		cfg.Semantic.BaseURL = value
	case "semantic.chat_model":
		if value == "" {
			return fmt.Errorf("semantic.chat_model no puede estar vacío")
		}
		cfg.Semantic.ChatModel = value
	case "semantic.embedding_model":
		if value == "" {
			return fmt.Errorf("semantic.embedding_model no puede estar vacío")
		}
		cfg.Semantic.EmbeddingModel = value
	default:
		return fmt.Errorf("campo %q desconocido (usa: agent.{provider,model,base_url,api_key_env} o semantic.{provider,base_url,chat_model,embedding_model})", path)
	}
	return nil
}

func sectionName(path string) string {
	if i := strings.Index(path, "."); i > 0 {
		return path[:i]
	}
	return path
}

func fieldName(path string) string {
	if i := strings.Index(path, "."); i > 0 {
		return path[i+1:]
	}
	return path
}

// syncFlatConfigFromModels refleja los valores de models.yml en el
// `config.yml` plano, para que el flujo de semantic index
// (clasificación+embeddings) los vea sin necesidad de migrar.
//
// Es idempotente: si el config.yml no tiene cambios pendientes, no
// lo toca. Si no existe, lo crea.
func syncFlatConfigFromModels(mc *config.ModelsConfig) error {
	flat, err := config.LoadConfig()
	if err != nil {
		// Si no existe config.yml, lo creamos con defaults.
		flat = config.DefaultConfig()
	}

	changed := false
	if flat.Provider != mc.Agent.Provider {
		flat.Provider = mc.Agent.Provider
		changed = true
	}
	if flat.Model != mc.Agent.Model {
		flat.Model = mc.Agent.Model
		changed = true
	}
	if flat.OllamaURL != mc.Semantic.BaseURL {
		flat.OllamaURL = mc.Semantic.BaseURL
		changed = true
	}
	if flat.OllamaModel != mc.Semantic.ChatModel {
		flat.OllamaModel = mc.Semantic.ChatModel
		changed = true
	}

	if !changed {
		return nil
	}

	// Guardar en el projectPath si existe, si no en el globalPath.
	savePath := flat.ProjectPath
	if savePath == "" {
		savePath = flat.GlobalPath
	}
	return flat.Save(savePath)
}

func init() {
	modelsInitCmd.Flags().BoolP("global", "g", false, "Crear models.yml global en vez del del proyecto")
	modelsCmd.AddCommand(modelsListCmd)
	modelsCmd.AddCommand(modelsShowCmd)
	modelsCmd.AddCommand(modelsSetCmd)
	modelsCmd.AddCommand(modelsInitCmd)
	modelsCmd.AddCommand(modelsPathCmd)
	rootCmd.AddCommand(modelsCmd)
}
