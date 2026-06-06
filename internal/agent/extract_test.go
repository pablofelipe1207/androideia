package agent

import (
	"testing"
)

func TestExtractToolCallsFromContent(t *testing.T) {
	cases := []struct {
		name        string
		content     string
		wantCount   int
		wantNames   []string
		wantArgKeys []string // keys that must exist in arguments
	}{
		{
			name:      "single write_file",
			content:   `{"name": "write_file", "arguments": {"path": "app/Foo.kt", "content": "x"}}`,
			wantCount: 1,
			wantNames: []string{"write_file"},
		},
		{
			name: "multiple tool calls separated by text",
			content: `Let me create the files.

{"name": "write_file", "arguments": {"path": "app/Foo.kt", "content": "x"}}

And also:

{"name": "write_file", "arguments": {"path": "app/Bar.kt", "content": "y"}}
`,
			wantCount: 2,
			wantNames: []string{"write_file", "write_file"},
		},
		{
			name: "tool call inside markdown code block",
			content: "Plan:\n```json\n" + `{"name": "write_file", "arguments": {"path": "x", "content": "y"}}` + "\n```\n",
			wantCount: 1,
			wantNames: []string{"write_file"},
		},
		{
			name:      "different tool names",
			content:   `{"name": "read_file", "arguments": {"path": "x"}}`,
			wantCount: 1,
			wantNames: []string{"read_file"},
		},
		{
			name:      "no tool calls",
			content:   "I will now proceed to create the files.",
			wantCount: 0,
		},
		{
			name:      "JSON without name field",
			content:   `{"foo": "bar", "arguments": {}}`,
			wantCount: 0,
		},
		{
			name:      "JSON without arguments field still treated as tool call (empty args)",
			content:   `{"name": "write_file"}`,
			wantCount: 1,
			wantNames: []string{"write_file"},
		},
		{
			name:      "missing arguments with different name",
			content:   `{"name": "noop"}`,
			wantCount: 1,
			wantNames: []string{"noop"},
		},
		{
			name:      "nested JSON in content",
			content:   `{"name": "write_file", "arguments": {"path": "x", "content": "{\"a\": 1, \"b\": [1,2,3]}"}}`,
			wantCount: 1,
			wantNames: []string{"write_file"},
		},
		{
			name:      "escaped quotes inside strings",
			content:   `{"name": "write_file", "arguments": {"path": "x", "content": "He said \"hi\""}}`,
			wantCount: 1,
			wantNames: []string{"write_file"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := extractToolCallsFromContent(tc.content)
			if len(calls) != tc.wantCount {
				t.Fatalf("expected %d tool calls, got %d (content: %q)", tc.wantCount, len(calls), tc.content)
			}
			for i, name := range tc.wantNames {
				if i >= len(calls) {
					break
				}
				if calls[i].Function.Name != name {
					t.Errorf("call %d: expected name %q, got %q", i, name, calls[i].Function.Name)
				}
				// Verificar que cada call tiene un ID
				if calls[i].ID == "" {
					t.Errorf("call %d: missing ID", i)
				}
				// Verificar que arguments es un objeto (Ollama lo exige).
				if _, ok := calls[i].Function.Arguments.(map[string]interface{}); !ok {
					t.Errorf("call %d: arguments should be map[string]interface{}, got %T", i, calls[i].Function.Arguments)
				}
			}
		})
	}
}

func TestExtractToolCallsFromContent_Realistic(t *testing.T) {
	// Simula el patrón exacto que el usuario reportó.
	content := `### IMPLEMENTAR (Implement)

#### 1. Create Authentication Module

Let's create a new module for authentication if it doesn't exist.

` + "```json" + `
{"name": "write_file", "arguments": {"path": "app/src/main/java/com/example/myapp/auth/AuthModule.kt", "content": "package com.example.myapp.auth\n\nobject AuthModule {\n    // Authentication logic will go here\n}"}}
` + "```" + `

#### 2. Add ViewModel

` + "```json" + `
{"name": "write_file", "arguments": {"path": "app/src/main/java/com/example/myapp/auth/AuthViewModel.kt", "content": "package com.example.myapp.auth\n..."}}
` + "```" + `
`

	calls := extractToolCallsFromContent(content)
	if len(calls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(calls))
	}
	if calls[0].Function.Name != "write_file" {
		t.Errorf("expected first call write_file, got %q", calls[0].Function.Name)
	}
	if calls[1].Function.Name != "write_file" {
		t.Errorf("expected second call write_file, got %q", calls[1].Function.Name)
	}
	// Verify arguments are valid JSON object (no strings, Ollama requires objects)
	for i, c := range calls {
		argsMap, ok := c.Function.Arguments.(map[string]interface{})
		if !ok {
			t.Fatalf("call %d: arguments should be map[string]interface{}, got %T", i, c.Function.Arguments)
		}
		if _, hasPath := argsMap["path"]; !hasPath {
			t.Errorf("call %d: missing 'path' in args", i)
		}
		if _, hasContent := argsMap["content"]; !hasContent {
			t.Errorf("call %d: missing 'content' in args", i)
		}
	}

	// Simula que el loop pasa las tool calls extraídas por parseToolCall
	// y que se obtienen los argumentos correctos.
	for i, c := range calls {
		name, args, err := parseToolCall(c)
		if err != nil {
			t.Fatalf("parseToolCall %d: %v", i, err)
		}
		if name != "write_file" {
			t.Errorf("call %d: expected name write_file, got %q", i, name)
		}
		if _, ok := args["path"]; !ok {
			t.Errorf("call %d: missing 'path' in args", i)
		}
		if _, ok := args["content"]; !ok {
			t.Errorf("call %d: missing 'content' in args", i)
		}
	}
}
