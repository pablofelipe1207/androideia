package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pablofelipe1207/androideia/internal/agent"
	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/pablofelipe1207/androideia/internal/llm"
	"github.com/pablofelipe1207/androideia/internal/memory"
	"github.com/pablofelipe1207/androideia/internal/store"
	"github.com/spf13/cobra"
)

var (
	agentModel    string
	agentResumeID int64
	agentSession  string
)

var agentCmd = &cobra.Command{
	Use:   "agent [task]",
	Short: "Ejecuta el loop de desarrollo guiado",
	Long: `Inicia el agente de desarrollo para ejecutar tareas de forma guiada.

La sesión queda persistida en .androideai/core.db; puedes verla con
"androideai memory list" y continuarla con --resume <id>.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// El task es opcional si se hace --resume.
		var task string
		if len(args) == 1 {
			task = args[0]
		}

		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %w", err)
		}

		// Override model from --model flag if provided
		if agentModel != "" {
			cfg.Model = agentModel
			fmt.Printf("Using model override: %s\n", cfg.Model)
		}

		// Auto-resolve model from Ollama if provider is ollama.
		if cfg.Provider == "ollama" {
			resolved, autoSelected, err := llm.ResolveOllamaModel(cfg.OllamaURL, cfg.Model)
			if err != nil {
				return fmt.Errorf("error resolving model: %w", err)
			}
			if autoSelected {
				fmt.Printf("Ollama has a single model installed; using %s (config had %s)\n", resolved, cfg.Model)
			}
			cfg.Model = resolved
		}

		// Open store
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Create LLM provider
		var llmProvider llm.Provider
		switch cfg.Provider {
		case "ollama":
			llmProvider = llm.NewOllamaProvider(cfg.OllamaURL, cfg.Model)
		case "anthropic":
			apiKey := os.Getenv("ANTHROPIC_API_KEY")
			llmProvider = llm.NewAnthropicProvider(apiKey, cfg.Model)
		case "openai":
			apiKey := os.Getenv("OPENAI_API_KEY")
			llmProvider = llm.NewOpenAIProvider(apiKey, cfg.Model, "")
		default:
			return fmt.Errorf("unknown provider: %s", cfg.Provider)
		}

		// Check if provider is available
		if !llmProvider.IsAvailable() {
			return fmt.Errorf("LLM provider '%s' is not available. Please check your configuration.", cfg.Provider)
		}

		// Create and run agent
		ag := agent.NewAgent(llmProvider, s.DB(), cfg)
		mem := memory.NewMemory(s.DB())
		ag.SetMemory(mem)

		// Reanudar conversación previa si se pidió.
		if agentResumeID > 0 {
			original, err := ag.ResumeSession(agentResumeID)
			if err != nil {
				return fmt.Errorf("error resuming session: %w", err)
			}
			fmt.Printf("[Memory] Resuming conversation #%d\n", agentResumeID)
			fmt.Printf("[Memory] Original task: %s\n", original)
			if task != "" {
				fmt.Printf("[Memory] New follow-up: %s\n", task)
			}
		} else if task == "" {
			return fmt.Errorf("provide a task, or use --resume <id> to continue a previous conversation")
		}

		if err := ag.Run(task); err != nil {
			return fmt.Errorf("agent error: %w", err)
		}

		// Informar al usuario dónde queda la sesión para futuros resumes.
		if id := ag.ConversationID(); id > 0 {
			fmt.Printf("\n[Memory] Conversación guardada con ID %d. Usa 'androideai memory show %d' para revisarla o 'androideai agent --resume %d \"...\"' para continuarla.\n", id, id, id)
		}

		return nil
	},
}

func init() {
	agentCmd.Flags().StringVarP(&agentModel, "model", "m", "", "Override del modelo LLM para esta ejecución (ej: qwen3-coder-64k-32k:latest)")
	agentCmd.Flags().Int64Var(&agentResumeID, "resume", 0, "Reanuda una conversación persistida por ID")
	agentCmd.Flags().StringVar(&agentSession, "session", "", "Nombre opcional para la sesión (sólo metadata)")
}
