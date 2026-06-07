package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pablofelipe1207/androideia/internal/index"
	"github.com/pablofelipe1207/androideia/internal/store"
	"github.com/spf13/cobra"
)

var featureCmd = &cobra.Command{
	Use:   "feature [command]",
	Short: "Gestiona y muestra features",
	Long:  `Agrupa y muestra las capas de features (Screen, ViewModel, UseCase, Repository, etc.).`,
}

var featureShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Muestra las capas de una feature",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		featureName := args[0]
		fmt.Printf("Looking for feature: %s\n\n", featureName)

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

		// Get feature
		feature, err := index.GetFeatureByName(s.DB(), featureName)
		if err != nil {
			return fmt.Errorf("error getting feature: %w", err)
		}

		// Print feature
		fmt.Print(feature.Format())

		return nil
	},
}

var featureListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista todas las features descubiertas en el índice",
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai index build' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		rows, err := s.DB().Query(`
			SELECT DISTINCT feature, COUNT(*) as count
			FROM symbols
			WHERE feature != '' AND feature IS NOT NULL
			GROUP BY feature
			ORDER BY count DESC
		`)
		if err != nil {
			return fmt.Errorf("error querying features: %w", err)
		}
		defer rows.Close()

		fmt.Println("Features descubiertas:")
		fmt.Println(strings.Repeat("─", 50))
		found := false
		for rows.Next() {
			var featureName string
			var count int
			if err := rows.Scan(&featureName, &count); err != nil {
				continue
			}
			found = true
			fmt.Printf("  %s (%d símbolos)\n", featureName, count)
		}
		if !found {
			fmt.Println("  (ninguna feature etiquetada aún)")
			fmt.Println("  Ejecuta 'androideai index build' para auto-etiquetar")
		}

		return nil
	},
}

func init() {
	featureCmd.AddCommand(featureShowCmd)
	featureCmd.AddCommand(featureListCmd)
}
