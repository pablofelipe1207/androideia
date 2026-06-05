package llm

import (
	"fmt"
	"testing"
)

func TestOllamaConnection(t *testing.T) {
	// Test with default Ollama URL
	provider := NewOllamaProvider("http://localhost:11434", "qwen3-coder-64k-32k:latest")

	fmt.Println("Testing Ollama connection...")

	// Check if Ollama is available
	if !provider.IsAvailable() {
		t.Skip("Ollama is not running, skipping test")
	}

	fmt.Println("✓ Ollama is available")

	// Test chat with a simple message
	messages := []Message{
		{Role: "user", Content: "Hello, respond with just 'OK'"},
	}

	resp, err := provider.Chat(messages, nil)
	if err != nil {
		t.Fatalf("Error calling Ollama: %v", err)
	}

	fmt.Printf("Response choices: %d\n", len(resp.Choices))
	if len(resp.Choices) == 0 {
		t.Fatal("No response from Ollama")
	}

	fmt.Printf("✓ Ollama responded: %s\n", resp.Choices[0].Message.Content)
}
