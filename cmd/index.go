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

		// Extract symbols and store
		// Use tree-sitter parser for better accuracy
		extractor := index.NewTreeSitterExtractor()
		fmt.Println("Using tree-sitter parser for precise symbol extraction")

		for _, file := range files {
			content, err := os.ReadFile(file.Path)
			if err != nil {
				return fmt.Errorf("error reading file %s: %w", file.Path, err)
			}

			// Extract metadata
			file.Package = extractor.InferPackage(string(content))
			file.Module = extractor.InferModule(file.Path)
			file.Layer = extractor.InferLayer(file.Path, string(content))

			// Insert file
			result, err := s.DB().Exec(
				"INSERT OR REPLACE INTO files (path, package, module, layer, hash, updated_at) VALUES (?, ?, ?, ?, ?, strftime('%s', 'now'))",
				file.Path, file.Package, file.Module, file.Layer, file.Hash,
			)
			if err != nil {
				return fmt.Errorf("error inserting file %s: %w", file.Path, err)
			}

			fileID, err := result.LastInsertId()
			if err != nil {
				return fmt.Errorf("error getting file ID: %w", err)
			}

			// Extract symbols
			symbols := extractor.ExtractSymbols(file.Path, string(content))

			// Auto-infer feature name from symbols and tag them (heuristic fallback)
			featureName := extractor.ExtractFeature(symbols)
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

		// Optional: LLM-based feature discovery (Ollama)
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
					fileToFeature, err := sem.DiscoverFeatures()
					if err != nil {
						fmt.Printf("Warning: LLM feature discovery failed: %v\n", err)
					} else {
						tagged, err := sem.TagSymbolsWithFeatures(fileToFeature)
						if err != nil {
							fmt.Printf("Warning: Failed to tag features: %v\n", err)
						} else {
							fmt.Printf("LLM feature discovery: tagged %d symbols across %d files\n", tagged, len(fileToFeature))
							// Print discovered features
							features := make(map[string]bool)
							for _, f := range fileToFeature {
								features[f] = true
							}
							for feat := range features {
								fmt.Printf("  Discovered feature: %s\n", feat)
							}
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
