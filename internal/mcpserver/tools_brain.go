package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pablofelipe1207/androideia/internal/brain"
)

func registerBrainTools(s *Server) {
	type searchArgs struct {
		Query string `json:"query" jsonschema:"texto de búsqueda para encontrar entradas de conocimiento"`
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "brain_search",
		Description: "Busca en la memoria del proyecto (cerebro) por texto. Encuentra decisiones, patrones, convenciones y conocimiento guardado.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in searchArgs) (*mcp.CallToolResult, any, error) {
		entries, err := s.brain.Search(in.Query)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error buscando: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		if len(entries) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No se encontraron entradas para: " + in.Query}},
			}, nil, nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Encontradas %d entradas de conocimiento:\n\n", len(entries))
		for _, e := range entries {
			fmt.Fprintf(&sb, "## %s (ID: %d)\n", e.Title, e.ID)
			fmt.Fprintf(&sb, "- Tipo: %s\n", e.Type)
			fmt.Fprintf(&sb, "- Estado: %s\n", e.Status)
			if e.Tags != "" {
				fmt.Fprintf(&sb, "- Tags: %s\n", e.Tags)
			}
			fmt.Fprintf(&sb, "- Contenido: %s\n\n", truncateStr(e.Content, 300))
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil, nil
	})

	type saveArgs struct {
		Type    string `json:"type" jsonschema:"tipo de entrada: decision, pattern, workaround, gotcha, convention, rule"`
		Title   string `json:"title" jsonschema:"título de la entrada"`
		Content string `json:"content" jsonschema:"contenido de la entrada"`
		Tags    string `json:"tags,omitempty" jsonschema:"tags separados por coma"`
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "brain_save",
		Description: "Guarda una entrada de conocimiento en la memoria del proyecto (cerebro). Se guarda como temporal hasta ser promovida.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in saveArgs) (*mcp.CallToolResult, any, error) {
		if in.Type == "" {
			in.Type = "decision"
		}
		entry := &brain.KnowledgeEntry{
			Type:    in.Type,
			Title:   in.Title,
			Content: in.Content,
			Tags:    in.Tags,
			Status:  "temp",
		}
		id, err := s.brain.Save(entry, false)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error guardando: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Entrada guardada con ID: %d (estado: temp). Usá brain_promote con id=%d para promoverla.", id, id)}},
		}, nil, nil
	})

	s.mcp.AddTool(&mcp.Tool{
		Name:         "brain_list",
		Description:  "Lista todas las entradas de conocimiento del cerebro del proyecto.",
		InputSchema:  json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		entries, err := s.brain.List()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error listando: %v", err)}},
				IsError: true,
			}, nil
		}
		if len(entries) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "El cerebro está vacío. Usá brain_save para agregar conocimiento."}},
			}, nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Cerebro del proyecto (%d entradas):\n\n", len(entries))
		for _, e := range entries {
			status := e.Status
			if status == "temp" {
				status = "⏳ temp"
			} else {
				status = "✓ promoted"
			}
			fmt.Fprintf(&sb, "- [%d] **%s** (%s) — %s\n", e.ID, e.Title, e.Type, status)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil
	})

	s.mcp.AddTool(&mcp.Tool{
		Name:         "brain_review",
		Description:  "Lista las entradas de conocimiento en estado temporal que esperan revisión.",
		InputSchema:  json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		entries, err := s.brain.Review()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			}, nil
		}
		if len(entries) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No hay entradas temporales pendientes de revisión."}},
			}, nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Entradas temporales (%d):\n\n", len(entries))
		for _, e := range entries {
			fmt.Fprintf(&sb, "- [%d] **%s** (%s)\n  %s\n\n", e.ID, e.Title, e.Type, truncateStr(e.Content, 200))
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil
	})

	type promoteArgs struct {
		ID int64 `json:"id" jsonschema:"ID de la entrada a promover"`
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "brain_promote",
		Description: "Promueve una entrada de conocimiento de 'temp' a 'promoted'. Las entradas promovidas son conocimiento permanente del proyecto.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in promoteArgs) (*mcp.CallToolResult, any, error) {
		if err := s.brain.Promote(in.ID); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error promoviendo: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Entrada %d promovida a 'promoted' exitosamente.", in.ID)}},
		}, nil, nil
	})
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
