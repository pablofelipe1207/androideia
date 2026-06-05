package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mobiai/androideai-core/internal/config"
	"github.com/mobiai/androideai-core/internal/store"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Inicializa la configuración y base de datos del proyecto",
	Long:  `Crea el directorio .androideai/, archivos de configuración y base de datos del proyecto.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Inicializando androideai-core...")

		// Crear directorio .androideai
		if err := os.MkdirAll(".androideai", 0755); err != nil {
			return fmt.Errorf("error creando directorio .androideai: %w", err)
		}

		// Crear archivo .gitignore si no existe
		gitignorePath := ".gitignore"
		gitignoreContent := ".androideai/core.db\n"

		if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
			if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
				return fmt.Errorf("error creando .gitignore: %w", err)
			}
			fmt.Println("Created .gitignore")
		} else {
			fmt.Println(".gitignore already exists, skipping")
		}

		// Crear config.yml por defecto si no existe
		configPath := filepath.Join(".androideai", "config.yml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			cfg := config.DefaultConfig()
			if err := cfg.Save(configPath); err != nil {
				return fmt.Errorf("error creando config.yml: %w", err)
			}
			fmt.Println("Created .androideai/config.yml")
		} else {
			fmt.Println("Config already exists, skipping")
		}

		// Crear base de datos
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			s, err := store.NewStore(dbPath)
			if err != nil {
				return fmt.Errorf("error creating database: %w", err)
			}
			defer s.Close()
			fmt.Println("Created .androideai/core.db with schema")
		} else {
			fmt.Println("Database already exists, skipping")
		}

		fmt.Println("Initialization complete!")
		return nil
	},
}
