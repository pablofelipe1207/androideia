package agent

import (
	"fmt"
	"testing"

	"github.com/mobiai/androideai-core/internal/config"
	"github.com/mobiai/androideai-core/internal/llm"
)

func TestAgentWithOllama(t *testing.T) {
	fmt.Println("Testing agent with Ollama...")

	// Create Ollama provider
	provider := llm.NewOllamaProvider("http://localhost:11434", "qwen3-coder-64k-32k:latest")

	if !provider.IsAvailable() {
		t.Skip("Ollama is not running, skipping test")
	}

	fmt.Println("✓ Ollama is available")

	// Create config
	cfg := &config.Config{
		Provider:  "ollama",
		OllamaURL: "http://localhost:11434",
		Model:     "qwen3-coder-64k-32k:latest",
		Approval:  "auto",
	}

	// Create agent (without database for this test)
	agent := NewAgent(provider, nil, cfg)

	fmt.Println("✓ Agent created")

	// Test a simple task
	fmt.Println("Testing simple task...")
	err := agent.Run("Respond with just 'OK' and nothing else")
	if err != nil {
		t.Fatalf("Error running agent: %v", err)
	}

	fmt.Println("✓ Agent completed task successfully")
}
