package agent

import (
	"testing"

	"github.com/pablofelipe1207/androideia/internal/llm"
)

func mkCall(name string, args map[string]interface{}) llm.ToolCall {
	return llm.ToolCall{
		ID:   "test",
		Type: "function",
		Function: llm.ToolCallFunc{
			Name:      name,
			Arguments: args,
		},
	}
}

func TestDedupeToolCalls(t *testing.T) {

	tests := []struct {
		name      string
		in        []llm.ToolCall
		wantNames []string
	}{
		{
			name:      "empty",
			in:        nil,
			wantNames: nil,
		},
		{
			name:      "single",
			in:        []llm.ToolCall{mkCall("a", map[string]interface{}{"x": 1})},
			wantNames: []string{"a"},
		},
		{
			name: "two different",
			in: []llm.ToolCall{
				mkCall("a", map[string]interface{}{"x": 1}),
				mkCall("b", map[string]interface{}{"y": 2}),
			},
			wantNames: []string{"a", "b"},
		},
		{
			name: "two identical consecutive",
			in: []llm.ToolCall{
				mkCall("a", map[string]interface{}{"x": 1}),
				mkCall("a", map[string]interface{}{"x": 1}),
			},
			wantNames: []string{"a"},
		},
		{
			name: "three identical consecutive",
			in: []llm.ToolCall{
				mkCall("android_scaffold", map[string]interface{}{"role": "data_class", "name": "User"}),
				mkCall("android_scaffold", map[string]interface{}{"role": "data_class", "name": "User"}),
				mkCall("android_scaffold", map[string]interface{}{"role": "data_class", "name": "User"}),
			},
			wantNames: []string{"android_scaffold"},
		},
		{
			name: "same name different args are not deduped",
			in: []llm.ToolCall{
				mkCall("a", map[string]interface{}{"x": 1}),
				mkCall("a", map[string]interface{}{"x": 2}),
			},
			wantNames: []string{"a", "a"},
		},
		{
			name: "key order does not matter",
			in: []llm.ToolCall{
				mkCall("a", map[string]interface{}{"x": 1, "y": 2}),
				mkCall("a", map[string]interface{}{"y": 2, "x": 1}),
			},
			wantNames: []string{"a"},
		},
		{
			name: "only consecutive duplicates removed",
			in: []llm.ToolCall{
				mkCall("a", map[string]interface{}{"x": 1}),
				mkCall("a", map[string]interface{}{"x": 2}),
				mkCall("a", map[string]interface{}{"x": 1}),
			},
			wantNames: []string{"a", "a", "a"},
		},
		{
			name: "non-consecutive duplicates preserved",
			in: []llm.ToolCall{
				mkCall("a", map[string]interface{}{"x": 1}),
				mkCall("b", map[string]interface{}{"x": 1}),
				mkCall("a", map[string]interface{}{"x": 1}),
			},
			wantNames: []string{"a", "b", "a"},
		},
		{
			name: "nil args treated as empty",
			in: []llm.ToolCall{
				mkCall("a", nil),
				mkCall("a", nil),
			},
			wantNames: []string{"a"},
		},
		{
			name: "different arg types are not deduped",
			in: []llm.ToolCall{
				mkCall("a", map[string]interface{}{"x": 1}),
				mkCall("a", map[string]interface{}{"x": "1"}),
			},
			wantNames: []string{"a", "a"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := dedupeToolCalls(tc.in)
			if len(got) != len(tc.wantNames) {
				t.Fatalf("len = %d, want %d (%v)", len(got), len(tc.wantNames), tc.wantNames)
			}
			for i, n := range tc.wantNames {
				if got[i].Function.Name != n {
					t.Errorf("idx %d: name = %q, want %q", i, got[i].Function.Name, n)
				}
			}
		})
	}
}

func TestCanonicalArgs(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]interface{}
		want string
	}{
		{
			name: "nil",
			in:   nil,
			want: "{}",
		},
		{
			name: "empty",
			in:   map[string]interface{}{},
			want: "{}",
		},
		{
			name: "key order independent",
			in:   map[string]interface{}{"b": 2, "a": 1},
			want: `{"a":1,"b":2}`,
		},
		{
			name: "nested map order independent",
			in: map[string]interface{}{
				"x": map[string]interface{}{"b": 2, "a": 1},
				"y": "z",
			},
			want: `{"x":{"a":1,"b":2},"y":"z"}`,
		},
		{
			name: "string with quotes escaped",
			in:   map[string]interface{}{"k": `He said "hi"`},
			want: `{"k":"He said \"hi\""}`,
		},
		{
			name: "array order preserved",
			in:   map[string]interface{}{"k": []interface{}{3, 1, 2}},
			want: `{"k":[3,1,2]}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := canonicalArgs(tc.in)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestDedupeToolCalls_RealScenario reproduce el log que vimos: el
// modelo qwen2.5-coder:7b emitió 5 tool calls en un solo turno con
// `android_scaffold` repetido 2 veces con los mismos argumentos. El
// loop debe descartar las duplicadas y procesar solo 4.
func TestDedupeToolCalls_RealScenario(t *testing.T) {
	in := []llm.ToolCall{
		mkCall("semantic_locate", map[string]interface{}{"query": "data_class User"}),
		mkCall("android_scaffold", map[string]interface{}{"role": "data_class", "name": "User"}),
		mkCall("android_scaffold", map[string]interface{}{"role": "data_class", "name": "User"}), // dup
		mkCall("validate_kotlin", map[string]interface{}{"path": "com/example/app/data/model/User.kt", "role": "data_class"}),
		mkCall("confirm_plan", map[string]interface{}{"plan": "Create a User data class with id, name, and email."}),
	}

	got := dedupeToolCalls(in)
	if len(got) != 4 {
		t.Fatalf("expected 4 (with 1 dup removed), got %d: %v", len(got), got)
	}
	want := []string{"semantic_locate", "android_scaffold", "validate_kotlin", "confirm_plan"}
	for i, n := range want {
		if got[i].Function.Name != n {
			t.Errorf("idx %d: name = %q, want %q", i, got[i].Function.Name, n)
		}
	}
}
