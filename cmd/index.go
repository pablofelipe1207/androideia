package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pablofelipe1207/androideia/internal/index"
	"github.com/pablofelipe1207/androideia/internal/store"
	"github.com/spf13/cobra"
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

			fmt.Printf("Indexed %s: %d symbols\n", file.Path, len(symbols))
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
	indexCmd.AddCommand(indexBuildCmd)
	indexCmd.AddCommand(indexRefreshCmd)
}
