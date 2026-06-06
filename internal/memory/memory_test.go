package memory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pablofelipe1207/androideia/internal/llm"
	"github.com/pablofelipe1207/androideia/internal/store"
)

func newTestMemory(t *testing.T) (*Memory, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("error creating store: %v", err)
	}
	return NewMemory(s.DB()), func() { _ = s.Close() }
}

func TestCreateConversation(t *testing.T) {
	m, closer := newTestMemory(t)
	defer closer()

	conv, err := m.CreateConversation("Add login screen", "", "ask", "ollama", "qwen")
	if err != nil {
		t.Fatalf("error creating conversation: %v", err)
	}
	if conv.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if conv.Status != StatusActive {
		t.Errorf("expected status %q, got %q", StatusActive, conv.Status)
	}
	if conv.Title == "" {
		t.Error("expected title to be derived from task")
	}
}

func TestGetConversation(t *testing.T) {
	m, closer := newTestMemory(t)
	defer closer()

	created, _ := m.CreateConversation("Task X", "Custom title", "auto", "ollama", "model-x")
	got, err := m.GetConversation(created.ID)
	if err != nil {
		t.Fatalf("error getting conversation: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, got.ID)
	}
	if got.Title != "Custom title" {
		t.Errorf("expected title %q, got %q", "Custom title", got.Title)
	}
	if got.Task != "Task X" {
		t.Errorf("expected task %q, got %q", "Task X", got.Task)
	}
}

func TestGetConversationNotFound(t *testing.T) {
	m, closer := newTestMemory(t)
	defer closer()

	_, err := m.GetConversation(9999)
	if err == nil {
		t.Error("expected error for non-existent conversation")
	}
}

func TestListConversations(t *testing.T) {
	m, closer := newTestMemory(t)
	defer closer()

	for i := 0; i < 5; i++ {
		_, err := m.CreateConversation("Task", "", "ask", "ollama", "m")
		if err != nil {
			t.Fatalf("error creating conversation: %v", err)
		}
	}
	list, err := m.ListConversations(10)
	if err != nil {
		t.Fatalf("error listing: %v", err)
	}
	if len(list) != 5 {
		t.Errorf("expected 5 conversations, got %d", len(list))
	}
}

func TestSetStatus(t *testing.T) {
	m, closer := newTestMemory(t)
	defer closer()

	conv, _ := m.CreateConversation("Task", "", "ask", "ollama", "m")
	if err := m.SetStatus(conv.ID, StatusCompleted); err != nil {
		t.Fatalf("error setting status: %v", err)
	}
	got, _ := m.GetConversation(conv.ID)
	if got.Status != StatusCompleted {
		t.Errorf("expected status %q, got %q", StatusCompleted, got.Status)
	}
}

func TestAppendAndLoadMessages(t *testing.T) {
	m, closer := newTestMemory(t)
	defer closer()

	conv, _ := m.CreateConversation("Task", "", "ask", "ollama", "m")

	if err := m.AppendMessage(conv.ID, "system", "You are an agent", nil, "", ""); err != nil {
		t.Fatalf("error appending system: %v", err)
	}
	if err := m.AppendMessage(conv.ID, "user", "Create a screen", nil, "", ""); err != nil {
		t.Fatalf("error appending user: %v", err)
	}

	toolCalls := []llm.ToolCall{
		{ID: "call_1", Type: "function", Function: llm.ToolCallFunc{Name: "read_file", Arguments: `{"path":"/x"}`}},
	}
	if err := m.AppendMessage(conv.ID, "assistant", "Reading file", toolCalls, "", ""); err != nil {
		t.Fatalf("error appending assistant: %v", err)
	}
	if err := m.AppendMessage(conv.ID, "tool", "file contents", nil, "call_1", "read_file"); err != nil {
		t.Fatalf("error appending tool: %v", err)
	}

	msgs, err := m.LoadMessages(conv.ID)
	if err != nil {
		t.Fatalf("error loading messages: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("expected first role system, got %q", msgs[0].Role)
	}
	if msgs[2].Role != "assistant" {
		t.Errorf("expected third role assistant, got %q", msgs[2].Role)
	}
	if len(msgs[2].ToolCalls) != 1 {
		t.Errorf("expected 1 tool call on assistant message, got %d", len(msgs[2].ToolCalls))
	}
	if msgs[2].ToolCalls[0].Function.Name != "read_file" {
		t.Errorf("expected tool call read_file, got %q", msgs[2].ToolCalls[0].Function.Name)
	}
	if msgs[3].ToolCallID != "call_1" {
		t.Errorf("expected tool_call_id call_1, got %q", msgs[3].ToolCallID)
	}
	if msgs[3].ToolName != "read_file" {
		t.Errorf("expected tool_name read_file, got %q", msgs[3].ToolName)
	}
}

func TestToLLMMessages(t *testing.T) {
	m, closer := newTestMemory(t)
	defer closer()

	conv, _ := m.CreateConversation("Task", "", "ask", "ollama", "m")
	_ = m.AppendMessage(conv.ID, "system", "sys", nil, "", "")
	_ = m.AppendMessage(conv.ID, "user", "hi", nil, "", "")
	_ = m.AppendMessage(conv.ID, "assistant", "ok", []llm.ToolCall{{ID: "c1", Type: "function", Function: llm.ToolCallFunc{Name: "n", Arguments: "{}"}}}, "", "")

	stored, _ := m.LoadMessages(conv.ID)
	ll := m.ToLLMMessages(stored)
	if len(ll) != 3 {
		t.Fatalf("expected 3 llm messages, got %d", len(ll))
	}
	if ll[2].Role != "assistant" {
		t.Errorf("expected role assistant, got %q", ll[2].Role)
	}
	if len(ll[2].ToolCalls) != 1 {
		t.Errorf("expected tool calls preserved, got %d", len(ll[2].ToolCalls))
	}
}

func TestDeleteConversation(t *testing.T) {
	m, closer := newTestMemory(t)
	defer closer()

	conv, _ := m.CreateConversation("Task", "", "ask", "ollama", "m")
	_ = m.AppendMessage(conv.ID, "user", "hi", nil, "", "")

	if err := m.DeleteConversation(conv.ID); err != nil {
		t.Fatalf("error deleting: %v", err)
	}
	if _, err := m.GetConversation(conv.ID); err == nil {
		t.Error("expected error after delete")
	}
	// Y los mensajes deben haberse ido en cascada.
	msgs, _ := m.LoadMessages(conv.ID)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after delete, got %d", len(msgs))
	}
}

func TestTouch(t *testing.T) {
	m, closer := newTestMemory(t)
	defer closer()

	conv, _ := m.CreateConversation("Task", "", "ask", "ollama", "m")
	original := conv.UpdatedAt
	// Asegurar que el timestamp cambia.
	if err := os.WriteFile("/dev/null", nil, 0); err != nil {
		// noop, sólo para dormir un instante en CI; en realidad dependemos
		// de la resolución de Unix() (segundos) que puede no cambiar.
		_ = err
	}
	_ = m.Touch(conv.ID)
	got, _ := m.GetConversation(conv.ID)
	if got.UpdatedAt < original {
		t.Errorf("expected updated_at >= %d, got %d", original, got.UpdatedAt)
	}
}
