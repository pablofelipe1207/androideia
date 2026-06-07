package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/pablofelipe1207/androideia/internal/llm"
	"github.com/pablofelipe1207/androideia/internal/semantic"
	"github.com/pablofelipe1207/androideia/internal/store"
	"github.com/spf13/cobra"
)

var semanticCmd = &cobra.Command{
	Use:   "semantic",
	Short: "Búsqueda semántica de código",
	Long: `Realiza búsquedas semánticas usando embeddings y mantiene un índice
LLM-asistido de archivos (tipo, tags, convenciones, arquitectura) que el
agente consulta para ubicarse en el proyecto.`,
}

var semanticIndexCmd = &cobra.Command{
	Use:   "index",
	Short: "Indexa archivos (clasificación LLM + embeddings)",
	Long: `Recorre todos los archivos del proyecto y, para cada uno, le pide al
LLM que lo clasifique (ViewModel, Activity, UseCase, Repository, ...) y
almacene sus tags, convenciones y arquitectura detectada. Después
genera los embeddings por símbolo para búsqueda semántica.

Si el LLM falla, igualmente intenta generar los embeddings de los
símbolos ya indexados.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Indexing project for semantic search...")
		fmt.Println()

		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %w", err)
		}

		// Open store
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' and 'androideai index build' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Auto-resolve model from Ollama (single-model shortcut)
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

		// 1) Clasificación por archivo (LLM). Es el paso nuevo: para
		//    cada .kt/.kts del índice, el LLM devuelve {type, tags,
		//    architecture, conventions, summary}.
		if !sem.IsAvailable() {
			fmt.Printf("⚠️  Ollama is not available at %s. Skipping LLM classification, only embeddings will be refreshed.\n\n", cfg.OllamaURL)
		} else {
			fmt.Printf("→ Clasificando archivos con %s ...\n", cfg.Model)
			classified, failed, err := sem.ClassifyAllFiles()
			if err != nil {
				return fmt.Errorf("error classifying files: %w", err)
			}
			fmt.Printf("\n  Clasificados: %d   Fallidos: %d\n", classified, failed)

			arch, _, _ := sem.ArchitectureSummary()
			fmt.Printf("  Arquitectura detectada: %s\n\n", arch)
		}

		// 2) Embeddings por símbolo (comportamiento histórico). Es
		//    independiente de la clasificación: si los embeddings ya
		//    existen, no se recalculan.
		fmt.Println("→ Generando embeddings de símbolos...")
		count, err := sem.IndexAll()
		if err != nil {
			return fmt.Errorf("error indexing symbols: %w", err)
		}
		fmt.Printf("  Embeddings nuevos: %d\n\n", count)

		fmt.Println("Semantic index ready. Try:")
		fmt.Println("  androideai semantic locate viewmodel")
		fmt.Println("  androideai semantic locate LoginViewModel")
		fmt.Println("  androideai semantic locate tag:auth")
		return nil
	},
}

var semanticSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Busca código por significado (embeddings)",
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

var semanticLocateCmd = &cobra.Command{
	Use:   "locate [query]",
	Short: "Localiza archivos por tipo, tag o nombre (usa el índice LLM)",
	Long: `Pregunta al índice semántico por archivos. Ejemplos:

  androideai semantic locate viewmodel        # todos los ViewModel
  androideai semantic locate usecase           # todos los UseCase
  androideai semantic locate repository        # todos los Repository
  androideai semantic locate LoginViewModel    # un archivo concreto
  androideai semantic locate tag:auth          # cualquier archivo taggeado "auth"
  androideai semantic locate "state flow"      # búsqueda libre sobre conventions/summary

El comando muestra para cada coincidencia: ruta, tipo, tags, capa,
arquitectura, un resumen y un snippet de las convenciones detectadas.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")
		limit, _ := cmd.Flags().GetInt("limit")
		showAll, _ := cmd.Flags().GetBool("all")

		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' and 'androideai index build' first")
		}
		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %w", err)
		}
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
		sem := semantic.NewSemantic(s.DB(), cfg.OllamaURL, cfg.Model)

		if showAll {
			limit = 200
		}

		results, err := sem.Locate(query, limit)
		if err != nil {
			return fmt.Errorf("error locating files: %w", err)
		}

		if len(results) == 0 {
			fmt.Printf("Sin coincidencias para %q. ¿Ya corriste 'androideai semantic index'?\n", query)
			return nil
		}

		fmt.Printf("🔎 %d coincidencia(s) para %q:\n\n", len(results), query)
		for i, r := range results {
			fmt.Printf("%d. %s\n", i+1, r.Path)
			fmt.Printf("   📦 package : %s\n", emptyAs(r.Package, "—"))
			fmt.Printf("   🏷  type    : %s\n", r.Type)
			fmt.Printf("   🏷  tags    : %s\n", strings.Join(r.Tags, ", "))
			fmt.Printf("   🏗  layer   : %s   module: %s   arch: %s\n", emptyAs(r.Layer, "—"), emptyAs(r.Module, "—"), r.Architecture)
			if r.Summary != "" {
				fmt.Printf("   📝 summary : %s\n", r.Summary)
			}
			if r.Conventions != "" {
				fmt.Printf("   📐 conv.   : %s\n", r.Conventions)
			}
			fmt.Printf("   🔍 match   : %s\n\n", r.MatchReason)
		}

		arch, _, _ := sem.ArchitectureSummary()
		fmt.Printf("Arquitectura del proyecto: %s\n", arch)
		return nil
	},
}

var semanticStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Muestra el estado de la búsqueda semántica",
	Long:  `Muestra información sobre los embeddings, las clasificaciones LLM y la conexión con Ollama.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %w", err)
		}

		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

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

		sem := semantic.NewSemantic(s.DB(), cfg.OllamaURL, cfg.Model)

		fmt.Println("Semantic Search Status")
		fmt.Println("=====================")
		fmt.Printf("Ollama URL: %s\n", cfg.OllamaURL)
		fmt.Printf("Model:      %s\n", cfg.Model)
		if sem.IsAvailable() {
			fmt.Println("Ollama:     ✅ Available")
		} else {
			fmt.Println("Ollama:     ❌ Not available")
		}

		var embCount int
		_ = s.DB().QueryRow("SELECT COUNT(*) FROM embeddings").Scan(&embCount)
		fmt.Printf("Embeddings: %d\n", embCount)

		var symCount int
		_ = s.DB().QueryRow("SELECT COUNT(*) FROM symbols").Scan(&symCount)
		fmt.Printf("Symbols:    %d\n", symCount)

		var fileCount int
		_ = s.DB().QueryRow("SELECT COUNT(*) FROM files").Scan(&fileCount)
		fmt.Printf("Files:      %d\n", fileCount)

		var classCount int
		_ = s.DB().QueryRow("SELECT COUNT(*) FROM file_semantics").Scan(&classCount)
		fmt.Printf("Classified: %d / %d files\n", classCount, fileCount)

		arch, arches, _ := sem.ArchitectureSummary()
		fmt.Printf("Architecture: %s\n", arch)
		if len(arches) > 0 {
			fmt.Printf("  (per-layer hits: %s)\n", strings.Join(arches, ", "))
		}

		// Top types
		rows, err := s.DB().Query(
			`SELECT type, COUNT(*) AS n FROM file_semantics WHERE type IS NOT NULL AND type != ''
			 GROUP BY type ORDER BY n DESC LIMIT 8`,
		)
		if err == nil {
			defer rows.Close()
			fmt.Println("\nTop types:")
			for rows.Next() {
				var t string
				var n int
				if err := rows.Scan(&t, &n); err == nil {
					fmt.Printf("  - %-12s %d\n", t, n)
				}
			}
		}
		return nil
	},
}

func emptyAs(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func init() {
	semanticSearchCmd.Flags().IntP("limit", "l", 10, "Maximum number of results")
	semanticLocateCmd.Flags().IntP("limit", "l", 10, "Maximum number of results")
	semanticLocateCmd.Flags().BoolP("all", "a", false, "Show up to 200 results")

	semanticCmd.AddCommand(semanticIndexCmd)
	semanticCmd.AddCommand(semanticSearchCmd)
	semanticCmd.AddCommand(semanticLocateCmd)
	semanticCmd.AddCommand(semanticStatusCmd)
}
