package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pablofelipe1207/androideia/internal/llm"
)

const (
	StatusActive      = "active"
	StatusCompleted   = "completed"
	StatusInterrupted = "interrupted"
	StatusApproved    = "approved"
	StatusDenied      = "denied"
)

type Conversation struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	Task         string `json:"task"`
	Status       string `json:"status"`
	ApprovalMode string `json:"approval_mode"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	Summary      string `json:"summary,omitempty"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

type StoredMessage struct {
	ID             int64        `json:"id"`
	ConversationID int64        `json:"conversation_id"`
	Role           string       `json:"role"`
	Content        string       `json:"content"`
	ToolCalls      []llm.ToolCall `json:"tool_calls,omitempty"`
	ToolCallID     string       `json:"tool_call_id,omitempty"`
	ToolName       string       `json:"tool_name,omitempty"`
	CreatedAt      int64        `json:"created_at"`
}

type Memory struct {
	db *sql.DB
}

func NewMemory(db *sql.DB) *Memory {
	return &Memory{db: db}
}

// CreateConversation crea una nueva conversación. Devuelve el ID asignado.
func (m *Memory) CreateConversation(task, title, approvalMode, provider, model string) (*Conversation, error) {
	if title == "" {
		title = deriveTitle(task)
	}
	now := time.Now().Unix()

	res, err := m.db.Exec(
		`INSERT INTO conversations (title, task, status, approval_mode, provider, model, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		title, task, StatusActive, approvalMode, provider, model, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating conversation: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting conversation id: %w", err)
	}

	return &Conversation{
		ID:           id,
		Title:        title,
		Task:         task,
		Status:       StatusActive,
		ApprovalMode: approvalMode,
		Provider:     provider,
		Model:        model,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// GetConversation devuelve una conversación por ID.
func (m *Memory) GetConversation(id int64) (*Conversation, error) {
	row := m.db.QueryRow(
		`SELECT id, title, task, status, COALESCE(approval_mode, ''), COALESCE(provider, ''),
		        COALESCE(model, ''), COALESCE(summary, ''), created_at, updated_at
		 FROM conversations WHERE id = ?`, id,
	)

	c := &Conversation{}
	err := row.Scan(&c.ID, &c.Title, &c.Task, &c.Status, &c.ApprovalMode, &c.Provider, &c.Model, &c.Summary, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("conversation %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("error getting conversation: %w", err)
	}
	return c, nil
}

// ListConversations devuelve las últimas N conversaciones (más recientes primero).
func (m *Memory) ListConversations(limit int) ([]*Conversation, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := m.db.Query(
		`SELECT id, title, task, status, COALESCE(approval_mode, ''), COALESCE(provider, ''),
		        COALESCE(model, ''), COALESCE(summary, ''), created_at, updated_at
		 FROM conversations
		 ORDER BY updated_at DESC
		 LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("error listing conversations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*Conversation
	for rows.Next() {
		c := &Conversation{}
		if err := rows.Scan(&c.ID, &c.Title, &c.Task, &c.Status, &c.ApprovalMode, &c.Provider, &c.Model, &c.Summary, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("error scanning conversation: %w", err)
		}
		out = append(out, c)
	}
	return out, nil
}

// SetStatus cambia el estado de la conversación y actualiza updated_at.
func (m *Memory) SetStatus(id int64, status string) error {
	_, err := m.db.Exec(
		`UPDATE conversations SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now().Unix(), id,
	)
	if err != nil {
		return fmt.Errorf("error updating conversation status: %w", err)
	}
	return nil
}

// Touch actualiza updated_at (para reordenar por uso reciente).
func (m *Memory) Touch(id int64) error {
	_, err := m.db.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, time.Now().Unix(), id)
	return err
}

// SetSummary guarda un resumen de la conversación (para sesiones completadas).
func (m *Memory) SetSummary(id int64, summary string) error {
	_, err := m.db.Exec(
		`UPDATE conversations SET summary = ?, updated_at = ? WHERE id = ?`,
		summary, time.Now().Unix(), id,
	)
	if err != nil {
		return fmt.Errorf("error setting summary: %w", err)
	}
	return nil
}

// GetCompletedSessionSummaries devuelve los resúmenes de las sesiones completadas.
func (m *Memory) GetCompletedSessionSummaries(limit int) ([]*Conversation, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := m.db.Query(
		`SELECT id, title, task, COALESCE(summary, ''), created_at
		 FROM conversations
		 WHERE status = 'completed' AND summary IS NOT NULL AND summary != ''
		 ORDER BY updated_at DESC
		 LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("error getting summaries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*Conversation
	for rows.Next() {
		c := &Conversation{}
		if err := rows.Scan(&c.ID, &c.Title, &c.Task, &c.Summary, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning summary: %w", err)
		}
		out = append(out, c)
	}
	return out, nil
}

// LoadMessagesForSummary carga solo los mensajes assistant y user (para generar resumen).
func (m *Memory) LoadMessagesForSummary(conversationID int64) ([]StoredMessage, error) {
	rows, err := m.db.Query(
		`SELECT id, conversation_id, role, COALESCE(content, ''), COALESCE(tool_calls, ''),
		        COALESCE(tool_call_id, ''), COALESCE(tool_name, ''), created_at
		 FROM messages WHERE conversation_id = ? AND role IN ('assistant', 'user')
		 ORDER BY id ASC`, conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("error loading messages for summary: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []StoredMessage
	for rows.Next() {
		msg := StoredMessage{}
		var toolCallsJSON string
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content, &toolCallsJSON,
			&msg.ToolCallID, &msg.ToolName, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning message: %w", err)
		}
		if toolCallsJSON != "" {
			if err := json.Unmarshal([]byte(toolCallsJSON), &msg.ToolCalls); err != nil {
				return nil, fmt.Errorf("error unmarshaling tool calls: %w", err)
			}
		}
		out = append(out, msg)
	}
	return out, nil
}

// DeleteConversation elimina una conversación y todos sus mensajes.
func (m *Memory) DeleteConversation(id int64) error {
	_, err := m.db.Exec(`DELETE FROM conversations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("error deleting conversation: %w", err)
	}
	return nil
}

// AppendMessage guarda un mensaje (system, user, assistant o tool) en la conversación.
// Para mensajes assistant con tool_calls, serializa las llamadas a JSON.
// Para mensajes tool, indica el tool_call_id y nombre asociado.
func (m *Memory) AppendMessage(conversationID int64, role, content string, toolCalls []llm.ToolCall, toolCallID, toolName string) error {
	var toolCallsJSON string
	if len(toolCalls) > 0 {
		b, err := json.Marshal(toolCalls)
		if err != nil {
			return fmt.Errorf("error marshaling tool calls: %w", err)
		}
		toolCallsJSON = string(b)
	}

	_, err := m.db.Exec(
		`INSERT INTO messages (conversation_id, role, content, tool_calls, tool_call_id, tool_name, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		conversationID, role, content, toolCallsJSON, toolCallID, toolName, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("error appending message: %w", err)
	}

	// Refresca updated_at para que la conversación suba al tope de "recientes".
	return m.Touch(conversationID)
}

// LoadMessages reconstruye el slice de mensajes de la conversación en orden.
// Los mensajes tool se asocian al tool_call_id correspondiente del assistant previo.
func (m *Memory) LoadMessages(conversationID int64) ([]StoredMessage, error) {
	rows, err := m.db.Query(
		`SELECT id, conversation_id, role, COALESCE(content, ''), COALESCE(tool_calls, ''),
		        COALESCE(tool_call_id, ''), COALESCE(tool_name, ''), created_at
		 FROM messages WHERE conversation_id = ? ORDER BY id ASC`, conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("error loading messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []StoredMessage
	for rows.Next() {
		msg := StoredMessage{}
		var toolCallsJSON string
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content, &toolCallsJSON,
			&msg.ToolCallID, &msg.ToolName, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning message: %w", err)
		}
		if toolCallsJSON != "" {
			if err := json.Unmarshal([]byte(toolCallsJSON), &msg.ToolCalls); err != nil {
				return nil, fmt.Errorf("error unmarshaling tool calls: %w", err)
			}
		}
		out = append(out, msg)
	}
	return out, nil
}

// ToLLMMessages convierte los mensajes almacenados al formato que espera el LLM,
// preservando el orden y los tool_calls del assistant.
func (m *Memory) ToLLMMessages(stored []StoredMessage) []llm.Message {
	out := make([]llm.Message, 0, len(stored))
	for _, s := range stored {
		msg := llm.Message{
			Role:    s.Role,
			Content: s.Content,
		}
		if len(s.ToolCalls) > 0 {
			msg.ToolCalls = s.ToolCalls
		}
		if s.ToolCallID != "" {
			msg.ToolCallID = s.ToolCallID
		}
		out = append(out, msg)
	}
	return out
}

func deriveTitle(task string) string {
	task = strings.TrimSpace(task)
	if task == "" {
		return "(sin título)"
	}
	// Primera línea, capada a 60 chars.
	if idx := strings.IndexAny(task, "\n"); idx > 0 {
		task = task[:idx]
	}
	if len(task) > 60 {
		task = task[:60] + "…"
	}
	return task
}
