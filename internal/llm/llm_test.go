package llm

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNormalizeMessagesForOllama_StringArgumentsBecomeObject(t *testing.T) {
	// Caso típico del fallback: arguments viene como string JSON.
	in := []Message{
		{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{
					ID:   "call_text_1",
					Type: "function",
					Function: ToolCallFunc{
						Name:      "ask_user",
						Arguments: `{"question": "¿Hilt o Koin?"}`,
					},
				},
			},
		},
	}
	out := normalizeMessagesForOllama(in)

	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
	if len(out[0].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(out[0].ToolCalls))
	}
	args, ok := out[0].ToolCalls[0].Function.Arguments.(map[string]interface{})
	if !ok {
		t.Fatalf("expected arguments to be map[string]interface{}, got %T", out[0].ToolCalls[0].Function.Arguments)
	}
	if args["question"] != "¿Hilt o Koin?" {
		t.Errorf("expected question to be preserved, got %v", args["question"])
	}

	// Verificar que al serializar de nuevo a JSON, arguments es un
	// OBJETO, no un string (que es lo que Ollama rechaza).
	b, err := json.Marshal(out[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(b, &probe); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	raw, ok := probe["tool_calls"]
	if !ok {
		t.Fatalf("expected tool_calls in serialized message")
	}
	// Extraer el `arguments` del primer tool_call serializado.
	var tcList []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &tcList); err != nil {
		t.Fatalf("unmarshal tool_calls: %v", err)
	}
	var fn map[string]json.RawMessage
	if err := json.Unmarshal(tcList[0]["function"], &fn); err != nil {
		t.Fatalf("unmarshal function: %v", err)
	}
	argBytes := fn["arguments"]
	// Debe empezar por '{' (objeto), no por '"' (string).
	if len(argBytes) == 0 {
		t.Fatalf("arguments missing")
	}
	if argBytes[0] != '{' {
		t.Errorf("expected arguments to start with '{' (object), got %q", string(argBytes))
	}
}

func TestNormalizeMessagesForOllama_ObjectArgumentsUnchanged(t *testing.T) {
	in := []Message{
		{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{
					Function: ToolCallFunc{
						Name:      "ask_user",
						Arguments: map[string]interface{}{"question": "ok"},
					},
				},
			},
		},
	}
	out := normalizeMessagesForOllama(in)
	args, ok := out[0].ToolCalls[0].Function.Arguments.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", out[0].ToolCalls[0].Function.Arguments)
	}
	if args["question"] != "ok" {
		t.Errorf("expected question=ok, got %v", args["question"])
	}
}

func TestNormalizeMessagesForOllama_NilArguments(t *testing.T) {
	in := []Message{
		{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{
					Function: ToolCallFunc{Name: "noop", Arguments: nil},
				},
			},
		},
	}
	out := normalizeMessagesForOllama(in)
	if out[0].ToolCalls[0].Function.Arguments == nil {
		t.Errorf("expected nil arguments to be replaced with empty object")
	}
}

func TestNormalizeMessagesForOllama_NoToolCalls(t *testing.T) {
	in := []Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
		{Role: "tool", Content: "result"},
	}
	out := normalizeMessagesForOllama(in)
	if len(out) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(out))
	}
	// Los mensajes sin tool_calls no se tocan.
	if out[0].Content != "hi" || out[1].Content != "hello" || out[2].Content != "result" {
		t.Errorf("messages were mutated unexpectedly")
	}
}

func TestNormalizeMessagesForOllama_RealisticFallback(t *testing.T) {
	// Caso realista: múltiples tool calls extraídas del texto, algunas
	// con arguments string, otras con arguments objeto.
	in := []Message{
		{Role: "system", Content: "You are an agent"},
		{Role: "user", Content: "Create a login feature"},
		{
			Role: "assistant",
			Content: "Plan:\n```json\n{\"name\":\"ask_user\",\"arguments\":{\"question\":\"...\"}}\n```",
			ToolCalls: []ToolCall{
				{
					ID:   "call_text_1",
					Type: "function",
					Function: ToolCallFunc{
						Name:      "ask_user",
						Arguments: `{"question": "specific integrations?"}`,
					},
				},
			},
		},
		{
			Role:       "tool",
			Content:    "Confirm",
			ToolCallID: "call_text_1",
		},
	}
	out := normalizeMessagesForOllama(in)

	// El assistant message debe tener arguments como objeto.
	args, ok := out[2].ToolCalls[0].Function.Arguments.(map[string]interface{})
	if !ok {
		t.Fatalf("expected arguments to be map, got %T", out[2].ToolCalls[0].Function.Arguments)
	}
	if args["question"] != "specific integrations?" {
		t.Errorf("expected question to be preserved, got %v", args["question"])
	}

	// Verificar que el JSON completo del request es válido y que
	// arguments es un objeto en el wire.
	all := map[string]interface{}{"messages": out}
	b, err := json.Marshal(all)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Buscar "arguments":"  (string) — no debe aparecer.
	if contains(b, []byte(`"arguments":"`)) {
		t.Errorf("found 'arguments' as string in wire JSON; Ollama will reject this")
	}
	// Buscar "arguments":{ — debe aparecer.
	if !contains(b, []byte(`"arguments":{`)) {
		t.Errorf("expected 'arguments' as object in wire JSON")
	}
}

func contains(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func TestNewOllamaProviderWithTimeout(t *testing.T) {
	cases := []struct {
		name    string
		timeout time.Duration
		want    time.Duration
	}{
		{"positive timeout", 5 * time.Minute, 5 * time.Minute},
		{"zero uses default", 0, 120 * time.Second},
		{"negative uses default", -1 * time.Second, 120 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewOllamaProviderWithTimeout("http://x", "m", tc.timeout)
			if p == nil {
				t.Fatal("provider is nil")
			}
			if p.httpClient.Timeout != tc.want {
				t.Errorf("timeout = %s, want %s", p.httpClient.Timeout, tc.want)
			}
		})
	}
}

func TestNewOllamaProvider_DefaultTimeout(t *testing.T) {
	p := NewOllamaProvider("http://x", "m")
	if p.httpClient.Timeout != 120*time.Second {
		t.Errorf("expected default 120s, got %s", p.httpClient.Timeout)
	}
}
