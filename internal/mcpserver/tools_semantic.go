package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerSemanticTools(s *Server) {
	type searchArgs struct {
		Query string `json:"query" jsonschema:"texto de búsqueda semántica (significado, no solo palabras exactas)"`
		Limit int    `json:"limit,omitempty" jsonschema:"cantidad máxima de resultados (default: 10)"`
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "semantic_search",
		Description: "Busca en el código por significado usando embeddings. Encuentra símbolos relacionados con un concepto, no solo coincidencias literales.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in searchArgs) (*mcp.CallToolResult, any, error) {
		if in.Limit <= 0 {
			in.Limit = 10
		}
		results, err := s.semantic.Search(in.Query, in.Limit)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error en búsqueda: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		if len(results) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No se encontraron resultados para: " + in.Query}},
			}, nil, nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Encontrados %d resultados para \"%s\":\n\n", len(results), in.Query)
		for i, r := range results {
			fmt.Fprintf(&sb, "%d. **%s** (%s)\n   Archivo: %s:%d\n   Similitud: %.2f\n\n",
				i+1, r.Name, r.Kind, r.Path, r.Line, r.Similarity)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil, nil
	})

	type locateArgs struct {
		Type string `json:"type,omitempty" jsonschema:"tipo de archivo (viewmodel, composable, usecase, repository, dao, activity, di_module, data_class, entity, nav_route)"`
		Tag  string `json:"tag,omitempty" jsonschema:"tag a buscar en la clasificación semántica"`
		Name string `json:"name,omitempty" jsonschema:"nombre parcial del archivo a buscar"`
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "semantic_locate",
		Description: "Localiza archivos por tipo, tag o nombre usando el índice semántico (clasificación LLM de archivos).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in locateArgs) (*mcp.CallToolResult, any, error) {
		if in.Type == "" && in.Tag == "" && in.Name == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "Debe especificar al menos un criterio: type, tag o name"}},
				IsError: true,
			}, nil, nil
		}

		query := `SELECT f.path, fs.type, fs.tags, fs.architecture, fs.summary
			FROM file_semantics fs
			JOIN files f ON f.id = fs.file_id
			WHERE 1=1`
		var args []interface{}

		if in.Type != "" {
			query += ` AND fs.type = ?`
			args = append(args, in.Type)
		}
		if in.Tag != "" {
			query += ` AND fs.tags LIKE ?`
			args = append(args, "%"+in.Tag+"%")
		}
		if in.Name != "" {
			query += ` AND f.path LIKE ?`
			args = append(args, "%"+in.Name+"%")
		}
		query += ` ORDER BY f.path`

		rows, err := s.db.Query(query, args...)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error en consulta: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		defer rows.Close()

		var sb strings.Builder
		count := 0
		for rows.Next() {
			var path, fileType, tags, architecture, summary string
			if err := rows.Scan(&path, &fileType, &tags, &architecture, &summary); err != nil {
				continue
			}
			count++
			fmt.Fprintf(&sb, "%d. **%s**\n   Tipo: %s\n   Tags: %s\n   Arquitectura: %s\n   Resumen: %s\n\n",
				count, path, fileType, tags, architecture, summary)
		}
		if count == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No se encontraron archivos con esos criterios"}},
			}, nil, nil
		}
		fmt.Fprintf(&sb, "Total: %d archivos", count)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil, nil
	})

	type graphArgs struct {
		Feature string `json:"feature,omitempty" jsonschema:"nombre de feature específica (opcional, si se omite muestra todas)"`
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "semantic_graph",
		Description: "Devuelve el feature graph: archivos agrupados por feature. Útil para entender la estructura del proyecto.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in graphArgs) (*mcp.CallToolResult, any, error) {
		fileToFeature, err := s.semantic.DiscoverFeatures()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error descubriendo features: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		if len(fileToFeature) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No se encontraron features. Ejecutá 'androideai semantic index' primero."}},
			}, nil, nil
		}

		features := make(map[string][]string)
		for path, feature := range fileToFeature {
			if in.Feature != "" && strings.ToLower(feature) != strings.ToLower(in.Feature) {
				continue
			}
			features[feature] = append(features[feature], path)
		}

		if len(features) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("No se encontró la feature '%s'", in.Feature)}},
			}, nil, nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Feature Graph (%d features):\n\n", len(features))
		for feature, files := range features {
			fmt.Fprintf(&sb, "## %s (%d archivos)\n", feature, len(files))
			for _, f := range files {
				fmt.Fprintf(&sb, "  - %s\n", f)
			}
			sb.WriteString("\n")
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil, nil
	})

	type depsArgs struct {
		Path string `json:"path" jsonschema:"ruta del archivo a analizar"`
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "semantic_deps",
		Description: "Muestra las dependencias (imports) de un archivo específico y qué otros archivos lo importan.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in depsArgs) (*mcp.CallToolResult, any, error) {
		rows, err := s.db.Query(`
			SELECT s.name, s.kind, s.signature
			FROM symbols s
			JOIN files f ON s.file_id = f.id
			WHERE f.path LIKE ?
			ORDER BY s.kind, s.name`, "%"+in.Path+"%")
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		defer rows.Close()

		var sb strings.Builder
		fmt.Fprintf(&sb, "Símbolos en %s:\n\n", in.Path)
		count := 0
		for rows.Next() {
			var name, kind, signature string
			if err := rows.Scan(&name, &kind, &signature); err != nil {
				continue
			}
			count++
			fmt.Fprintf(&sb, "- **%s** (%s) %s\n", name, kind, signature)
		}
		if count == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No se encontraron símbolos para: " + in.Path}},
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil, nil
	})

	type suggestArgs struct {
		Feature string `json:"feature" jsonschema:"nombre de la feature a analizar"`
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "semantic_suggest",
		Description: "Sugiere capas faltantes para una feature. Analiza qué archivos existen y qué capas de la arquitectura MVVM están ausentes.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in suggestArgs) (*mcp.CallToolResult, any, error) {
		rows, err := s.db.Query(`
			SELECT fs.type, f.path
			FROM file_semantics fs
			JOIN files f ON f.id = fs.file_id
			WHERE LOWER(f.path) LIKE '%' || LOWER(?) || '%'
			ORDER BY fs.type`, in.Feature)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		defer rows.Close()

		existing := make(map[string][]string)
		for rows.Next() {
			var fileType, path string
			if err := rows.Scan(&fileType, &path); err != nil {
				continue
			}
			existing[fileType] = append(existing[fileType], path)
		}

		allLayers := []string{"viewmodel", "composable", "usecase", "repository", "dao", "di_module", "activity"}
		var missing []string
		for _, layer := range allLayers {
			if _, ok := existing[layer]; !ok {
				missing = append(missing, layer)
			}
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Feature: %s\n\n", in.Feature)
		fmt.Fprintf(&sb, "Capas existentes (%d):\n", len(existing))
		for layer, files := range existing {
			fmt.Fprintf(&sb, "  ✓ %s (%d archivos)\n", layer, len(files))
		}
		if len(missing) > 0 {
			fmt.Fprintf(&sb, "\nCapas faltantes (%d):\n", len(missing))
			for _, layer := range missing {
				fmt.Fprintf(&sb, "  ✗ %s\n", layer)
			}
		} else {
			fmt.Fprintf(&sb, "\nTodas las capas están presentes.")
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil, nil
	})

	type indexArgs struct {
		Force bool `json:"force,omitempty" jsonschema:"re-indexar todos los archivos aunque ya estén indexados"`
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "semantic_index",
		Description: "Indexa el proyecto: clasifica archivos con LLM y genera embeddings. Requerido antes de usar search/locate/graph.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in indexArgs) (*mcp.CallToolResult, any, error) {
		count, err := s.semantic.IndexAll()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error indexando: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Indexación completada: %d símbolos indexados", count)}},
		}, nil, nil
	})
}
