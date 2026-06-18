package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pablofelipe1207/androideia/internal/task"
	"github.com/pablofelipe1207/androideia/internal/version"
)

func registerTaskTools(s *Server) {
	type listArgs struct {
		Status string `json:"status,omitempty" jsonschema:"filtrar por estado: pending, queued, processing, completed, failed, cancelled (vacío = todos)"`
		Limit  int    `json:"limit,omitempty" jsonschema:"cantidad máxima de resultados (default: 20)"`
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "task_list",
		Description: "Lista las tareas de la cola de tareas del proyecto.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listArgs) (*mcp.CallToolResult, any, error) {
		if in.Limit <= 0 {
			in.Limit = 20
		}
		tasks, err := s.tasks.List(in.Status, in.Limit)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error listando tareas: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		if len(tasks) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No hay tareas."}},
			}, nil, nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Tareas (%d):\n\n", len(tasks))
		for _, t := range tasks {
			priority := task.PriorityToString(t.Priority)
			fmt.Fprintf(&sb, "- [%d] **%s** — %s | %s | %s\n",
				t.ID, t.Title, priority, t.Status, t.Type)
			if t.Description != "" {
				fmt.Fprintf(&sb, "  %s\n", truncateStr(t.Description, 150))
			}
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil, nil
	})

	type createArgs struct {
		Title       string `json:"title" jsonschema:"título de la tarea"`
		Description string `json:"description,omitempty" jsonschema:"descripción detallada de la tarea"`
		Type        string `json:"type,omitempty" jsonschema:"tipo: feature, bugfix, refactor, test, review"`
		Priority    string `json:"priority,omitempty" jsonschema:"prioridad: low, medium, high, urgent"`
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "task_create",
		Description: "Crea una nueva tarea en la cola de tareas del proyecto.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createArgs) (*mcp.CallToolResult, any, error) {
		if in.Title == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "El título es requerido"}},
				IsError: true,
			}, nil, nil
		}
		priority := task.StringToPriority(in.Priority)
		t, err := s.tasks.Add(in.Title, in.Description, in.Type, priority)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error creando tarea: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Tarea creada con ID: %d\nTítulo: %s\nPrioridad: %s\nTipo: %s",
				t.ID, t.Title, task.PriorityToString(t.Priority), t.Type)}},
		}, nil, nil
	})

	type getArgs struct {
		ID int64 `json:"id" jsonschema:"ID de la tarea a consultar"`
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "task_get",
		Description: "Obtiene el detalle de una tarea específica por su ID.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in getArgs) (*mcp.CallToolResult, any, error) {
		t, err := s.tasks.Get(in.ID)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error obteniendo tarea: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Tarea #%d\n", t.ID)
		fmt.Fprintf(&sb, "Título: %s\n", t.Title)
		fmt.Fprintf(&sb, "Descripción: %s\n", t.Description)
		fmt.Fprintf(&sb, "Tipo: %s\n", t.Type)
		fmt.Fprintf(&sb, "Prioridad: %s\n", task.PriorityToString(t.Priority))
		fmt.Fprintf(&sb, "Estado: %s\n", t.Status)
		if t.Result != "" {
			fmt.Fprintf(&sb, "Resultado: %s\n", t.Result)
		}
		if t.Error != "" {
			fmt.Fprintf(&sb, "Error: %s\n", t.Error)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil, nil
	})

	s.mcp.AddTool(&mcp.Tool{
		Name:         "task_stats",
		Description:  "Devuelve estadísticas de la cola de tareas (totales por estado).",
		InputSchema:  json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		stats, err := s.tasks.GetQueueStats()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			}, nil
		}
		var sb strings.Builder
		sb.WriteString("Estadísticas de la cola de tareas:\n\n")
		total := 0
		for status, count := range stats {
			fmt.Fprintf(&sb, "- %s: %d\n", status, count)
			total += count
		}
		fmt.Fprintf(&sb, "\nTotal: %d", total)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil
	})
}

func registerProjectTools(s *Server) {
	s.mcp.AddTool(&mcp.Tool{
		Name:         "project_info",
		Description:  "Devuelve información general del proyecto androideia: versión, tablas en la DB, estadísticas del índice.",
		InputSchema:  json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var sb strings.Builder
		fmt.Fprintf(&sb, "=== androideia ===\n")
		fmt.Fprintf(&sb, "Versión: %s\n\n", version.Version)

		// Stats from DB
		counts := map[string]int{}
		tables := []string{"files", "symbols", "knowledge_entries", "embeddings", "file_semantics", "tasks", "conversations"}
		for _, table := range tables {
			var count int
			err := s.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
			if err == nil {
				counts[table] = count
			}
		}

		fmt.Fprintf(&sb, "Estadísticas de la base de datos:\n")
		for _, table := range tables {
			if c, ok := counts[table]; ok {
				fmt.Fprintf(&sb, "  - %s: %d registros\n", table, c)
			}
		}

		// Roles
		fmt.Fprintf(&sb, "\nPlantillas disponibles: %d roles\n", len(scaffoldAllRoles()))

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil
	})
}

func readFileContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
