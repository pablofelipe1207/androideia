package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Muestra o modifica la configuración de androideai",
	Long: `Gestiona la configuración del agente androideai.

Por defecto, los comandos leen y escriben la configuración del proyecto
actual (./.androideai/config.yml). Usa --global para trabajar sobre la
configuración global del usuario (~/.androideai/config.yml).

La configuración de proyecto se fusiona sobre la global, y la gana
cuando ambas definen la misma clave.`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Muestra la configuración efectiva (proyecto + global)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		fmt.Printf("model:       %s\n", cfg.Model)
		fmt.Printf("ollama_url:  %s\n", cfg.OllamaURL)
		fmt.Printf("provider:    %s\n", cfg.Provider)
		fmt.Printf("approval:    %s\n", cfg.Approval)
		fmt.Println()
		fmt.Printf("global:      %s\n", cfg.GlobalPath)
		fmt.Printf("project:     %s\n", cfg.ProjectPath)
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Obtiene el valor efectivo de una clave de configuración",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		val, err := getConfigValue(cfg, args[0])
		if err != nil {
			return err
		}
		fmt.Println(val)
		return nil
	},
}

var (
	configSetGlobal bool
)

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Establece un valor de configuración",
	Long: `Establece un valor en la configuración. Por defecto modifica
la configuración del proyecto (./.androideai/config.yml). Usa --global
para modificar la configuración global (~/.androideai/config.yml).

Claves válidas: model, ollama_url, provider, approval`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]

		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("error getting home directory: %w", err)
		}

		var path string
		if configSetGlobal {
			path = filepath.Join(home, ".androideai", "config.yml")
		} else {
			path = filepath.Join(".androideai", "config.yml")
		}

		cfg, err := config.LoadConfigFromFile(path)
		if err != nil {
			return err
		}

		if err := setConfigValue(cfg, key, value); err != nil {
			return err
		}

		if err := cfg.Save(path); err != nil {
			return err
		}

		scope := "project"
		if configSetGlobal {
			scope = "global"
		}
		fmt.Printf("✅ %s.%s = %s (guardado en %s)\n", scope, key, value, path)
		return nil
	},
}

func init() {
	configSetCmd.Flags().BoolVarP(&configSetGlobal, "global", "g", false, "Modificar la configuración global (~/.androideai/config.yml)")
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
}

func getConfigValue(cfg *config.Config, key string) (string, error) {
	switch key {
	case "model":
		return cfg.Model, nil
	case "ollama_url", "ollama-url":
		return cfg.OllamaURL, nil
	case "ollama_model", "ollama-model":
		return cfg.OllamaModel, nil
	case "provider":
		return cfg.Provider, nil
	case "approval":
		return cfg.Approval, nil
	default:
		return "", fmt.Errorf("clave desconocida: %q (usa: model, ollama_url, ollama_model, provider, approval)", key)
	}
}

func setConfigValue(cfg *config.Config, key, value string) error {
	switch key {
	case "model":
		if value == "" {
			return fmt.Errorf("el modelo no puede estar vacío")
		}
		cfg.Model = value
	case "ollama_url", "ollama-url":
		if value == "" {
			return fmt.Errorf("la URL no puede estar vacía")
		}
		cfg.OllamaURL = value
	case "ollama_model", "ollama-model":
		// Vacío significa "volver al comportamiento por defecto"
		// (cae a cfg.Model).
		cfg.OllamaModel = value
	case "provider":
		switch value {
		case "ollama", "anthropic", "openai", "opencode_zen":
			cfg.Provider = value
		default:
			return fmt.Errorf("provider inválido: %q (usa: ollama, anthropic, openai, opencode_zen)", value)
		}
	case "approval":
		switch value {
		case "ask", "auto", "never":
			cfg.Approval = value
		default:
			return fmt.Errorf("approval inválido: %q (usa: ask, auto, never)", value)
		}
	default:
		return fmt.Errorf("clave desconocida: %q (usa: model, ollama_url, provider, approval)", key)
	}
	return nil
}
