package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pablofelipe1207/androideia/internal/scaffold"
)

func registerScaffoldTools(s *Server) {
	type templateArgs struct {
		Role              string `json:"role" jsonschema:"rol del componente: viewmodel, composable, activity, usecase, repository, dao, di_module, data_class, entity, nav_route"`
		Name              string `json:"name" jsonschema:"nombre PascalCase del componente (ej: Login)"`
		Feature           string `json:"feature,omitempty" jsonschema:"slug en minúsculas (ej: login). Default: name en minúsculas"`
		Package           string `json:"package,omitempty" jsonschema:"paquete destino. Default: com.example.app.feature.<feature>"`
		AppPackage        string `json:"app_package,omitempty" jsonschema:"paquete raíz de la app. Default: com.example.app"`
		RepositoryPackage string `json:"repository_package,omitempty" jsonschema:"paquete del repositorio (para UseCase)"`
		RepositoryName    string `json:"repository_name,omitempty" jsonschema:"nombre del repositorio (para UseCase). Default: <Name>Repository"`
		EntityName        string `json:"entity_name,omitempty" jsonschema:"nombre de la entidad (para DAO). Default: <Name>Entity"`
		Table             string `json:"table,omitempty" jsonschema:"nombre de tabla SQL (para DAO/Entity). Default: <feature>s"`
		ReturnType        string `json:"return_type,omitempty" jsonschema:"tipo de retorno del UseCase. Default: Unit"`
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "scaffold_template",
		Description: "Devuelve la plantilla canónica Kotlin para un rol de Android (ViewModel, Composable, Activity, UseCase, Repository, DAO, DI Module, data class, Entity, Nav Route). Incluye reglas de validación.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in templateArgs) (*mcp.CallToolResult, any, error) {
		role := scaffold.Role(in.Role)
		if !scaffold.IsValidRole(role) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Rol desconocido: %s. Roles soportados: %s", in.Role, strings.Join(scaffoldAllRoles(), ", "))}},
				IsError: true,
			}, nil, nil
		}
		spec, err := scaffold.SpecFor(role)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error obteniendo spec: %v", err)}},
				IsError: true,
			}, nil, nil
		}

		name := in.Name
		if name == "" {
			name = "Example"
		}
		feature := in.Feature
		if feature == "" {
			feature = strings.ToLower(name)
		}
		pkg := in.Package
		if pkg == "" {
			pkg = "com.example.app.feature." + feature
		}
		appPkg := in.AppPackage
		if appPkg == "" {
			appPkg = "com.example.app"
		}
		repoPkg := in.RepositoryPackage
		if repoPkg == "" {
			repoPkg = pkg
		}
		repoName := in.RepositoryName
		if repoName == "" {
			repoName = name + "Repository"
		}
		entityName := in.EntityName
		if entityName == "" {
			entityName = name + "Entity"
		}
		table := in.Table
		if table == "" {
			table = feature + "s"
		}
		returnType := in.ReturnType
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

		var sb strings.Builder
		fmt.Fprintf(&sb, "## Plantilla: %s (%s)\n", role, spec.DisplayName)
		fmt.Fprintf(&sb, "Archivo sugerido: %s\n\n", spec.FileNameHint)
		fmt.Fprintf(&sb, "```kotlin\n%s\n```\n\n", rendered)
		fmt.Fprintf(&sb, "Reglas de validación:\n")
		for _, r := range spec.Rules {
			status := "✓"
			if !r.MustMatch {
				status = "✗"
			}
			fmt.Fprintf(&sb, "  %s %s\n", status, r.Description)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil, nil
	})

	s.mcp.AddTool(&mcp.Tool{
		Name:         "scaffold_list",
		Description:  "Lista todos los roles de plantillas canónicas disponibles para scaffolding Android.",
		InputSchema:  json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		roles := scaffold.AllRoles()
		var sb strings.Builder
		fmt.Fprintf(&sb, "Plantillas canónicas disponibles (%d):\n\n", len(roles))
		for _, r := range roles {
			spec, err := scaffold.SpecFor(r)
			if err != nil {
				continue
			}
			fmt.Fprintf(&sb, "- **%s** — %s\n  Archivo: %s\n\n", r, spec.DisplayName, spec.FileNameHint)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil
	})

	type validateArgs struct {
		Path    string `json:"path" jsonschema:"ruta del archivo .kt a validar"`
		Content string `json:"content,omitempty" jsonschema:"contenido del archivo (alternativa a path)"`
		Role    string `json:"role" jsonschema:"rol contra el que validar: viewmodel, composable, activity, usecase, repository, dao, di_module, data_class, entity, nav_route"`
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "scaffold_validate",
		Description: "Valida un archivo Kotlin contra las reglas del rol especificado. Devuelve errores y warnings.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in validateArgs) (*mcp.CallToolResult, any, error) {
		role := scaffold.Role(in.Role)
		if !scaffold.IsValidRole(role) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Rol desconocido: %s", in.Role)}},
				IsError: true,
			}, nil, nil
		}

		content := in.Content
		if content == "" && in.Path != "" {
			data, err := readFileContent(in.Path)
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error leyendo archivo: %v", err)}},
					IsError: true,
				}, nil, nil
			}
			content = data
		}
		if content == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "Se requiere content o path"}},
				IsError: true,
			}, nil, nil
		}

		issues := scaffold.Validate(content, role)
		if len(issues) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "✓ Archivo válido. Todas las reglas pasan."}},
			}, nil, nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Validación para %s (%d issues):\n\n", in.Role, len(issues))
		for _, issue := range issues {
			icon := "⚠"
			if issue.Severity == "error" {
				icon = "✗"
			}
			fmt.Fprintf(&sb, "%s [%s] %s", icon, issue.Rule, issue.Message)
			if issue.Line > 0 {
				fmt.Fprintf(&sb, " (línea %d)", issue.Line)
			}
			sb.WriteString("\n")
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil, nil
	})
}
