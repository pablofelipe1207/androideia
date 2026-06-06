package llm

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestListModels_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"models":[{"name":"llama3.2:3b"},{"name":"qwen2.5-coder:7b"}]}`)
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL, "ignored")
	models, err := provider.ListModels()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0] != "llama3.2:3b" || models[1] != "qwen2.5-coder:7b" {
		t.Errorf("unexpected models: %v", models)
	}
}

func TestListModels_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"models":[]}`)
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL, "ignored")
	models, err := provider.ListModels()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 0 {
		t.Errorf("expected 0 models, got %d", len(models))
	}
}

func TestListModels_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL, "ignored")
	if _, err := provider.ListModels(); err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestListModels_Unreachable(t *testing.T) {
	provider := NewOllamaProvider("http://127.0.0.1:1", "ignored") // closed port
	if _, err := provider.ListModels(); err == nil {
		t.Fatal("expected error for unreachable host, got nil")
	}
}

func TestResolveOllamaModel_SingleModel_AutoSelect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"models":[{"name":"llama3.2:3b"}]}`)
	}))
	defer server.Close()

	got, auto, err := ResolveOllamaModel(server.URL, "qwen3-coder-64k-32k:latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "llama3.2:3b" {
		t.Errorf("expected 'llama3.2:3b', got %q", got)
	}
	if !auto {
		t.Error("expected autoSelected=true when Ollama has one model different from config")
	}
}

func TestResolveOllamaModel_SingleModel_MatchesConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"models":[{"name":"llama3.2:3b"}]}`)
	}))
	defer server.Close()

	got, auto, err := ResolveOllamaModel(server.URL, "llama3.2:3b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "llama3.2:3b" {
		t.Errorf("expected 'llama3.2:3b', got %q", got)
	}
	if auto {
		t.Error("expected autoSelected=false when configured model == only model")
	}
}

func TestResolveOllamaModel_MultipleModels_ConfigInList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"models":[{"name":"llama3.2:3b"},{"name":"qwen2.5-coder:7b"}]}`)
	}))
	defer server.Close()

	got, auto, err := ResolveOllamaModel(server.URL, "qwen2.5-coder:7b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "qwen2.5-coder:7b" {
		t.Errorf("expected config model, got %q", got)
	}
	if auto {
		t.Error("expected autoSelected=false when config model is in list")
	}
}

func TestResolveOllamaModel_MultipleModels_ConfigNotInList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"models":[{"name":"llama3.2:3b"},{"name":"qwen2.5-coder:7b"}]}`)
	}))
	defer server.Close()

	_, _, err := ResolveOllamaModel(server.URL, "missing-model:latest")
	if err == nil {
		t.Fatal("expected error when config model not in list, got nil")
	}
}

func TestResolveOllamaModel_NoModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"models":[]}`)
	}))
	defer server.Close()

	_, _, err := ResolveOllamaModel(server.URL, "any-model")
	if err == nil {
		t.Fatal("expected error when Ollama has no models, got nil")
	}
}

func TestResolveOllamaModel_OllamaUnreachable_FallbackToConfig(t *testing.T) {
	// When Ollama is down we silently fall back to the configured model
	// (the IsAvailable check will produce a clearer error later).
	got, auto, err := ResolveOllamaModel("http://127.0.0.1:1", "configured-model")
	if err != nil {
		t.Fatalf("expected no error on unreachable Ollama, got: %v", err)
	}
	if got != "configured-model" {
		t.Errorf("expected fallback to 'configured-model', got %q", got)
	}
	if auto {
		t.Error("expected autoSelected=false on fallback")
	}
}
