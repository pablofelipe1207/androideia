package task

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/pablofelipe1207/androideia/internal/agent"
	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/pablofelipe1207/androideia/internal/llm"
	"github.com/pablofelipe1207/androideia/internal/memory"
)

// TaskQueue gestiona la cola de procesamiento de tareas.
type TaskQueue struct {
	manager *TaskManager
	llm     llm.Provider
	config  *config.Config
	db      *sql.DB
}

// NewTaskQueue crea una nueva cola de tareas.
func NewTaskQueue(db *sql.DB, llmProvider llm.Provider, cfg *config.Config) *TaskQueue {
	return &TaskQueue{
		manager: NewTaskManager(db),
		llm:     llmProvider,
		config:  cfg,
		db:      db,
	}
}

// ProcessNext procesa la siguiente tarea en la cola.
func (q *TaskQueue) ProcessNext() (*Task, error) {
	// Obtener siguiente tarea
	t, err := q.manager.GetNextToProcess()
	if err != nil {
		return nil, err
	}

	// Marcar como procesando
	if err := q.manager.Update(t.ID, map[string]interface{}{
		"status": StatusProcessing,
	}); err != nil {
		return nil, fmt.Errorf("error marking task as processing: %w", err)
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	fmt.Printf("  PROCESANDO TAREA #%d\n", t.ID)
	fmt.Printf("  Título: %s\n", t.Title)
	fmt.Printf("  Prioridad: %s\n", PriorityToString(t.Priority))
	fmt.Printf("  Tipo: %s\n", t.Type)
	fmt.Printf("%s\n\n", strings.Repeat("=", 60))

	// Si hay LLM disponible, procesar con el agente
	if q.llm != nil && q.llm.IsAvailable() {
		result, err := q.processWithAgent(t)
		if err != nil {
			_ = q.manager.Update(t.ID, map[string]interface{}{
				"status": StatusFailed,
				"error":  err.Error(),
			})
			return t, fmt.Errorf("error processing task: %w", err)
		}

		// Marcar como completada
		if err := q.manager.Update(t.ID, map[string]interface{}{
			"status": StatusCompleted,
			"result": result,
		}); err != nil {
			return t, fmt.Errorf("error marking task as completed: %w", err)
		}

		t.Status = StatusCompleted
		t.Result = result
	} else {
		// Sin LLM, solo marcar como completada con mensaje
		if err := q.manager.Update(t.ID, map[string]interface{}{
			"status": StatusCompleted,
			"result": "Tarea procesada (sin LLM - modo offline)",
		}); err != nil {
			return t, fmt.Errorf("error marking task as completed: %w", err)
		}

		t.Status = StatusCompleted
		t.Result = "Tarea procesada (sin LLM - modo offline)"
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	fmt.Printf("  TAREA #%d COMPLETADA\n", t.ID)
	fmt.Printf("%s\n", strings.Repeat("=", 60))

	return t, nil
}

// ProcessAll procesa todas las tareas en la cola.
func (q *TaskQueue) ProcessAll() ([]*Task, error) {
	var processed []*Task

	for {
		t, err := q.ProcessNext()
		if err != nil {
			if err.Error() == "no tasks in queue" {
				break
			}
			return processed, err
		}
		processed = append(processed, t)
	}

	return processed, nil
}

// processWithAgent procesa una tarea usando el agente LLM.
func (q *TaskQueue) processWithAgent(t *Task) (string, error) {
	if q.llm == nil || !q.llm.IsAvailable() {
		return "", fmt.Errorf("LLM not available")
	}

	// Crear agente
	ag := agent.NewAgent(q.llm, q.db, q.config)

	// Conectar memoria si está disponible
	if q.db != nil {
		mem := memory.NewMemory(q.db)
		ag.SetMemory(mem)
	}

	// Ejecutar tarea
	taskPrompt := buildTaskPrompt(t)
	if err := ag.Run(taskPrompt); err != nil {
		return "", err
	}

	// Obtener resultado del historial
	history := ag.GetConversationHistory()
	if len(history) > 0 {
		// Buscar último mensaje del assistant
		for i := len(history) - 1; i >= 0; i-- {
			if history[i].Role == "assistant" && history[i].Content != "" {
				return history[i].Content, nil
			}
		}
	}

	return "Tarea procesada exitosamente", nil
}

// buildTaskPrompt construye el prompt para procesar una tarea.
func buildTaskPrompt(t *Task) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Tarea: %s\n\n", t.Title))

	if t.Description != "" {
		sb.WriteString(fmt.Sprintf("Descripción: %s\n\n", t.Description))
	}

	sb.WriteString(fmt.Sprintf("Tipo: %s\n", t.Type))
	sb.WriteString(fmt.Sprintf("Prioridad: %s\n\n", PriorityToString(t.Priority)))

	sb.WriteString("Por favor, procesa esta tarea y proporciona un resultado.")

	return sb.String()
}

// GetManager retorna el TaskManager.
func (q *TaskQueue) GetManager() *TaskManager {
	return q.manager
}

// PrintTask imprime una tarea formateada.
func PrintTask(t *Task) {
	fmt.Printf("  %-6d %-10s %-8s %-20s %s\n",
		t.ID,
		PriorityToString(t.Priority),
		t.Status,
		t.Type,
		truncate(t.Title, 40),
	)
}

// truncate trunca un string a maxLen caracteres.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
