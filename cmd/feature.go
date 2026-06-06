package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pablofelipe1207/androideia/internal/index"
	"github.com/pablofelipe1207/androideia/internal/store"
	"github.com/spf13/cobra"
)

var featureCmd = &cobra.Command{
	Use:   "feature [name]",
	Short: "Muestra las capas de una feature",
	Long:  `Agrupa y muestra todas las capas de una feature específica (Screen, ViewModel, UseCase, Repository, etc.).`,
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
