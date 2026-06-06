package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pablofelipe1207/androideia/internal/store"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search [keyword]",
	Short: "Busca en el índice de código",
	Long:  `Realiza búsquedas FTS5 en el índice de código fuente.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		keyword := args[0]
		fmt.Printf("Searching for: %s\n", keyword)

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

		// Search using FTS5
		rows, err := s.DB().Query(
			`SELECT f.path, s.line, snippet(symbols_fts, 0, '<b>', '</b>', '...', 20) as snippet
			FROM symbols_fts
			JOIN symbols s ON s.name = symbols_fts.name
			JOIN files f ON s.file_id = f.id
			WHERE symbols_fts MATCH ?
			ORDER BY rank`,
			keyword,
		)
		if err != nil {
			return fmt.Errorf("error searching: %w", err)
		}
		defer rows.Close()

		found := false
		for rows.Next() {
			var path string
			var line int
			var snippet string
			if err := rows.Scan(&path, &line, &snippet); err != nil {
				return fmt.Errorf("error scanning row: %w", err)
			}
			fmt.Printf("%s:%d %s\n", path, line, snippet)
			found = true
		}

		if !found {
			fmt.Println("No results found")
		}

		return nil
	},
}

var symbolCmd = &cobra.Command{
	Use:   "symbol [name|kind]",
	Short: "Busca símbolos específicos",
	Long:  `Filtra símbolos por nombre o tipo.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		fmt.Printf("Searching symbol: %s\n", query)

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

		// Search by name or kind
		rows, err := s.DB().Query(
			`SELECT s.name, s.kind, f.path, s.line
			FROM symbols s
			JOIN files f ON s.file_id = f.id
			WHERE s.name LIKE ? OR s.kind LIKE ?
			ORDER BY s.name`,
			"%"+query+"%", "%"+query+"%",
		)
		if err != nil {
			return fmt.Errorf("error searching symbols: %w", err)
		}
		defer rows.Close()

		found := false
		for rows.Next() {
			var name, kind, path string
			var line int
			if err := rows.Scan(&name, &kind, &path, &line); err != nil {
				return fmt.Errorf("error scanning row: %w", err)
			}
			fmt.Printf("%s (%s) %s:%d\n", name, kind, path, line)
			found = true
		}

		if !found {
			fmt.Println("No symbols found")
		}

		return nil
	},
}

func init() {
	searchCmd.AddCommand(symbolCmd)
}
