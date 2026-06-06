package agent

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/pablofelipe1207/androideia/internal/memory"
	"github.com/pablofelipe1207/androideia/internal/store"
)

func TestLooksLikeConfirmationRequest(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "english please confirm",
			content: "Before proceeding, please confirm that you want to create the login screen.",
			want:    true,
		},
		{
			name:    "english should i proceed",
			content: "I will now create the files. Should I proceed?",
			want:    true,
		},
		{
			name:    "spanish confirmas",
			content: "Voy a crear el archivo LoginScreen.kt. ¿Confirmas?",
			want:    true,
		},
		{
			name:    "spanish procedo",
			content: "Plan listo. ¿Procedo con la implementación?",
			want:    true,
		},
		{
			name:    "spanish quieres",
			content: "¿Quieres que use Hilt o Koin?",
			want:    true,
		},
		{
			name:    "english would you like",
			content: "Would you like me to also add the tests?",
			want:    true,
		},
		{
			name:    "non confirmation text",
			content: "I created the file successfully. The build is green.",
			want:    false,
		},
		{
			name:    "empty",
			content: "",
			want:    false,
		},
		{
			name:    "question but not about confirming",
			content: "What is the package name?",
			want:    false,
		},
		{
			name:    "should i continue",
			content: "I stopped at the ViewModel. Should I continue with the Repository?",
			want:    true,
		},
	}

	a := &Agent{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := a.looksLikeConfirmationRequest(tc.content)
			if got != tc.want {
				t.Errorf("looksLikeConfirmationRequest(%q) = %v, want %v", tc.content, got, tc.want)
			}
		})
	}
}

func TestAgentStartAndResumeSession(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("error creating store: %v", err)
	}
	defer s.Close()

	cfg := &config.Config{Provider: "ollama", OllamaURL: "x", Model: "m", Approval: "ask"}
	provider := newMockProvider()
	a := NewAgent(provider, s.DB(), cfg)
	a.SetMemory(memory.NewMemory(s.DB()))

	id, err := a.StartSession("Add login screen")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	a.persistMessage("user", "Add login screen", nil, "", "")
	a.persistMessage("assistant", "Plan: ...", nil, "", "")

	// Reanudar con un agente nuevo.
	provider2 := newMockProvider()
	a2 := NewAgent(provider2, s.DB(), cfg)
	a2.SetMemory(memory.NewMemory(s.DB()))

	originalTask, err := a2.ResumeSession(id)
	if err != nil {
		t.Fatalf("ResumeSession: %v", err)
	}
	if originalTask != "Add login screen" {
		t.Errorf("expected original task %q, got %q", "Add login screen", originalTask)
	}
	if len(a2.messages) < 3 {
		t.Errorf("expected at least 3 messages after resume (system + user + assistant), got %d", len(a2.messages))
	}
	if a2.messages[0].Role != "system" {
		t.Errorf("expected first message system, got %q", a2.messages[0].Role)
	}
}

func TestAgentResumeRejectsCompletedSession(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("error creating store: %v", err)
	}
	defer s.Close()

	cfg := &config.Config{Provider: "ollama", OllamaURL: "x", Model: "m", Approval: "ask"}
	provider := newMockProvider()
	a := NewAgent(provider, s.DB(), cfg)
	a.SetMemory(memory.NewMemory(s.DB()))

	id, _ := a.StartSession("Task")
	_ = memory.NewMemory(s.DB()).SetStatus(id, memory.StatusCompleted)

	provider2 := newMockProvider()
	a2 := NewAgent(provider2, s.DB(), cfg)
	a2.SetMemory(memory.NewMemory(s.DB()))

	if _, err := a2.ResumeSession(id); err == nil {
		t.Error("expected error resuming completed session")
	} else if !strings.Contains(err.Error(), "completed") {
		t.Errorf("expected error to mention 'completed', got: %v", err)
	}
}
