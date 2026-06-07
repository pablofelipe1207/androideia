package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/pablofelipe1207/androideia/internal/scaffold"
	"github.com/spf13/cobra"
)

var scaffoldCmd = &cobra.Command{
	Use:   "scaffold <role> <Name>",
	Short: "Genera (o muestra) una plantilla canónica Android para un rol",
	Long: `Devuelve la plantilla canónica Kotlin para uno de los roles
soportados. La misma plantilla es la que usa la herramienta del agente
'android_scaffold', así que el resultado es estable.

Roles soportados:
  viewmodel, composable, activity, usecase, repository, dao,
  di_module, data_class, entity, nav_route

Sustituciones aplicadas:
  <Name>               Nombre PascalCase del componente (p.ej. Login)
  <feature>            Slug en minúsculas (p.ej. login) — default = <Name> en minúsculas
  <package>            Paquete destino — default = com.example.app.feature.<feature>
  <appPackage>         Paquete raíz de la app — default = com.example.app
  <repositoryPackage>  Paquete del repositorio (para UseCase) — default = com.example.app.feature.<feature>
  <RepositoryName>     Nombre del repo (para UseCase) — default = <Name>Repository
  <entity_name>        Nombre de la entidad (para DAO) — default = <Name>Entity
  <table>              Nombre de tabla SQL — default = <feature>s
  <return_type>        Tipo de retorno del UseCase — default = Unit

Por defecto la plantilla se imprime por pantalla. Con --output <path>
se escribe directamente a disco.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		roleStr, name := args[0], args[1]
		role := scaffold.Role(roleStr)
		if !scaffold.IsValidRole(role) {
			return fmt.Errorf("rol desconocido %q (soportados: %s)", roleStr, strings.Join(rolesList(), ", "))
		}

		spec, err := scaffold.SpecFor(role)
		if err != nil {
			return err
		}

		feature, _ := cmd.Flags().GetString("feature")
		if feature == "" {
			feature = strings.ToLower(name)
		}
		pkg, _ := cmd.Flags().GetString("package")
		if pkg == "" {
			pkg = "com.example.app.feature." + feature
		}
		appPkg, _ := cmd.Flags().GetString("app-package")
		if appPkg == "" {
			appPkg = "com.example.app"
		}
		repoPkg, _ := cmd.Flags().GetString("repository-package")
		if repoPkg == "" {
			repoPkg = pkg
		}
		repoName, _ := cmd.Flags().GetString("repository-name")
		if repoName == "" {
			repoName = name + "Repository"
		}
		entityName, _ := cmd.Flags().GetString("entity-name")
		if entityName == "" {
			entityName = name + "Entity"
		}
		table, _ := cmd.Flags().GetString("table")
		if table == "" {
			table = feature + "s"
		}
		returnType, _ := cmd.Flags().GetString("return-type")
		if returnType == "" {
			returnType = "Unit"
		}

		vars := scaffold.TemplateVars{
			Package:           pkg,
			AppPackage:        appPkg,
			RepositoryPackage: repoPkg,
			Name:              name,
			Feature:           feature,
			UseCaseName:       name,
			UseCaseCamel:      feature,
			RepositoryName:    repoName,
			EntityName:        entityName,
			Table:             table,
			ReturnType:        returnType,
		}
		rendered := scaffold.RenderTemplate(spec.Template, vars)

		outPath, _ := cmd.Flags().GetString("output")
		if outPath != "" {
			if err := os.WriteFile(outPath, []byte(rendered), 0644); err != nil {
				return fmt.Errorf("error escribiendo %s: %w", outPath, err)
			}
			fmt.Printf("✓ Plantilla %s escrita a %s\n", role, outPath)
			fmt.Println("  Recuerda rellenar los TODO y luego correr:")
			fmt.Printf("    androideai validate %s %s\n", outPath, role)
			return nil
		}

		fmt.Printf("Plantilla %s (%s)\n", role, spec.DisplayName)
		fmt.Printf("FileNameHint: %s\n\n", spec.FileNameHint)
		fmt.Println("```kotlin")
		fmt.Println(rendered)
		fmt.Println("```")
		fmt.Println()
		fmt.Println("Reglas de validación:")
		for _, r := range spec.Rules {
			fmt.Printf("  • %s\n", r.Description)
		}
		return nil
	},
}

var scaffoldListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista los roles soportados por 'androideai scaffold'",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Roles soportados:")
		for _, r := range scaffold.AllRoles() {
			spec, err := scaffold.SpecFor(r)
			if err != nil {
				continue
			}
			fmt.Printf("  • %-12s %s\n", r, spec.DisplayName)
		}
		return nil
	},
}

func rolesList() []string {
	out := make([]string, 0, len(scaffold.AllRoles()))
	for _, r := range scaffold.AllRoles() {
		out = append(out, string(r))
	}
	return out
}

func init() {
	scaffoldCmd.Flags().String("feature", "", "Slug del feature (default: <Name> en minúsculas)")
	scaffoldCmd.Flags().String("package", "", "Paquete destino (default: com.example.app.feature.<feature>)")
	scaffoldCmd.Flags().String("app-package", "", "Paquete raíz de la app (default: com.example.app)")
	scaffoldCmd.Flags().String("repository-package", "", "Paquete del repositorio (para UseCase)")
	scaffoldCmd.Flags().String("repository-name", "", "Nombre del repositorio (para UseCase)")
	scaffoldCmd.Flags().String("entity-name", "", "Nombre de la entidad (para DAO)")
	scaffoldCmd.Flags().String("table", "", "Nombre de la tabla SQL (para DAO/Entity)")
	scaffoldCmd.Flags().String("return-type", "", "Tipo de retorno del UseCase (default: Unit)")
	scaffoldCmd.Flags().StringP("output", "o", "", "Escribe la plantilla a un archivo en vez de imprimirla")

	scaffoldCmd.AddCommand(scaffoldListCmd)
}
