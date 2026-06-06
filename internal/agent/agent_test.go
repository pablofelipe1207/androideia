package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/pablofelipe1207/androideia/internal/llm"
	"github.com/pablofelipe1207/androideia/internal/store"
)

func TestNewToolRegistry(t *testing.T) {
	// Create a temporary database
	tmpDir, err := os.MkdirTemp("", "agent-tools-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Error creating store: %v", err)
	}
	defer s.Close()

	// Create tool registry
	registry := NewToolRegistry(s.DB())
	if registry == nil {
		t.Fatal("Tool registry is nil")
	}

	// Check that tools are registered
	tools := registry.GetTools()
	if len(tools) == 0 {
		t.Error("No tools registered")
	}

	// Check for specific tools
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Function.Name] = true
	}

	expectedTools := []string{"read_file", "write_file", "index_search", "index_feature", "brain_search", "gradle", "test"}
	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("Tool '%s' not registered", expected)
		}
	}
}

func TestExecuteToolReadFile(t *testing.T) {
	// Create a temporary file
	tmpDir, err := os.MkdirTemp("", "agent-readfile-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, world!"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Error creating test file: %v", err)
	}

	// Create a temporary database
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Error creating store: %v", err)
	}
	defer s.Close()

	// Create tool registry
	registry := NewToolRegistry(s.DB())

	// Execute read_file tool
	result, err := registry.ExecuteTool("read_file", map[string]interface{}{
		"path": testFile,
	})
	if err != nil {
		t.Fatalf("Error executing read_file: %v", err)
	}

	if result != testContent {
		t.Errorf("Expected '%s', got '%s'", testContent, result)
	}
}

func TestExecuteToolWriteFile(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "agent-writefile-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temporary database
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Error creating store: %v", err)
	}
	defer s.Close()

	// Create tool registry
	registry := NewToolRegistry(s.DB())

	// Execute write_file tool
	testFile := filepath.Join(tmpDir, "output.txt")
	testContent := "Test content"
	result, err := registry.ExecuteTool("write_file", map[string]interface{}{
		"path":    testFile,
		"content": testContent,
	})
	if err != nil {
		t.Fatalf("Error executing write_file: %v", err)
	}

	// Verify file was written
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Error reading written file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Expected '%s', got '%s'", testContent, string(content))
	}

	// Check result message
	if result != "File written: "+testFile {
		t.Errorf("Expected result message, got '%s'", result)
	}
}

func TestExecuteToolIndexSearch(t *testing.T) {
	// Create a temporary database
	tmpDir, err := os.MkdirTemp("", "agent-indexsearch-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Error creating store: %v", err)
	}
	defer s.Close()

	// Insert test data
	_, err = s.DB().Exec(`INSERT INTO files (path, package, module, layer, hash, updated_at) 
		VALUES ('test.kt', 'com.example', 'app', 'ui', 'hash1', strftime('%s', 'now'))`)
	if err != nil {
		t.Fatalf("Error inserting file: %v", err)
	}

	_, err = s.DB().Exec(`INSERT INTO symbols (file_id, name, kind, signature, line, feature) 
		VALUES (1, 'LoginScreen', 'screen', '@Composable fun LoginScreen()', 10, 'login')`)
	if err != nil {
		t.Fatalf("Error inserting symbol: %v", err)
	}

	// Insert into FTS
	_, err = s.DB().Exec(`INSERT INTO symbols_fts (name, signature, package, path, doc) 
		VALUES ('LoginScreen', '@Composable fun LoginScreen()', 'com.example', 'test.kt', '@Composable fun LoginScreen()')`)
	if err != nil {
		t.Fatalf("Error inserting into FTS: %v", err)
	}

	// Create tool registry
	registry := NewToolRegistry(s.DB())

	// Execute index_search tool
	result, err := registry.ExecuteTool("index_search", map[string]interface{}{
		"query": "LoginScreen",
	})
	if err != nil {
		t.Fatalf("Error executing index_search: %v", err)
	}

	// Check that we got results
	if result == "No results found" {
		t.Error("Expected search results, got none")
	}
}

func TestParseToolCall(t *testing.T) {
	// Test parsing tool call
	toolCall := llm.ToolCall{
		ID:   "call_123",
		Type: "function",
		Function: llm.ToolCallFunc{
			Name:      "read_file",
			Arguments: `{"path": "/path/to/file.txt"}`,
		},
	}

	name, args, err := parseToolCall(toolCall)
	if err != nil {
		t.Fatalf("Error parsing tool call: %v", err)
	}

	if name != "read_file" {
		t.Errorf("Expected name 'read_file', got '%s'", name)
	}

	if args["path"] != "/path/to/file.txt" {
		t.Errorf("Expected path '/path/to/file.txt', got '%v'", args["path"])
	}
}

func TestBuildUserPrompt(t *testing.T) {
	// Test building user prompt
	task := "Create a login screen"
	prompt := BuildUserPrompt(task)

	if prompt != task {
		t.Errorf("Expected prompt to be task, got '%s'", prompt)
	}
}

func TestBuildContextPrompt(t *testing.T) {
	// Test building context prompt
	context := map[string]interface{}{
		"features": []string{"login", "register"},
		"files":    []string{"LoginScreen.kt", "RegisterScreen.kt"},
		"knowledge": []string{"Use Hilt for DI"},
	}

	prompt := BuildContextPrompt(context)

	// Check that prompt contains context information
	if len(prompt) == 0 {
		t.Error("Context prompt is empty")
	}

	// Check for specific content
	if !containsString(prompt, "login") {
		t.Error("Context prompt missing feature 'login'")
	}

	if !containsString(prompt, "LoginScreen.kt") {
		t.Error("Context prompt missing file 'LoginScreen.kt'")
	}

	if !containsString(prompt, "Use Hilt for DI") {
		t.Error("Context prompt missing knowledge 'Use Hilt for DI'")
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestNewAgent(t *testing.T) {
	// Create a temporary database
	tmpDir, err := os.MkdirTemp("", "agent-new-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Error creating store: %v", err)
	}
	defer s.Close()

	// Create a mock LLM provider
	llmProvider := llm.NewOllamaProvider("http://localhost:11434", "test-model")

	// Create config
	cfg := &config.Config{
		Model:     "test-model",
		OllamaURL: "http://localhost:11434",
		Provider:  "ollama",
		Approval:  "ask",
	}

	// Create agent
	agent := NewAgent(llmProvider, s.DB(), cfg)
	if agent == nil {
		t.Fatal("Agent is nil")
	}

	// Check that agent has correct components
	if agent.llm != llmProvider {
		t.Error("Agent LLM provider mismatch")
	}

	if agent.config != cfg {
		t.Error("Agent config mismatch")
	}

	// Check conversation history
	history := agent.GetConversationHistory()
	if len(history) != 1 {
		t.Errorf("Expected 1 message in history, got %d", len(history))
	}

	if history[0].Role != "system" {
		t.Errorf("Expected first message to be system, got '%s'", history[0].Role)
	}
}
