package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/pablofelipe1207/androideia/internal/llm"
	"github.com/pablofelipe1207/androideia/internal/semantic"
	"github.com/pablofelipe1207/androideia/internal/store"
	"github.com/spf13/cobra"
)

var semanticCmd = &cobra.Command{
	Use:   "semantic",
	Short: "Búsqueda semántica de código",
	Long:  `Realiza búsquedas semánticas usando embeddings para encontrar código por significado.`,
}

var semanticIndexCmd = &cobra.Command{
	Use:   "index",
	Short: "Indexa símbolos para búsqueda semántica",
	Long:  `Genera embeddings para todos los símbolos y los almacena para búsqueda semántica.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Indexing symbols for semantic search...")

		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %w", err)
		}

		// Open store
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai index build' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Check if Ollama is available
		probe := semantic.NewSemantic(s.DB(), cfg.OllamaURL, cfg.Model)
		if !probe.IsAvailable() {
			return fmt.Errorf("Ollama is not available at %s. Please start Ollama first.", cfg.OllamaURL)
		}

		// Auto-resolve embedding model from Ollama (single-model shortcut)
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

		// Create semantic instance with resolved model
		sem := semantic.NewSemantic(s.DB(), cfg.OllamaURL, cfg.Model)

		// Index all symbols
		count, err := sem.IndexAll()
		if err != nil {
			return fmt.Errorf("error indexing symbols: %w", err)
		}

		fmt.Printf("Indexed %d symbols for semantic search\n", count)
		return nil
	},
}

var semanticSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Busca código por significado",
	Long:  `Realiza una búsqueda semántica para encontrar código relacionado por significado.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		limit, _ := cmd.Flags().GetInt("limit")

		fmt.Printf("Semantic search for: %s\n\n", query)

		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %w", err)
		}

		// Open store
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai index build' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Auto-resolve embedding model from Ollama (single-model shortcut)
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

		// Create semantic instance
		sem := semantic.NewSemantic(s.DB(), cfg.OllamaURL, cfg.Model)

		// Search
		results, err := sem.Search(query, limit)
		if err != nil {
			return fmt.Errorf("error searching: %w", err)
		}

		if len(results) == 0 {
			fmt.Println("No results found")
			return nil
		}

		fmt.Printf("Found %d results:\n\n", len(results))
		for i, result := range results {
			fmt.Printf("%d. %s (%s)\n", i+1, result.Name, result.Kind)
			fmt.Printf("   📍 %s:%d\n", result.Path, result.Line)
			fmt.Printf("   📊 Similarity: %.2f\n\n", result.Similarity)
		}

		return nil
	},
}

var semanticStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Muestra el estado de la búsqueda semántica",
	Long:  `Muestra información sobre el estado de los embeddings y la conexión con Ollama.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// Auto-resolve embedding model from Ollama (single-model shortcut)
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

		// Create semantic instance
		sem := semantic.NewSemantic(s.DB(), cfg.OllamaURL, cfg.Model)

		// Check Ollama availability
		fmt.Println("Semantic Search Status")
		fmt.Println("=====================")
		fmt.Printf("Ollama URL: %s\n", cfg.OllamaURL)
		fmt.Printf("Model: %s\n", cfg.Model)

		if sem.IsAvailable() {
			fmt.Println("Ollama Status: ✅ Available")
		} else {
			fmt.Println("Ollama Status: ❌ Not available")
		}

		// Count embeddings
		var count int
		err = s.DB().QueryRow("SELECT COUNT(*) FROM embeddings").Scan(&count)
		if err != nil {
			return fmt.Errorf("error counting embeddings: %w", err)
		}
		fmt.Printf("Embeddings indexed: %d\n", count)

		// Count symbols
		var symbolCount int
		err = s.DB().QueryRow("SELECT COUNT(*) FROM symbols").Scan(&symbolCount)
		if err != nil {
			return fmt.Errorf("error counting symbols: %w", err)
		}
		fmt.Printf("Total symbols: %d\n", symbolCount)

		return nil
	},
}

func init() {
	// Search command flags
	semanticSearchCmd.Flags().IntP("limit", "l", 10, "Maximum number of results")

	// Add commands
	semanticCmd.AddCommand(semanticIndexCmd)
	semanticCmd.AddCommand(semanticSearchCmd)
	semanticCmd.AddCommand(semanticStatusCmd)
}
