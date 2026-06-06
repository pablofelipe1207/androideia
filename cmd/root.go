package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "androideai",
	Short: "androideai-core es un agente de desarrollo Android offline-first",
	Long:  `Un CLI independiente en Go que actúa como agente de desarrollo Android offline-first con índice SQLite, memoria de proyecto, ops Gradle/adb, loop de agente con Ollama, y skills extensibles.`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Muestra la versión de androideai",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("androideai %s\n", Version)
		fmt.Printf("Build: %s\n", BuildTime)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(indexCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(featureCmd)
	rootCmd.AddCommand(brainCmd)
	rootCmd.AddCommand(androidCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(semanticCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(modelsCmd)
}
