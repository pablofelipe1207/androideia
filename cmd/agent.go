package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mobiai/androideai-core/internal/agent"
	"github.com/mobiai/androideai-core/internal/config"
	"github.com/mobiai/androideai-core/internal/llm"
	"github.com/mobiai/androideai-core/internal/store"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent [task]",
	Short: "Ejecuta el loop de desarrollo guiado",
	Long:  `Inicia el agente de desarrollo para ejecutar tareas de forma guiada.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		task := args[0]
		fmt.Printf("Starting agent for task: %s\n\n", task)

		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %w", err)
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
		agent := agent.NewAgent(llmProvider, s.DB(), cfg)
		if err := agent.Run(task); err != nil {
			return fmt.Errorf("agent error: %w", err)
		}

		return nil
	},
}
