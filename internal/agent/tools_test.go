package agent

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestExecuteToolConfirmPlan_MissingArgs(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir)
	defer s.Close()

	r := NewToolRegistry(s.DB())
	_, err := r.ExecuteTool("confirm_plan", map[string]interface{}{})
	if err == nil || !strings.Contains(err.Error(), "plan is required") {
		t.Fatalf("expected 'plan is required' error, got: %v", err)
	}
}

func TestExecuteToolAskUser_MissingArgs(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir)
	defer s.Close()

	r := NewToolRegistry(s.DB())
	_, err := r.ExecuteTool("ask_user", map[string]interface{}{})
	if err == nil || !strings.Contains(err.Error(), "question is required") {
		t.Fatalf("expected 'question is required' error, got: %v", err)
	}
}

func TestExecuteToolConfirmPlan_EOFReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir)
	defer s.Close()

	swapStdinReader(strings.NewReader(""))
	defer swapStdinReader(strings.NewReader("")) // restaurar

	r := NewToolRegistry(s.DB())
	result, err := r.ExecuteTool("confirm_plan", map[string]interface{}{"plan": "do X"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "denied" {
		t.Errorf("expected 'denied' on empty stdin, got %q", result)
	}
}

func TestExecuteToolConfirmPlan_ApprovedAndFeedback(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir)
	defer s.Close()

	cases := []struct {
		name     string
		input    string
		expected string
		isEdit   bool
	}{
		{"y", "y", "approved", false},
		{"yes", "yes", "approved", false},
		{"Y", "Y", "approved", false},
		{"n", "n", "denied", false},
		{"no", "no", "denied", false},
		{"empty", "", "denied", false},
		{"feedback", "please use foo", "approved (feedback: please use foo)", false},
		{"edit_then_plan", "e\nnew plan", "edit:new plan", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			swapStdinReader(strings.NewReader(c.input + "\n"))
			r := NewToolRegistry(s.DB())
			result, err := r.ExecuteTool("confirm_plan", map[string]interface{}{"plan": "do X"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.isEdit {
				if !strings.HasPrefix(result, "edit:") {
					t.Errorf("expected prefix 'edit:', got %q", result)
				}
				if !strings.Contains(result, "new plan") {
					t.Errorf("expected result to contain 'new plan', got %q", result)
				}
			} else if result != c.expected {
				t.Errorf("expected %q, got %q", c.expected, result)
			}
		})
	}
}

func TestExecuteToolAskUser_FreeText(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir)
	defer s.Close()

	swapStdinReader(strings.NewReader("Hilt\n"))
	r := NewToolRegistry(s.DB())
	result, err := r.ExecuteTool("ask_user", map[string]interface{}{"question": "DI framework?"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hilt" {
		t.Errorf("expected 'Hilt', got %q", result)
	}
}

func TestReadUserResponse_EOF(t *testing.T) {
	swapStdinReader(errorReader{})
	got := readUserResponse()
	if got != "" {
		t.Errorf("expected empty string on EOF, got %q", got)
	}
}

func TestSwapStdinReader_OnlyAffectsAgent(t *testing.T) {
	// Smoke test de que el helper no rompe nada.
	swapStdinReader(strings.NewReader("hello\n"))
	if got := readUserResponse(); got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
	swapStdinReader(strings.NewReader("")) // restaurar para tests posteriores
}

var _ = errors.New

// errorReader siempre devuelve error.
type errorReader struct{}

func (errorReader) Read(_ []byte) (int, error) {
	return 0, io.EOF
}
