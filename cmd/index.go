package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/pablofelipe1207/androideia/internal/index"
	"github.com/pablofelipe1207/androideia/internal/semantic"
	"github.com/pablofelipe1207/androideia/internal/store"
	"github.com/spf13/cobra"
)

var (
	indexUseLLM bool
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Comandos de indexación de código",
	Long:  `Indexa y gestiona el índice de código fuente.`,
}

var indexBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Construye el índice de código",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Building code index...")

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

		// Walk files
		walker := index.NewWalker(".")
		if err := walker.LoadGitignore(); err != nil {
			return fmt.Errorf("error loading .gitignore: %w", err)
		}

		files, err := walker.Walk()
		if err != nil {
			return fmt.Errorf("error walking files: %w", err)
		}

		fmt.Printf("Found %d Kotlin files\n", len(files))

		// Limpiar índices anteriores para re-indexar limpio
		s.DB().Exec("DELETE FROM symbols")
		s.DB().Exec("DELETE FROM symbols_fts")

		// Extract symbols and store
		// Try tree-sitter parser first, fallback to regex-based KotlinExtractor
		treeSitterExtractor := index.NewTreeSitterExtractor()
		kotlinExtractor := index.NewKotlinExtractor()
		fmt.Println("Using tree-sitter parser with regex fallback for symbol extraction")

		for _, file := range files {
			content, err := os.ReadFile(file.Path)
			if err != nil {
				return fmt.Errorf("error reading file %s: %w", file.Path, err)
			}

			// Extract metadata (use kotlinExtractor for package/module/layer since tree-sitter stub doesn't implement them)
			file.Package = kotlinExtractor.InferPackage(string(content))
			file.Module = kotlinExtractor.InferModule(file.Path)
			file.Layer = kotlinExtractor.InferLayer(file.Path, string(content))

			// Insert file
			_, err = s.DB().Exec(
				"INSERT OR REPLACE INTO files (path, package, module, layer, hash, updated_at) VALUES (?, ?, ?, ?, ?, strftime('%s', 'now'))",
				file.Path, file.Package, file.Module, file.Layer, file.Hash,
			)
			if err != nil {
				return fmt.Errorf("error inserting file %s: %w", file.Path, err)
			}

			// Get file ID (query instead of LastInsertId to avoid issues with INSERT OR REPLACE)
			var fileID int64
			err = s.DB().QueryRow("SELECT id FROM files WHERE path = ?", file.Path).Scan(&fileID)
			if err != nil {
				return fmt.Errorf("error getting file ID for %s: %w", file.Path, err)
			}

			// Extract symbols - try tree-sitter first, fallback to regex
			symbols := treeSitterExtractor.ExtractSymbols(file.Path, string(content))
			if len(symbols) == 0 {
				// Fallback to regex-based extractor
				symbols = kotlinExtractor.ExtractSymbols(file.Path, string(content))
			}

			// Auto-infer feature name from symbols and tag them (heuristic fallback)
			featureName := kotlinExtractor.ExtractFeature(symbols)
			if featureName != "" {
				for i := range symbols {
					symbols[i].Feature = featureName
				}
			}

			for _, sym := range symbols {
				_, err := s.DB().Exec(
					"INSERT INTO symbols (file_id, name, kind, signature, line, feature) VALUES (?, ?, ?, ?, ?, ?)",
					fileID, sym.Name, sym.Kind, sym.Signature, sym.Line, sym.Feature,
				)
				if err != nil {
					return fmt.Errorf("error inserting symbol %s: %w", sym.Name, err)
				}

				// Insert into FTS
				_, err = s.DB().Exec(
					"INSERT INTO symbols_fts (name, signature, package, path, doc) VALUES (?, ?, ?, ?, ?)",
					sym.Name, sym.Signature, file.Package, file.Path, sym.Signature,
				)
				if err != nil {
					return fmt.Errorf("error inserting into FTS: %w", err)
				}
			}

			fmt.Printf("Indexed %s: %d symbols (feature: %s)\n", file.Path, len(symbols), featureName)
		}

		// Optional: LLM-based feature discovery (Ollama) - only for capable models
		// Disabled by default because small models (qwen2.5:1.5b) can't follow complex grouping instructions.
		// The heuristic ExtractFeature() already works correctly (e.g. extracts "counter" from CounterViewModel).
		// Enable with --use-llm ONLY if using a capable model (qwen2.5-coder:7b+, llama3+, etc.)
		if indexUseLLM {
			fmt.Println("Running LLM-based feature discovery...")
			mc, _, err := config.LoadModelsConfig()
			if err == nil && mc.Semantic.Provider == "ollama" {
				cfg, _ := config.LoadConfig()
				baseURL := mc.Semantic.BaseURL
				if baseURL == "" {
					baseURL = "http://localhost:11434"
				}
				model := mc.Semantic.ChatModel
				if model == "" {
					model = cfg.EffectiveOllamaModel()
				}

				sem := semantic.NewSemantic(s.DB(), baseURL, model)
				if sem.IsAvailable() {
					// Collect heuristic features already found
					heuristicFeatures := make(map[string]bool)
					rows, err := s.DB().Query(`
						SELECT DISTINCT feature, COUNT(*) 
						FROM symbols 
						WHERE feature != '' AND feature IS NOT NULL
						GROUP BY feature
					`)
					if err == nil {
						defer rows.Close()
						for rows.Next() {
							var feat string
							var count int
							if rows.Scan(&feat, &count) == nil {
								heuristicFeatures[feat] = true
							}
						}
					}

					fileToFeature, err := sem.DiscoverFeatures()
					if err != nil {
						fmt.Printf("Warning: LLM feature discovery failed: %v\n", err)
					} else {
						// Merge: keep heuristic features, add LLM-only ones
						for _, feat := range fileToFeature {
							heuristicFeatures[feat] = true
						}
						fmt.Printf("LLM feature discovery: %d features total (heuristic + LLM)\n", len(heuristicFeatures))
						for feat := range heuristicFeatures {
							fmt.Printf("  Feature: %s\n", feat)
						}
					}
				} else {
					fmt.Println("  Ollama not available, skipping LLM feature discovery")
				}
			}
		}

		fmt.Println("Index build complete!")
		return nil
	},
}

var indexRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresca el índice existente",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Refreshing code index...")
		// For now, just run build
		return indexBuildCmd.RunE(cmd, args)
	},
}

func init() {
	indexBuildCmd.Flags().BoolVar(&indexUseLLM, "use-llm", false, "Usar Ollama para descubrimiento inteligente de features")
	indexCmd.AddCommand(indexBuildCmd)
	indexCmd.AddCommand(indexRefreshCmd)
}
