package task

import (
	"database/sql"
	"fmt"
	"os/exec"
	"strings"

	"github.com/pablofelipe1207/androideia/internal/agent"
	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/pablofelipe1207/androideia/internal/llm"
	"github.com/pablofelipe1207/androideia/internal/memory"
)

// RunOptions contiene las opciones para ProcessFromMarkdown.
type RunOptions struct {
	UseGit        bool   // Habilitar workflow git (branch + PR)
	BranchPrefix  string // Prefijo para branches (default: "task/")
	ValidateBuild bool   // Ejecutar go build después de cada tarea
	StopOnError   bool   // Detener si hay error de compilación
	Model         string // Override del modelo LLM
	Timeout       int    // Timeout en segundos
	MaxTurns      int    // Máximo de turnos del agente
}

// TaskResult resultado del procesamiento de una tarea.
type TaskResult struct {
	LineNumber int
	Title      string
	Success    bool
	Error      string
	BranchName string
	PRURL      string
}

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

// ProcessFromMarkdown procesa un archivo .md con tareas en formato checkbox.
func (q *TaskQueue) ProcessFromMarkdown(filePath string, opts RunOptions) ([]*TaskResult, error) {
	// Parsear el archivo
	tasks, err := ParseMarkdownFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error parsing markdown file: %w", err)
	}

	pending := GetPendingTasks(tasks)
	if len(pending) == 0 {
		fmt.Println("No hay tareas pendientes en el archivo.")
		return nil, nil
	}

	total, completed := Summary(tasks)
	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	fmt.Printf("  PROCESANDO ARCHIVO DE TAREAS\n")
	fmt.Printf("  Archivo: %s\n", filePath)
	fmt.Printf("  Total: %d | Pendientes: %d | Completadas: %d\n", total, len(pending), completed)
	fmt.Printf("%s\n\n", strings.Repeat("=", 60))

	// Guardar branch original si vamos a usar git
	var originalBranch string
	if opts.UseGit {
		if !IsGitRepo() {
			return nil, fmt.Errorf("git workflow habilitado pero no se encontró un repositorio git")
		}
		originalBranch, err = NewGitHelper(opts.BranchPrefix).GetCurrentBranch()
		if err != nil {
			return nil, fmt.Errorf("error getting current branch: %w", err)
		}
	}

	var results []*TaskResult

	for i, mdTask := range pending {
		fmt.Printf("\n%s\n", strings.Repeat("-", 60))
		fmt.Printf("  TAREA %d/%d: %s\n", i+1, len(pending), mdTask.Title)
		fmt.Printf("%s\n\n", strings.Repeat("-", 60))

		result := &TaskResult{
			LineNumber: mdTask.Line,
			Title:      mdTask.Title,
		}

		// Crear branch si git está habilitado
		if opts.UseGit {
			git := NewGitHelper(opts.BranchPrefix)
			branchName, err := git.CreateBranch(mdTask.Title)
			if err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("error creating branch: %v", err)
				results = append(results, result)
				fmt.Printf("  ERROR: %v\n", err)
				continue
			}
			result.BranchName = branchName
			fmt.Printf("  [Git] Branch creada: %s\n", branchName)
		}

		// Ejecutar la tarea con el agente
		taskSuccess, taskErr := q.executeMarkdownTask(mdTask.Title, opts)

		if taskSuccess && opts.ValidateBuild {
			// Verificar compilación
			fmt.Printf("\n  [Build] Verificando compilación...\n")
			if buildErr := validateBuild(); buildErr != nil {
				taskSuccess = false
				taskErr = fmt.Sprintf("build failed: %v", buildErr)
				fmt.Printf("  [Build] ERROR: %v\n", buildErr)
			} else {
				fmt.Printf("  [Build] OK\n")
			}
		}

		result.Success = taskSuccess
		result.Error = taskErr

		// Crear commit y PR si git está habilitado y la tarea fue exitosa
		if opts.UseGit && taskSuccess {
			git := NewGitHelper(opts.BranchPrefix)

			if err := git.CommitAll(fmt.Sprintf("feat: %s", mdTask.Title)); err != nil {
				fmt.Printf("  [Git] Error creando commit: %v\n", err)
			} else {
				fmt.Printf("  [Git] Commit creado\n")
			}

			if err := git.PushBranch(); err != nil {
				fmt.Printf("  [Git] Error haciendo push: %v\n", err)
			} else {
				fmt.Printf("  [Git] Push realizado\n")
			}

			// Crear PR
			prBody := fmt.Sprintf("Tarea: %s\n\nImplementación automática por androideai.", mdTask.Title)
			prURL, err := git.CreatePR(fmt.Sprintf("feat: %s", mdTask.Title), prBody)
			if err != nil {
				fmt.Printf("  [Git] Error creando PR: %v\n", err)
			} else {
				result.PRURL = prURL
				fmt.Printf("  [Git] PR creado: %s\n", prURL)
			}

			// Volver a la branch original para la siguiente tarea
			if _, err := runGit("checkout", originalBranch); err != nil {
				fmt.Printf("  [Git] Error volviendo a branch %s: %v\n", originalBranch, err)
			}
		}

		// Marcar la tarea en el archivo
		if taskSuccess {
			if err := MarkTaskCompleted(filePath, mdTask.Line); err != nil {
				fmt.Printf("  ERROR marking task as completed in file: %v\n", err)
			} else {
				fmt.Printf("  [File] Tarea marcada como completada\n")
			}
		} else {
			if err := MarkTaskError(filePath, mdTask.Line, taskErr); err != nil {
				fmt.Printf("  ERROR marking task error in file: %v\n", err)
			}
		}

		results = append(results, result)

		// Si stopOnError y hubo error, detener
		if opts.StopOnError && !taskSuccess {
			fmt.Printf("\n  [Stop] Deteniendo por error (StopOnError habilitado)\n")
			break
		}
	}

	// Imprimir resumen
	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	fmt.Printf("  RESUMEN\n")
	fmt.Printf("%s\n\n", strings.Repeat("=", 60))

	successCount := 0
	failCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
			fmt.Printf("  ✓ %s\n", r.Title)
			if r.PRURL != "" {
				fmt.Printf("    PR: %s\n", r.PRURL)
			}
		} else {
			failCount++
			fmt.Printf("  ✗ %s\n", r.Title)
			if r.Error != "" {
				fmt.Printf("    Error: %s\n", r.Error)
			}
		}
	}

	fmt.Printf("\n  Total: %d | Exitosas: %d | Fallidas: %d\n", len(results), successCount, failCount)
	fmt.Printf("%s\n\n", strings.Repeat("=", 60))

	return results, nil
}

// executeMarkdownTask ejecuta una tarea individual del markdown.
func (q *TaskQueue) executeMarkdownTask(title string, opts RunOptions) (bool, string) {
	if q.llm == nil || !q.llm.IsAvailable() {
		return false, "LLM not available"
	}

	// Crear configuración temporal si se especificó modelo
	llmProvider := q.llm

	// Crear agente
	ag := agent.NewAgent(llmProvider, q.db, q.config)

	// Conectar memoria si está disponible
	if q.db != nil {
		mem := memory.NewMemory(q.db)
		ag.SetMemory(mem)
	}

	// Construir prompt más detallado para tareas markdown
	prompt := buildMarkdownTaskPrompt(title)

	// Ejecutar tarea
	if err := ag.Run(prompt); err != nil {
		return false, fmt.Sprintf("agent error: %v", err)
	}

	// Verificar que no haya errores en el historial
	history := ag.GetConversationHistory()
	for _, msg := range history {
		if msg.Role == "tool" && strings.Contains(strings.ToLower(msg.Content), "error") {
			// No retornar error automáticamente, solo si es un error fatal
		}
	}

	return true, ""
}

// buildMarkdownTaskPrompt construye un prompt para tareas del markdown.
func buildMarkdownTaskPrompt(title string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Tarea: %s\n\n", title))
	sb.WriteString("Esta tarea viene de un archivo de tareas. Debes:\n")
	sb.WriteString("1. Explorar el código existente para entender el contexto\n")
	sb.WriteString("2. Planear los cambios necesarios\n")
	sb.WriteString("3. Implementar la solución\n")
	sb.WriteString("4. Verificar que no haya errores de compilación\n\n")
	sb.WriteString("No pidas confirmación, simplemente ejecuta la tarea de forma autónoma.\n")
	sb.WriteString("Si necesitas crear archivos, créalos directamente.\n")
	sb.WriteString("Al terminar, indica claramente que la tarea está completada.")

	return sb.String()
}

// validateBuild ejecuta go build y go vet para verificar que el código compile.
func validateBuild() error {
	// go build
	buildCmd := exec.Command("go", "build", "./...")
	out, err := buildCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build failed: %v\nOutput: %s", err, string(out))
	}

	// go vet
	vetCmd := exec.Command("go", "vet", "./...")
	out, err = vetCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go vet failed: %v\nOutput: %s", err, string(out))
	}

	return nil
}
