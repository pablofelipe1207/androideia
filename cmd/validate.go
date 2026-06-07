package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/pablofelipe1207/androideia/internal/scaffold"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate <file> <role>",
	Short: "Valida que un archivo .kt/.kts cumple el contrato de su rol",
	Long: `Lee <file> y comprueba que cumple las reglas de validación
estáticas del rol indicado (las mismas que aplica el agente con la
herramienta 'validate_kotlin').

Roles soportados:
  viewmodel, composable, activity, usecase, repository, dao,
  di_module, data_class, entity, nav_route

Salida:
  - Una línea de resumen (OK / OK con N warning(s) / FAIL).
  - Una lista de issues con severidad, regla y mensaje.

Exit code:
  0 si pasa (con o sin warnings), 1 si hay errores.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, roleStr := args[0], args[1]
		role := scaffold.Role(roleStr)
		if !scaffold.IsValidRole(role) {
			return fmt.Errorf("rol desconocido %q (soportados: %s)", roleStr, strings.Join(rolesList(), ", "))
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error leyendo %s: %w", path, err)
		}

		issues := scaffold.Validate(string(content), role)
		fmt.Printf("%s — %s\n\n", path, scaffold.Summary(issues))
		if len(issues) == 0 {
			return nil
		}
		for _, i := range issues {
			marker := "✗"
			if i.Severity == "warning" {
				marker = "!"
			}
			loc := ""
			if i.Line > 0 {
				loc = fmt.Sprintf(" (line %d)", i.Line)
			}
			fmt.Printf("  %s [%s] %s%s\n", marker, i.Rule, i.Message, loc)
		}

		// Exit code != 0 si hay errores (no warnings), para integrarlo
		// en CI / pre-commit / gradle hook.
		for _, i := range issues {
			if i.Severity == "error" {
				os.Exit(1)
			}
		}
		return nil
	},
}

func init() {
	validateCmd.Flags().Bool("strict", false, "(reservado) Trata warnings como errores")
}
