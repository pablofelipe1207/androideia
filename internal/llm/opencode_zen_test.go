package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenCodeZenProvider_Chat_Success(t *testing.T) {
	var gotAuth string
	var gotPath string
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-1",
			"object": "chat.completion",
			"created": 1700000000,
			"model": "minimax-m3-free",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "Hello!"
					},
					"finish_reason": "stop"
				}
			]
		}`))
	}))
	defer server.Close()

	p := NewOpenCodeZenProviderWithOptions("minimax-m3-free", "", server.URL, 0)

	resp, err := p.Chat([]Message{{Role: "user", Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path = %q, want /chat/completions", gotPath)
	}
	if gotAuth != "" {
		t.Errorf("auth header = %q, want empty (no key)", gotAuth)
	}
	if gotBody["model"] != "minimax-m3-free" {
		t.Errorf("model in body = %v, want minimax-m3-free", gotBody["model"])
	}
	if gotBody["stream"] != false {
		t.Errorf("stream in body = %v, want false", gotBody["stream"])
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("len(choices) = %d, want 1", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Hello!" {
		t.Errorf("content = %q, want Hello!", resp.Choices[0].Message.Content)
	}
}

func TestOpenCodeZenProvider_Chat_WithAPIKey(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

	p := NewOpenCodeZenProviderWithOptions("minimax-m3-free", "sk-test-123", server.URL, 0)
	_, err := p.Chat([]Message{{Role: "user", Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if gotAuth != "Bearer sk-test-123" {
		t.Errorf("auth = %q, want %q", gotAuth, "Bearer sk-test-123")
	}
}

func TestOpenCodeZenProvider_Chat_WithTools(t *testing.T) {
	var gotBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"1","type":"function","function":{"name":"foo","arguments":"{}"}}]}}]}`))
	}))
	defer server.Close()

	p := NewOpenCodeZenProviderWithOptions("minimax-m3-free", "", server.URL, 0)
	tools := []Tool{{Type: "function", Function: ToolFunction{Name: "foo", Description: "test tool"}}}
	resp, err := p.Chat([]Message{{Role: "user", Content: "hi"}}, tools)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	toolsSent, ok := gotBody["tools"].([]interface{})
	if !ok || len(toolsSent) != 1 {
		t.Fatalf("tools not sent correctly: %v", gotBody["tools"])
	}
	if len(resp.Choices[0].Message.ToolCalls) != 1 {
		t.Errorf("tool calls = %d, want 1", len(resp.Choices[0].Message.ToolCalls))
	}
}

func TestOpenCodeZenProvider_Chat_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer server.Close()

	p := NewOpenCodeZenProviderWithOptions("minimax-m3-free", "bad-key", server.URL, 0)
	_, err := p.Chat([]Message{{Role: "user", Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention status 401, got: %v", err)
	}
}

func TestOpenCodeZenProvider_Chat_TrimsBaseURLTrailingSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, want /chat/completions (no double slash)", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

	p := NewOpenCodeZenProviderWithOptions("minimax-m3-free", "", server.URL+"/", 0)
	if _, err := p.Chat([]Message{{Role: "user", Content: "hi"}}, nil); err != nil {
		t.Fatalf("Chat: %v", err)
	}
}

func TestOpenCodeZenProvider_IsAvailable(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"available", http.StatusOK, true},
		{"unauthorized", http.StatusUnauthorized, false},
		{"server error", http.StatusInternalServerError, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer server.Close()

			p := NewOpenCodeZenProviderWithOptions("minimax-m3-free", "", server.URL, 0)
			if got := p.IsAvailable(); got != tc.want {
				t.Errorf("IsAvailable = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestOpenCodeZenProvider_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("path = %q, want /models", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"object": "list",
			"data": [
				{"id": "minimax-m3-free", "object": "model", "created": 1, "owned_by": "opencode"},
				{"id": "minimax-m2.5", "object": "model", "created": 1, "owned_by": "opencode"},
				{"id": "gpt-5", "object": "model", "created": 1, "owned_by": "opencode"}
			]
		}`))
	}))
	defer server.Close()

	p := NewOpenCodeZenProviderWithOptions("minimax-m3-free", "", server.URL, 0)
	models, err := p.ListModels()
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("len(models) = %d, want 3", len(models))
	}
	if models[0] != "minimax-m3-free" {
		t.Errorf("first model = %q, want minimax-m3-free", models[0])
	}
}

func TestOpenCodeZenProvider_Embed_NotImplementedYet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"model does not support embeddings"}`))
	}))
	defer server.Close()

	p := NewOpenCodeZenProviderWithOptions("minimax-m3-free", "", server.URL, 0)
	_, err := p.Embed("hello world")
	if err == nil {
		t.Fatal("expected error (Zen doesn't support embeddings), got nil")
	}
	// El mensaje de error debe guiar al usuario hacia Ollama.
	if !strings.Contains(err.Error(), "embedding") {
		t.Errorf("error should mention embeddings: %v", err)
	}
}

func TestOpenCodeZenProvider_Embed_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Errorf("path = %q, want /embeddings", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"object": "list",
			"data": [
				{"object": "embedding", "index": 0, "embedding": [0.1, 0.2, 0.3]}
			],
			"model": "some-embedding-model"
		}`))
	}))
	defer server.Close()

	p := NewOpenCodeZenProviderWithOptions("some-embedding-model", "", server.URL, 0)
	vec, err := p.Embed("hello")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vec) != 3 || vec[0] != 0.1 || vec[1] != 0.2 || vec[2] != 0.3 {
		t.Errorf("vec = %v, want [0.1, 0.2, 0.3]", vec)
	}
}

func TestOpenCodeZenProvider_DefaultBaseURL(t *testing.T) {
	p := NewOpenCodeZenProvider("minimax-m3-free")
	if p.baseURL != "https://opencode.ai/zen/v1" {
		t.Errorf("baseURL = %q, want https://opencode.ai/zen/v1", p.baseURL)
	}
}

func TestOpenCodeZenProvider_DefaultTimeout(t *testing.T) {
	p := NewOpenCodeZenProvider("minimax-m3-free")
	if p.httpClient.Timeout != 120*1e9 { // 120s en nanos
		t.Errorf("timeout = %v, want 120s", p.httpClient.Timeout)
	}
}
