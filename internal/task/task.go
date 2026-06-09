package task

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Priority levels
const (
	PriorityLow     = 0
	PriorityMedium  = 1
	PriorityHigh    = 2
	PriorityUrgent  = 3
)

// Status values
const (
	StatusPending     = "pending"
	StatusQueued      = "queued"
	StatusProcessing  = "processing"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
	StatusCancelled   = "cancelled"
)

// Task types
const (
	TypeFeature  = "feature"
	TypeBugfix   = "bugfix"
	TypeRefactor = "refactor"
	TypeTest     = "test"
	TypeReview   = "review"
)

// Task representa una tarea en el sistema.
type Task struct {
	ID             int64  `json:"id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Priority       int    `json:"priority"`
	Status         string `json:"status"`
	Type           string `json:"type"`
	Result         string `json:"result,omitempty"`
	Error          string `json:"error,omitempty"`
	ConversationID int64  `json:"conversation_id,omitempty"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
	StartedAt      int64  `json:"started_at,omitempty"`
	CompletedAt    int64  `json:"completed_at,omitempty"`
}

// TaskManager gestiona las tareas en la base de datos.
type TaskManager struct {
	db *sql.DB
}

// NewTaskManager crea un nuevo TaskManager.
func NewTaskManager(db *sql.DB) *TaskManager {
	return &TaskManager{db: db}
}

// InitDB crea la tabla de tareas si no existe.
func (tm *TaskManager) InitDB() error {
	_, err := tm.db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT,
			priority INTEGER DEFAULT 0,
			status TEXT DEFAULT 'pending',
			type TEXT,
			result TEXT,
			error TEXT,
			conversation_id INTEGER,
			created_at INTEGER,
			updated_at INTEGER,
			started_at INTEGER,
			completed_at INTEGER
		)
	`)
	return err
}

// Add crea una nueva tarea.
func (tm *TaskManager) Add(title, description, taskType string, priority int) (*Task, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if priority < PriorityLow || priority > PriorityUrgent {
		priority = PriorityLow
	}
	if taskType == "" {
		taskType = TypeFeature
	}

	now := time.Now().Unix()
	res, err := tm.db.Exec(
		`INSERT INTO tasks (title, description, priority, status, type, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		title, description, priority, StatusPending, taskType, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating task: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting task id: %w", err)
	}

	return &Task{
		ID:          id,
		Title:       title,
		Description: description,
		Priority:    priority,
		Status:      StatusPending,
		Type:        taskType,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// Get obtiene una tarea por ID.
func (tm *TaskManager) Get(id int64) (*Task, error) {
	row := tm.db.QueryRow(
		`SELECT id, title, COALESCE(description, ''), priority, status, COALESCE(type, ''),
		        COALESCE(result, ''), COALESCE(error, ''), COALESCE(conversation_id, 0),
		        created_at, updated_at, COALESCE(started_at, 0), COALESCE(completed_at, 0)
		 FROM tasks WHERE id = ?`, id,
	)

	t := &Task{}
	err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Priority, &t.Status, &t.Type,
		&t.Result, &t.Error, &t.ConversationID, &t.CreatedAt, &t.UpdatedAt,
		&t.StartedAt, &t.CompletedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("error getting task: %w", err)
	}
	return t, nil
}

// List lista tareas con filtros opcionales.
func (tm *TaskManager) List(status string, limit int) ([]*Task, error) {
	if limit <= 0 {
		limit = 50
	}

	var query string
	var args []interface{}

	if status != "" {
		query = `SELECT id, title, COALESCE(description, ''), priority, status, COALESCE(type, ''),
		                COALESCE(result, ''), COALESCE(error, ''), COALESCE(conversation_id, 0),
		                created_at, updated_at, COALESCE(started_at, 0), COALESCE(completed_at, 0)
				 FROM tasks WHERE status = ? ORDER BY priority DESC, created_at ASC LIMIT ?`
		args = append(args, status, limit)
	} else {
		query = `SELECT id, title, COALESCE(description, ''), priority, status, COALESCE(type, ''),
		                COALESCE(result, ''), COALESCE(error, ''), COALESCE(conversation_id, 0),
		                created_at, updated_at, COALESCE(started_at, 0), COALESCE(completed_at, 0)
				 FROM tasks ORDER BY priority DESC, created_at ASC LIMIT ?`
		args = append(args, limit)
	}

	rows, err := tm.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error listing tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		t := &Task{}
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Priority, &t.Status, &t.Type,
			&t.Result, &t.Error, &t.ConversationID, &t.CreatedAt, &t.UpdatedAt,
			&t.StartedAt, &t.CompletedAt); err != nil {
			return nil, fmt.Errorf("error scanning task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// Update actualiza campos de una tarea.
func (tm *TaskManager) Update(id int64, updates map[string]interface{}) error {
	t, err := tm.Get(id)
	if err != nil {
		return err
	}

	setClauses := []string{"updated_at = ?"}
	args := []interface{}{time.Now().Unix()}

	if title, ok := updates["title"].(string); ok && title != "" {
		setClauses = append(setClauses, "title = ?")
		args = append(args, title)
	}
	if desc, ok := updates["description"].(string); ok {
		setClauses = append(setClauses, "description = ?")
		args = append(args, desc)
	}
	if priority, ok := updates["priority"].(int); ok {
		setClauses = append(setClauses, "priority = ?")
		args = append(args, priority)
	}
	if status, ok := updates["status"].(string); ok && status != "" {
		setClauses = append(setClauses, "status = ?")
		args = append(args, status)
		if status == StatusProcessing && t.StartedAt == 0 {
			setClauses = append(setClauses, "started_at = ?")
			args = append(args, time.Now().Unix())
		}
		if status == StatusCompleted || status == StatusFailed {
			setClauses = append(setClauses, "completed_at = ?")
			args = append(args, time.Now().Unix())
		}
	}
	if taskType, ok := updates["type"].(string); ok && taskType != "" {
		setClauses = append(setClauses, "type = ?")
		args = append(args, taskType)
	}
	if result, ok := updates["result"].(string); ok {
		setClauses = append(setClauses, "result = ?")
		args = append(args, result)
	}
	if errMsg, ok := updates["error"].(string); ok {
		setClauses = append(setClauses, "error = ?")
		args = append(args, errMsg)
	}
	if convID, ok := updates["conversation_id"].(int64); ok {
		setClauses = append(setClauses, "conversation_id = ?")
		args = append(args, convID)
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE tasks SET %s WHERE id = ?", strings.Join(setClauses, ", "))

	_, err = tm.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("error updating task: %w", err)
	}
	return nil
}

// Delete elimina una tarea.
func (tm *TaskManager) Delete(id int64) error {
	_, err := tm.db.Exec("DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("error deleting task: %w", err)
	}
	return nil
}

// Cancel cancela una tarea.
func (tm *TaskManager) Cancel(id int64) error {
	return tm.Update(id, map[string]interface{}{"status": StatusCancelled})
}

// Queue agrega una tarea a la cola (cambia status a queued).
func (tm *TaskManager) Queue(id int64) error {
	return tm.Update(id, map[string]interface{}{"status": StatusQueued})
}

// QueueMultiple agrega múltiples tareas a la cola.
func (tm *TaskManager) QueueMultiple(ids []int64) error {
	for _, id := range ids {
		if err := tm.Queue(id); err != nil {
			return fmt.Errorf("error queuing task %d: %w", id, err)
		}
	}
	return nil
}

// GetNextToProcess obtiene la siguiente tarea a procesar (mayor prioridad primero).
func (tm *TaskManager) GetNextToProcess() (*Task, error) {
	row := tm.db.QueryRow(
		`SELECT id, title, COALESCE(description, ''), priority, status, COALESCE(type, ''),
		        COALESCE(result, ''), COALESCE(error, ''), COALESCE(conversation_id, 0),
		        created_at, updated_at, COALESCE(started_at, 0), COALESCE(completed_at, 0)
		 FROM tasks WHERE status = ? ORDER BY priority DESC, created_at ASC LIMIT 1`,
		StatusQueued,
	)

	t := &Task{}
	err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Priority, &t.Status, &t.Type,
		&t.Result, &t.Error, &t.ConversationID, &t.CreatedAt, &t.UpdatedAt,
		&t.StartedAt, &t.CompletedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no tasks in queue")
	}
	if err != nil {
		return nil, fmt.Errorf("error getting next task: %w", err)
	}
	return t, nil
}

// GetQueueStats devuelve estadísticas de la cola.
func (tm *TaskManager) GetQueueStats() (map[string]int, error) {
	rows, err := tm.db.Query(
		`SELECT status, COUNT(*) FROM tasks GROUP BY status`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		stats[status] = count
	}
	return stats, nil
}

// ClearCompleted elimina todas las tareas completadas.
func (tm *TaskManager) ClearCompleted() (int64, error) {
	res, err := tm.db.Exec("DELETE FROM tasks WHERE status = ?", StatusCompleted)
	if err != nil {
		return 0, fmt.Errorf("error clearing completed tasks: %w", err)
	}
	return res.RowsAffected()
}

// ClearCancelled elimina todas las tareas canceladas.
func (tm *TaskManager) ClearCancelled() (int64, error) {
	res, err := tm.db.Exec("DELETE FROM tasks WHERE status = ?", StatusCancelled)
	if err != nil {
		return 0, fmt.Errorf("error clearing cancelled tasks: %w", err)
	}
	return res.RowsAffected()
}

// PriorityToString convierte un nivel de prioridad a string.
func PriorityToString(p int) string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityMedium:
		return "medium"
	case PriorityHigh:
		return "high"
	case PriorityUrgent:
		return "urgent"
	default:
		return "unknown"
	}
}

// StringToPriority convierte un string a nivel de prioridad.
func StringToPriority(s string) int {
	switch strings.ToLower(s) {
	case "low", "baja":
		return PriorityLow
	case "medium", "media":
		return PriorityMedium
	case "high", "alta":
		return PriorityHigh
	case "urgent", "urgente":
		return PriorityUrgent
	default:
		return PriorityLow
	}
}
