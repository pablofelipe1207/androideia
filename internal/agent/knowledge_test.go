package agent

import (
	"fmt"
	"testing"

	"github.com/pablofelipe1207/androideia/internal/brain"
	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/pablofelipe1207/androideia/internal/llm"
	"github.com/pablofelipe1207/androideia/internal/store"
)

func TestSearchRelevantKnowledge(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Error creating store: %v", err)
	}
	defer s.Close()

	// Add some knowledge entries
	b := brain.NewBrain(s.DB())
	
	// Add a knowledge entry
	entry := &brain.KnowledgeEntry{
		Type:    "architecture",
		Title:   "MVVM Pattern",
		Content: "We use MVVM architecture with ViewModel, Repository, and UseCase layers.",
		Tags:    "architecture,mvvm",
		Status:  "promoted",
	}
	
	if _, err := b.Save(entry, false); err != nil {
		t.Fatalf("Error saving entry: %v", err)
	}

	// Create agent
	cfg := &config.Config{
		Provider:  "ollama",
		OllamaURL: "http://localhost:11434",
		Model:     "qwen3-coder-64k-32k:latest",
		Approval:  "auto",
	}

	provider := newMockProvider()
	agent := NewAgent(provider, s.DB(), cfg)

	// Test search
	knowledge := agent.searchRelevantKnowledge("Create a new ViewModel")
	fmt.Printf("Knowledge found: %s\n", knowledge)

	if knowledge == "" {
		t.Error("Expected knowledge to be found")
	}

	// Test with unrelated task
	knowledge = agent.searchRelevantKnowledge("Fix the build.gradle file")
	fmt.Printf("Knowledge for unrelated task: %s\n", knowledge)
}

type mockProvider struct{}

func newMockProvider() *mockProvider {
	return &mockProvider{}
}

func (m *mockProvider) Chat(messages []llm.Message, tools []llm.Tool) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{
		Choices: []llm.Choice{
			{Message: llm.Message{Role: "assistant", Content: "OK"}},
		},
	}, nil
}

func (m *mockProvider) IsAvailable() bool {
	return true
}
