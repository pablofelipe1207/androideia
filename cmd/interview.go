package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/pablofelipe1207/androideia/internal/interview"
	"github.com/pablofelipe1207/androideia/internal/llm"
	"github.com/pablofelipe1207/androideia/internal/store"
	"github.com/spf13/cobra"
)

var (
	interviewCategory   string
	interviewDifficulty string
	interviewCount      int
	interviewModel      string
	interviewTimeout    int
	interviewNoLLM      bool
)

var interviewCmd = &cobra.Command{
	Use:   "interview",
	Short: "Inicia una entrevista técnica de Android",
	Long: `Ejecuta una sesión interactiva de entrevista técnica sobre Android.

Puedes filtrar por categoría y dificultad. Las respuestas se evalúan
inmediatamente con explicaciones detalladas.

Ejemplos:
  androideai interview                          # Entrevista general
  androideai interview --difficulty easy         # Solo preguntas fáciles
  androideai interview --category compose        # Solo Compose
  androideai interview --count 5 --difficulty hard  # 5 preguntas difíciles`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Cargar configuración de modelos
		mc, _, err := config.LoadModelsConfig()
		if err != nil {
			return fmt.Errorf("error loading models config: %w", err)
		}

		// Override model from flag
		if interviewModel != "" {
			mc.Agent.Model = interviewModel
		}

		// Resolver modelo
		if mc.Agent.Provider == "ollama" {
			baseURL := mc.Agent.BaseURL
			if baseURL == "" {
				baseURL = mc.Semantic.BaseURL
			}
			if baseURL == "" {
				baseURL = "http://localhost:11434"
			}
			resolved, _, err := llm.ResolveOllamaModel(baseURL, mc.Agent.Model)
			if err != nil {
				return fmt.Errorf("error resolving model: %w", err)
			}
			mc.Agent.Model = resolved
		}

		// Crear LLM provider
		var llmProvider llm.Provider
		if !interviewNoLLM {
			timeoutDur := time.Duration(interviewTimeout) * time.Second
			if timeoutDur <= 0 {
				timeoutDur = 60 * time.Second
			}

			switch mc.Agent.Provider {
			case "ollama":
				baseURL := mc.Agent.BaseURL
				if baseURL == "" {
					baseURL = mc.Semantic.BaseURL
				}
				if baseURL == "" {
					baseURL = "http://localhost:11434"
				}
				llmProvider = llm.NewOllamaProviderWithTimeout(baseURL, mc.Agent.Model, timeoutDur)
			case "opencode_zen":
				llmProvider = llm.NewOpenCodeZenProviderWithOptions(
					mc.Agent.Model,
					mc.Agent.APIKey(),
					mc.Agent.BaseURL,
					timeoutDur,
				)
			}
		}

		// Abrir DB
		var db *store.Store
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); err == nil {
			db, err = store.NewStore(dbPath)
			if err != nil {
				fmt.Printf("  (Advertencia: no se pudo abrir DB: %v)\n", err)
			} else {
				defer db.Close()
			}
		}

		// Configurar entrevista
		config := interview.InterviewConfig{
			Category:   interview.Category(interviewCategory),
			Difficulty: interview.Difficulty(interviewDifficulty),
			Count:      interviewCount,
			UseLLM:     !interviewNoLLM && llmProvider != nil && llmProvider.IsAvailable(),
		}

		// Crear y ejecutar entrevista
		var interviewDB *sql.DB
		if db != nil {
			interviewDB = db.DB()
		}

		intv := interview.NewInterview(interviewDB, llmProvider, config)
		return intv.Run()
	},
}

var interviewHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Muestra el historial de entrevistas",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Abrir DB
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("no hay historial (ejecuta 'androideai interview' primero)")
		}

		db, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer db.Close()

		interview.PrintHistory(db.DB())
		return nil
	},
}

var interviewCategoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "Lista las categorías disponibles",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\n  Categorías disponibles:")
		fmt.Println("  " + strings.Repeat("-", 40))
		for _, cat := range interview.AllCategories() {
			fmt.Printf("  • %s\n", cat)
		}
		fmt.Println()
	},
}

var interviewDifficultiesCmd = &cobra.Command{
	Use:   "difficulties",
	Short: "Lista las dificultades disponibles",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\n  Dificultades disponibles:")
		fmt.Println("  " + strings.Repeat("-", 40))
		for _, diff := range interview.AllDifficulties() {
			fmt.Printf("  • %s\n", diff)
		}
		fmt.Println()
	},
}

func init() {
	interviewCmd.Flags().StringVar(&interviewCategory, "category", "", "Categoría: compose, architecture, di, async, storage, navigation, testing")
	interviewCmd.Flags().StringVar(&interviewDifficulty, "difficulty", "", "Dificultad: easy, medium, hard")
	interviewCmd.Flags().IntVar(&interviewCount, "count", 10, "Número de preguntas")
	interviewCmd.Flags().StringVarP(&interviewModel, "model", "m", "", "Override del modelo LLM")
	interviewCmd.Flags().IntVar(&interviewTimeout, "timeout", 60, "Timeout en segundos para el LLM")
	interviewCmd.Flags().BoolVar(&interviewNoLLM, "no-llm", false, "Deshabilitar generación de preguntas por LLM")

	interviewCmd.AddCommand(interviewHistoryCmd)
	interviewCmd.AddCommand(interviewCategoriesCmd)
	interviewCmd.AddCommand(interviewDifficultiesCmd)

	rootCmd.AddCommand(interviewCmd)
}
