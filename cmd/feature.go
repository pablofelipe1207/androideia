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
			return fmt.Errorf("database not found, run 'androideai init' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Buscar viewmodels en file_semantics y extraer nombre base
		rows, err := s.DB().Query(`
			SELECT f.path
			FROM file_semantics fs
			JOIN files f ON f.id = fs.file_id
			WHERE fs.type = 'viewmodel'
		`)
		if err != nil {
			return fmt.Errorf("error querying features: %w", err)
		}
		defer rows.Close()

		features := make(map[string]int)
		for rows.Next() {
			var path string
			if err := rows.Scan(&path); err != nil {
				continue
			}
			// Extraer nombre del archivo: CounterViewModel.kt -> counter
			base := filepath.Base(path)
			base = strings.TrimSuffix(base, ".kt")
			base = strings.TrimSuffix(base, ".java")
			base = strings.TrimSuffix(base, "ViewModel")
			base = strings.TrimSuffix(base, "vm")
			feature := strings.ToLower(base)
			if feature == "" {
				continue
			}
			features[feature]++
		}

		// Contar archivos relacionados por cada feature
		for feat := range features {
			var count int
			s.DB().QueryRow(`
				SELECT COUNT(*) FROM files
				WHERE LOWER(path) LIKE '%' || ? || '%'
			`, feat).Scan(&count)
			features[feat] = count
		}

		fmt.Println("Features descubiertas:")
		fmt.Println(strings.Repeat("─", 50))
		found := false
		for feat, count := range features {
			if count <= 2 {
				continue
			}
			found = true
			fmt.Printf("  %s (%d archivos)\n", feat, count)
		}
		if !found {
			fmt.Println("  (ninguna feature etiquetada aún)")
			fmt.Println("  Ejecuta 'androideai init' para descubrir features")
		}

		return nil
	},
}

func init() {
	featureCmd.AddCommand(featureShowCmd)
	featureCmd.AddCommand(featureListCmd)
}
