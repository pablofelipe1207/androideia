package cmd

import (
	"fmt"

	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/pablofelipe1207/androideia/internal/llm"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Gestiona los modelos LLM disponibles",
	Long:  `Lista o consulta los modelos disponibles en el proveedor LLM configurado.`,
}

var modelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista los modelos disponibles en Ollama",
	Long: `Lista los modelos instalados en Ollama (consulta GET /api/tags).

También marca cuál sería el modelo que se usaría al ejecutar el agente,
siguiendo las reglas de auto-selección:
  - Si Ollama tiene exactamente 1 modelo, se usa ese.
  - Si el modelo configurado está en la lista, se usa ese.
  - Si ninguno coincide, el agente fallará con un error que lista los
    modelos disponibles.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %w", err)
		}

		if cfg.Provider != "ollama" {
			return fmt.Errorf("listado de modelos solo soportado para provider=ollama (actual: %s)", cfg.Provider)
		}

		provider := llm.NewOllamaProvider(cfg.OllamaURL, cfg.Model)
		models, err := provider.ListModels()
		if err != nil {
			return fmt.Errorf("error listando modelos: %w", err)
		}

		fmt.Printf("Modelos disponibles en Ollama (%s):\n", cfg.OllamaURL)
		if len(models) == 0 {
			fmt.Println("  (ninguno — instala uno con: ollama pull <modelo>)")
			return nil
		}

		for _, m := range models {
			marker := "  "
			if m == cfg.Model {
				marker = "* "
			}
			fmt.Printf("%s%s\n", marker, m)
		}

		// Show what would be auto-selected
		resolved, autoSelected, err := llm.ResolveOllamaModel(cfg.OllamaURL, cfg.Model)
		if err != nil {
			fmt.Printf("\n⚠️  %v\n", err)
			return nil
		}
		fmt.Println()
		if autoSelected {
			fmt.Printf("Auto-selección: %s (configurado: %s)\n", resolved, cfg.Model)
		} else if resolved != cfg.Model {
			fmt.Printf("Modelo efectivo: %s (configurado: %s)\n", resolved, cfg.Model)
		} else {
			fmt.Printf("Modelo configurado disponible: %s\n", resolved)
		}

		return nil
	},
}

func init() {
	modelsCmd.AddCommand(modelsListCmd)
}
