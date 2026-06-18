package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/pablofelipe1207/androideia/internal/llm"
	"github.com/pablofelipe1207/androideia/internal/store"
	"github.com/pablofelipe1207/androideia/internal/task"
	"github.com/spf13/cobra"
)

var (
	taskPriority    string
	taskType        string
	taskStatus      string
	taskModel       string
	taskTimeout     int
	taskAutoApprove bool
	// Flags para task run
	taskRunFile       string
	taskRunGit        bool
	taskRunBranchPref string
	taskRunValidate   bool
	taskRunStopError  bool
	taskRunMaxTurns   int
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Gestiona la cola de tareas del agente",
	Long:  `Administra tareas que el agente debe procesar. Permite agregar, listar, actualizar y procesar tareas.`,
}

var taskAddCmd = &cobra.Command{
	Use:   "add [title]",
	Short: "Agrega una nueva tarea a la cola",
	Long:  `Agrega una tarea nueva con título, descripción, tipo y prioridad.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]
		description, _ := cmd.Flags().GetString("description")
		priority := task.StringToPriority(taskPriority)
		taskTypeVal := taskType
		if taskTypeVal == "" {
			taskTypeVal = task.TypeFeature
		}

		tm, closer, err := openTaskManager()
		if err != nil {
			return err
		}
		defer closer()

		t, err := tm.Add(title, description, taskTypeVal, priority)
		if err != nil {
			return err
		}

		fmt.Printf("Tarea #%d creada: %s\n", t.ID, t.Title)
		fmt.Printf("  Prioridad: %s\n", task.PriorityToString(t.Priority))
		fmt.Printf("  Tipo: %s\n", t.Type)
		fmt.Printf("  Estado: %s\n", t.Status)
		return nil
	},
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista las tareas",
	RunE: func(cmd *cobra.Command, args []string) error {
		tm, closer, err := openTaskManager()
		if err != nil {
			return err
		}
		defer closer()

		tasks, err := tm.List(taskStatus, 50)
		if err != nil {
			return err
		}

		if len(tasks) == 0 {
			fmt.Println("No hay tareas.")
			return nil
		}

		fmt.Printf("Encontradas %d tareas:\n\n", len(tasks))
		fmt.Println("  ID     PRIORIDAD  ESTADO   TIPO                TÍTULO")
		fmt.Println(strings.Repeat("─", 70))

		for _, t := range tasks {
			task.PrintTask(t)
		}
		return nil
	},
}

var taskShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Muestra el detalle de una tarea",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("ID inválido: %s", args[0])
		}

		tm, closer, err := openTaskManager()
		if err != nil {
			return err
		}
		defer closer()

		t, err := tm.Get(id)
		if err != nil {
			return err
		}

		fmt.Printf("Tarea #%d\n", t.ID)
		fmt.Printf("  Título:       %s\n", t.Title)
		fmt.Printf("  Descripción:  %s\n", t.Description)
		fmt.Printf("  Prioridad:    %s (%d)\n", task.PriorityToString(t.Priority), t.Priority)
		fmt.Printf("  Estado:       %s\n", t.Status)
		fmt.Printf("  Tipo:         %s\n", t.Type)
		if t.Result != "" {
			fmt.Printf("  Resultado:    %s\n", t.Result)
		}
		if t.Error != "" {
			fmt.Printf("  Error:        %s\n", t.Error)
		}
		if t.ConversationID > 0 {
			fmt.Printf("  Conversación: %d\n", t.ConversationID)
		}
		fmt.Printf("  Creada:       %s\n", time.Unix(t.CreatedAt, 0).Format("2006-01-02 15:04:05"))
		fmt.Printf("  Actualizada:  %s\n", time.Unix(t.UpdatedAt, 0).Format("2006-01-02 15:04:05"))
		if t.StartedAt > 0 {
			fmt.Printf("  Iniciada:     %s\n", time.Unix(t.StartedAt, 0).Format("2006-01-02 15:04:05"))
		}
		if t.CompletedAt > 0 {
			fmt.Printf("  Completada:   %s\n", time.Unix(t.CompletedAt, 0).Format("2006-01-02 15:04:05"))
		}
		return nil
	},
}

var taskUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "Actualiza una tarea",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("ID inválido: %s", args[0])
		}

		tm, closer, err := openTaskManager()
		if err != nil {
			return err
		}
		defer closer()

		updates := make(map[string]interface{})

		if title, _ := cmd.Flags().GetString("title"); title != "" {
			updates["title"] = title
		}
		if desc, _ := cmd.Flags().GetString("description"); desc != "" {
			updates["description"] = desc
		}
		if p := taskPriority; p != "" {
			updates["priority"] = task.StringToPriority(p)
		}
		if s := taskStatus; s != "" {
			updates["status"] = s
		}
		if t := taskType; t != "" {
			updates["type"] = t
		}

		if len(updates) == 0 {
			return fmt.Errorf("no hay campos para actualizar")
		}

		if err := tm.Update(id, updates); err != nil {
			return err
		}

		fmt.Printf("Tarea #%d actualizada.\n", id)
		return nil
	},
}

var taskDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Elimina una tarea",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("ID inválido: %s", args[0])
		}

		tm, closer, err := openTaskManager()
		if err != nil {
			return err
		}
		defer closer()

		if err := tm.Delete(id); err != nil {
			return err
		}

		fmt.Printf("Tarea #%d eliminada.\n", id)
		return nil
	},
}

var taskCancelCmd = &cobra.Command{
	Use:   "cancel [id]",
	Short: "Cancela una tarea",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("ID inválido: %s", args[0])
		}

		tm, closer, err := openTaskManager()
		if err != nil {
			return err
		}
		defer closer()

		if err := tm.Cancel(id); err != nil {
			return err
		}

		fmt.Printf("Tarea #%d cancelada.\n", id)
		return nil
	},
}

var taskQueueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Muestra las tareas en cola",
	RunE: func(cmd *cobra.Command, args []string) error {
		tm, closer, err := openTaskManager()
		if err != nil {
			return err
		}
		defer closer()

		tasks, err := tm.List(task.StatusQueued, 50)
		if err != nil {
			return err
		}

		if len(tasks) == 0 {
			fmt.Println("No hay tareas en cola.")
			return nil
		}

		fmt.Printf("Tareas en cola: %d\n\n", len(tasks))
		fmt.Println("  ID     PRIORIDAD  ESTADO   TIPO                TÍTULO")
		fmt.Println(strings.Repeat("─", 70))

		for _, t := range tasks {
			task.PrintTask(t)
		}
		return nil
	},
}

var taskProcessCmd = &cobra.Command{
	Use:   "process",
	Short: "Procesa la siguiente tarea de la cola",
	RunE: func(cmd *cobra.Command, args []string) error {
		processAll, _ := cmd.Flags().GetBool("all")

		// Cargar configuración
		mc, _, err := config.LoadModelsConfig()
		if err != nil {
			return fmt.Errorf("error loading models config: %w", err)
		}

		if taskModel != "" {
			mc.Agent.Model = taskModel
		}

		// Resolver modelo
		if mc.Agent.Provider == "ollama" {
			baseURL := mc.Agent.BaseURL
			if baseURL == "" {
				baseURL = mc.Semantic.BaseURL
			}
			if baseURL == "" {
				baseURL = "http://localhost:11434"
			}
			resolved, _, err := llm.ResolveOllamaModel(baseURL, mc.Agent.Model)
			if err != nil {
				return fmt.Errorf("error resolving model: %w", err)
			}
			mc.Agent.Model = resolved
		}

		// Crear LLM provider
		var llmProvider llm.Provider
		timeoutDur := time.Duration(taskTimeout) * time.Second
		if timeoutDur <= 0 {
			timeoutDur = 120 * time.Second
		}

		switch mc.Agent.Provider {
		case "ollama":
			baseURL := mc.Agent.BaseURL
			if baseURL == "" {
				baseURL = mc.Semantic.BaseURL
			}
			if baseURL == "" {
				baseURL = "http://localhost:11434"
			}
			llmProvider = llm.NewOllamaProviderWithTimeout(baseURL, mc.Agent.Model, timeoutDur)
		case "opencode_zen":
			llmProvider = llm.NewOpenCodeZenProviderWithOptions(
				mc.Agent.Model,
				mc.Agent.APIKey(),
				mc.Agent.BaseURL,
				timeoutDur,
			)
		case "anthropic":
			apiKey := mc.Agent.APIKey()
			llmProvider = llm.NewAnthropicProvider(apiKey, mc.Agent.Model)
		case "openai":
			apiKey := mc.Agent.APIKey()
			llmProvider = llm.NewOpenAIProvider(apiKey, mc.Agent.Model, mc.Agent.BaseURL)
		default:
			return fmt.Errorf("unknown provider: %s", mc.Agent.Provider)
		}

		// Abrir DB
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Cargar configuración del agente
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %w", err)
		}
		if taskAutoApprove {
			cfg.Approval = "auto"
		}

		// Crear cola
		queue := task.NewTaskQueue(s.DB(), llmProvider, cfg)

		if processAll {
			processed, err := queue.ProcessAll()
			if err != nil {
				return err
			}
			fmt.Printf("\nTotal procesadas: %d\n", len(processed))
		} else {
			if _, err := queue.ProcessNext(); err != nil {
				return err
			}
		}

		return nil
	},
}

var taskStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Muestra estadísticas de la cola",
	RunE: func(cmd *cobra.Command, args []string) error {
		tm, closer, err := openTaskManager()
		if err != nil {
			return err
		}
		defer closer()

		stats, err := tm.GetQueueStats()
		if err != nil {
			return err
		}

		fmt.Println("Estadísticas de tareas:")
		fmt.Println(strings.Repeat("─", 40))

		total := 0
		for _, count := range stats {
			total += count
		}

		fmt.Printf("  Total:    %d\n", total)
		fmt.Printf("  Pendientes: %d\n", stats[task.StatusPending])
		fmt.Printf("  En cola:    %d\n", stats[task.StatusQueued])
		fmt.Printf("  Procesando: %d\n", stats[task.StatusProcessing])
		fmt.Printf("  Completadas: %d\n", stats[task.StatusCompleted])
		fmt.Printf("  Fallidas:   %d\n", stats[task.StatusFailed])
		fmt.Printf("  Canceladas: %d\n", stats[task.StatusCancelled])
		return nil
	},
}

var taskClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Limpia tareas completadas o canceladas",
	RunE: func(cmd *cobra.Command, args []string) error {
		tm, closer, err := openTaskManager()
		if err != nil {
			return err
		}
		defer closer()

		clearType, _ := cmd.Flags().GetString("type")

		var count int64
		switch clearType {
		case "completed":
			count, err = tm.ClearCompleted()
		case "cancelled":
			count, err = tm.ClearCancelled()
		case "all":
			c1, _ := tm.ClearCompleted()
			c2, _ := tm.ClearCancelled()
			count = c1 + c2
		default:
			return fmt.Errorf("tipo inválido: %s (usa: completed, cancelled, all)", clearType)
		}

		if err != nil {
			return err
		}

		fmt.Printf("Eliminadas %d tareas.\n", count)
		return nil
	},
}

var taskRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Procesa tareas desde un archivo markdown",
	Long: `Procesa un archivo .md con tareas en formato checkbox (- [ ] tarea).
Las tareas se ejecutan una por una con el agente de IA, sin confirmación.
Opcionalmente puede crear branches y PRs para cada tarea.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if taskRunFile == "" {
			return fmt.Errorf("debes especificar el archivo de tareas con --file")
		}

		// Cargar configuración
		mc, _, err := config.LoadModelsConfig()
		if err != nil {
			return fmt.Errorf("error loading models config: %w", err)
		}

		if taskModel != "" {
			mc.Agent.Model = taskModel
		}

		// Resolver modelo
		if mc.Agent.Provider == "ollama" {
			baseURL := mc.Agent.BaseURL
			if baseURL == "" {
				baseURL = mc.Semantic.BaseURL
			}
			if baseURL == "" {
				baseURL = "http://localhost:11434"
			}
			resolved, _, err := llm.ResolveOllamaModel(baseURL, mc.Agent.Model)
			if err != nil {
				return fmt.Errorf("error resolving model: %w", err)
			}
			mc.Agent.Model = resolved
		}

		// Crear LLM provider
		var llmProvider llm.Provider
		timeoutDur := time.Duration(taskTimeout) * time.Second
		if timeoutDur <= 0 {
			timeoutDur = 120 * time.Second
		}

		switch mc.Agent.Provider {
		case "ollama":
			baseURL := mc.Agent.BaseURL
			if baseURL == "" {
				baseURL = mc.Semantic.BaseURL
			}
			if baseURL == "" {
				baseURL = "http://localhost:11434"
			}
			llmProvider = llm.NewOllamaProviderWithTimeout(baseURL, mc.Agent.Model, timeoutDur)
		case "opencode_zen":
			llmProvider = llm.NewOpenCodeZenProviderWithOptions(
				mc.Agent.Model,
				mc.Agent.APIKey(),
				mc.Agent.BaseURL,
				timeoutDur,
			)
		case "anthropic":
			apiKey := mc.Agent.APIKey()
			llmProvider = llm.NewAnthropicProvider(apiKey, mc.Agent.Model)
		case "openai":
			apiKey := mc.Agent.APIKey()
			llmProvider = llm.NewOpenAIProvider(apiKey, mc.Agent.Model, mc.Agent.BaseURL)
		default:
			return fmt.Errorf("unknown provider: %s", mc.Agent.Provider)
		}

		// Abrir DB
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Cargar configuración del agente
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %w", err)
		}
		if taskAutoApprove {
			cfg.Approval = "auto"
		}

		// Crear cola
		queue := task.NewTaskQueue(s.DB(), llmProvider, cfg)

		// Configurar opciones
		opts := task.RunOptions{
			UseGit:        taskRunGit,
			BranchPrefix:  taskRunBranchPref,
			ValidateBuild: taskRunValidate,
			StopOnError:   taskRunStopError,
			Model:         mc.Agent.Model,
			Timeout:       taskTimeout,
			MaxTurns:      taskRunMaxTurns,
		}

		// Ejecutar
		results, err := queue.ProcessFromMarkdown(taskRunFile, opts)
		if err != nil {
			return err
		}

		// Retornar error si hubo fallas
		if results != nil {
			for _, r := range results {
				if !r.Success {
					return fmt.Errorf("task failed: %s - %s", r.Title, r.Error)
				}
			}
		}

		return nil
	},
}

func openTaskManager() (*task.TaskManager, func(), error) {
	dbPath := filepath.Join(".androideai", "core.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("database not found, run 'androideai init' first")
	}
	s, err := store.NewStore(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("error opening database: %w", err)
	}
	tm := task.NewTaskManager(s.DB())
	if err := tm.InitDB(); err != nil {
		s.Close()
		return nil, nil, fmt.Errorf("error initializing task table: %w", err)
	}
	return tm, func() { _ = s.Close() }, nil
}

func init() {
	// Flags para add
	taskAddCmd.Flags().StringVarP(&taskPriority, "priority", "p", "low", "Prioridad: low, medium, high, urgent")
	taskAddCmd.Flags().StringVarP(&taskType, "type", "t", "feature", "Tipo: feature, bugfix, refactor, test, review")
	taskAddCmd.Flags().StringP("description", "d", "", "Descripción de la tarea")

	// Flags para list
	taskListCmd.Flags().StringVarP(&taskStatus, "status", "s", "", "Filtrar por estado")

	// Flags para update
	taskUpdateCmd.Flags().StringP("title", "", "", "Nuevo título")
	taskUpdateCmd.Flags().StringP("description", "d", "", "Nueva descripción")
	taskUpdateCmd.Flags().StringVarP(&taskPriority, "priority", "p", "", "Nueva prioridad")
	taskUpdateCmd.Flags().StringVarP(&taskStatus, "status", "s", "", "Nuevo estado")
	taskUpdateCmd.Flags().StringVarP(&taskType, "type", "t", "", "Nuevo tipo")

	// Flags para process
	taskProcessCmd.Flags().Bool("all", false, "Procesar todas las tareas en cola")
	taskProcessCmd.Flags().StringVarP(&taskModel, "model", "m", "", "Override del modelo LLM")
	taskProcessCmd.Flags().IntVar(&taskTimeout, "timeout", 120, "Timeout en segundos para el LLM")
	taskProcessCmd.Flags().BoolVarP(&taskAutoApprove, "yes", "y", false, "Auto-aprobar operaciones")

	// Flags para clear
	taskClearCmd.Flags().StringP("type", "t", "completed", "Tipo a limpiar: completed, cancelled, all")

	// Flags para run
	taskRunCmd.Flags().StringVarP(&taskRunFile, "file", "f", "", "Archivo .md con tareas (requerido)")
	taskRunCmd.Flags().BoolVar(&taskRunGit, "git", false, "Habilitar workflow git (branch + PR por tarea)")
	taskRunCmd.Flags().StringVar(&taskRunBranchPref, "branch-prefix", "task/", "Prefijo para branches de git")
	taskRunCmd.Flags().BoolVar(&taskRunValidate, "validate-build", true, "Validar compilación después de cada tarea")
	taskRunCmd.Flags().BoolVar(&taskRunStopError, "stop-on-error", false, "Detener si hay error de compilación")
	taskRunCmd.Flags().StringVarP(&taskModel, "model", "m", "", "Override del modelo LLM")
	taskRunCmd.Flags().IntVar(&taskTimeout, "timeout", 120, "Timeout en segundos para el LLM")
	taskRunCmd.Flags().IntVar(&taskRunMaxTurns, "max-turns", 25, "Máximo de turnos del agente por tarea")
	taskRunCmd.Flags().BoolVarP(&taskAutoApprove, "yes", "y", false, "Auto-aprobar operaciones (siempre true para task run)")

	taskCmd.AddCommand(taskAddCmd)
	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskShowCmd)
	taskCmd.AddCommand(taskUpdateCmd)
	taskCmd.AddCommand(taskDeleteCmd)
	taskCmd.AddCommand(taskCancelCmd)
	taskCmd.AddCommand(taskQueueCmd)
	taskCmd.AddCommand(taskProcessCmd)
	taskCmd.AddCommand(taskStatsCmd)
	taskCmd.AddCommand(taskClearCmd)
	taskCmd.AddCommand(taskRunCmd)

	rootCmd.AddCommand(taskCmd)
}
