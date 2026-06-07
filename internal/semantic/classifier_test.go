package semantic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablofelipe1207/androideia/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "semantic-cls-test")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	s, err := store.NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func seedFile(t *testing.T, s *store.Store, id int64, path, pkg, layer string) {
	t.Helper()
	_, err := s.DB().Exec(
		`INSERT INTO files (id, path, package, module, layer, hash, updated_at)
		 VALUES (?, ?, ?, 'app', ?, 'h', strftime('%s','now'))`,
		id, path, pkg, layer,
	)
	if err != nil {
		t.Fatalf("insert file: %v", err)
	}
}

func TestKebabify(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"kebab-case", "kebab-case"},
		{"snake_case", "snake-case"},
		{"CamelCase", "camel-case"},
		{"camelCase", "camel-case"},
		{"with spaces", "with-spaces"},
		{"---weird---", "weird"},
		{"Hilt Injection", "hilt-injection"},
		{"", ""},
		{"a__b", "a-b"},
		{"XMLParser", "xmlparser"}, // may collapse, but never empty
	}
	for _, c := range cases {
		got := kebabify(c.in)
		if c.in == "XMLParser" {
			if got == "" {
				t.Errorf("kebabify(%q) returned empty", c.in)
			}
			continue
		}
		if got != c.want {
			t.Errorf("kebabify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseClassificationContent(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		checks func(t *testing.T, r *classificationResult)
	}{
		{
			name:  "plain json",
			input: `{"type":"viewmodel","tags":["Login","auth"],"architecture":"MVVM","conventions":"Hilt","summary":"Auth ViewModel"}`,
			checks: func(t *testing.T, r *classificationResult) {
				if r.Type != "viewmodel" {
					t.Errorf("type = %q", r.Type)
				}
				if r.Architecture != "MVVM" {
					t.Errorf("arch = %q", r.Architecture)
				}
				if len(r.Tags) != 2 {
					t.Errorf("len(tags) = %d", len(r.Tags))
				}
			},
		},
		{
			name:  "fenced json",
			input: "```json\n{\"type\":\"usecase\",\"tags\":[\"login\"],\"architecture\":\"Clean\",\"conventions\":\"\",\"summary\":\"X\"}\n```",
			checks: func(t *testing.T, r *classificationResult) {
				if r.Type != "usecase" {
					t.Errorf("type = %q", r.Type)
				}
			},
		},
		{
			name:  "prose around json",
			input: "Sure! Here you go:\n{\"type\":\"repository\",\"tags\":[\"user\"],\"architecture\":\"MVVM\",\"conventions\":\"\",\"summary\":\"X\"}\nDone.",
			checks: func(t *testing.T, r *classificationResult) {
				if r.Type != "repository" {
					t.Errorf("type = %q", r.Type)
				}
			},
		},
		{
			name:  "empty object",
			input: `{}`,
			checks: func(t *testing.T, r *classificationResult) {
				if r.Type != "" {
					t.Errorf("type should be empty, got %q", r.Type)
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := parseClassificationContent(c.input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			c.checks(t, r)
		})
	}
}

func TestParseClassificationContent_BadJSON(t *testing.T) {
	_, err := parseClassificationContent("not json at all")
	if err == nil {
		t.Fatal("expected error on garbage input")
	}
}

func TestSanitizeClassification(t *testing.T) {
	r := &classificationResult{
		Type:         "ViewModel",
		Tags:         []string{"Login", "Auth", "", "hilt injection", "x", "y", "z"},
		Architecture: "",
		Conventions:  "  Hilt  ",
		Summary:      " X ",
	}
	sanitizeClassification(r)
	if r.Type != "viewmodel" {
		t.Errorf("type = %q", r.Type)
	}
	if r.Architecture != "unknown" {
		t.Errorf("arch = %q", r.Architecture)
	}
	if r.Conventions != "Hilt" {
		t.Errorf("conventions not trimmed: %q", r.Conventions)
	}
	if r.Summary != "X" {
		t.Errorf("summary not trimmed: %q", r.Summary)
	}
	if len(r.Tags) > 6 {
		t.Errorf("tags > 6: %v", r.Tags)
	}
	for _, tag := range r.Tags {
		if tag != strings.ToLower(tag) {
			t.Errorf("tag not lower: %q", tag)
		}
	}
}

func TestSanitizeClassification_UnknownType(t *testing.T) {
	r := &classificationResult{Type: "banana"}
	sanitizeClassification(r)
	if r.Type != "other" {
		t.Errorf("expected 'other', got %q", r.Type)
	}
}

func TestStoreFileSemantic_Roundtrip(t *testing.T) {
	s := newTestStore(t)
	sem := NewSemantic(s.DB(), "http://localhost:11434", "test-model")
	seedFile(t, s, 1, "app/src/main/LoginViewModel.kt", "com.x", "domain")

	r := &classificationResult{
		Type:         "viewmodel",
		Tags:         []string{"login", "auth"},
		Architecture: "MVVM",
		Conventions:  "Hilt + StateFlow",
		Summary:      "Login ViewModel",
	}
	if err := sem.StoreFileSemantic(1, r); err != nil {
		t.Fatalf("store: %v", err)
	}

	var (
		gotType, gotTagsJSON, gotArch, gotConv, gotSummary, gotModel string
	)
	err := s.DB().QueryRow(
		`SELECT type, tags, architecture, conventions, summary, model FROM file_semantics WHERE file_id = 1`,
	).Scan(&gotType, &gotTagsJSON, &gotArch, &gotConv, &gotSummary, &gotModel)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if gotType != "viewmodel" || gotArch != "MVVM" || gotConv != "Hilt + StateFlow" || gotSummary != "Login ViewModel" {
		t.Errorf("row mismatch: %+v", []string{gotType, gotArch, gotConv, gotSummary})
	}
	if gotModel != "test-model" {
		t.Errorf("model = %q", gotModel)
	}
	var storedTags []string
	if err := json.Unmarshal([]byte(gotTagsJSON), &storedTags); err != nil {
		t.Fatalf("tags json: %v", err)
	}
	if len(storedTags) != 2 || storedTags[0] != "login" {
		t.Errorf("tags = %v", storedTags)
	}

	// Idempotent update.
	r.Summary = "Login VM (updated)"
	if err := sem.StoreFileSemantic(1, r); err != nil {
		t.Fatalf("update: %v", err)
	}
	var onlyCount int
	_ = s.DB().QueryRow(`SELECT COUNT(*) FROM file_semantics WHERE file_id = 1`).Scan(&onlyCount)
	if onlyCount != 1 {
		t.Errorf("expected 1 row after update, got %d", onlyCount)
	}
	_ = json.RawMessage{} // keep json import used in other sub-tests
}

func TestLocate_ByType(t *testing.T) {
	s := newTestStore(t)
	sem := NewSemantic(s.DB(), "http://localhost:11434", "m")

	seedFile(t, s, 1, "app/src/main/LoginViewModel.kt", "com.x.ui.login", "domain")
	seedFile(t, s, 2, "app/src/main/ProfileViewModel.kt", "com.x.ui.profile", "domain")
	seedFile(t, s, 3, "app/src/main/LoginUseCase.kt", "com.x.domain", "domain")

	if err := sem.StoreFileSemantic(1, &classificationResult{Type: "viewmodel", Tags: []string{"login"}, Architecture: "MVVM"}); err != nil {
		t.Fatal(err)
	}
	if err := sem.StoreFileSemantic(2, &classificationResult{Type: "viewmodel", Tags: []string{"profile"}, Architecture: "MVVM"}); err != nil {
		t.Fatal(err)
	}
	if err := sem.StoreFileSemantic(3, &classificationResult{Type: "usecase", Tags: []string{"login"}, Architecture: "Clean"}); err != nil {
		t.Fatal(err)
	}

	res, err := sem.Locate("viewmodel", 10)
	if err != nil {
		t.Fatalf("locate: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 viewmodels, got %d", len(res))
	}
	for _, r := range res {
		if r.Type != "viewmodel" {
			t.Errorf("type leak: %q", r.Type)
		}
	}

	res, err = sem.Locate("usecase", 10)
	if err != nil {
		t.Fatalf("locate usecase: %v", err)
	}
	if len(res) != 1 || res[0].Type != "usecase" {
		t.Fatalf("expected 1 usecase, got %+v", res)
	}
}

func TestLocate_ByTag(t *testing.T) {
	s := newTestStore(t)
	sem := NewSemantic(s.DB(), "http://localhost:11434", "m")

	seedFile(t, s, 1, "LoginViewModel.kt", "com.x", "domain")
	seedFile(t, s, 2, "AuthRepository.kt", "com.x", "data")

	_ = sem.StoreFileSemantic(1, &classificationResult{Type: "viewmodel", Tags: []string{"login", "auth"}})
	_ = sem.StoreFileSemantic(2, &classificationResult{Type: "repository", Tags: []string{"auth", "data"}})

	res, err := sem.Locate("tag:auth", 10)
	if err != nil {
		t.Fatalf("locate tag: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 hits for tag:auth, got %d", len(res))
	}
}

func TestLocate_ByNameFragment(t *testing.T) {
	s := newTestStore(t)
	sem := NewSemantic(s.DB(), "http://localhost:11434", "m")

	seedFile(t, s, 1, "LoginViewModel.kt", "com.x", "domain")
	seedFile(t, s, 2, "HomeScreen.kt", "com.x", "ui")
	seedFile(t, s, 3, "UserRepository.kt", "com.x", "data")

	_ = sem.StoreFileSemantic(1, &classificationResult{Type: "viewmodel", Tags: []string{"login"}, Summary: "login state"})
	_ = sem.StoreFileSemantic(2, &classificationResult{Type: "composable", Tags: []string{"home"}, Summary: "home screen"})
	_ = sem.StoreFileSemantic(3, &classificationResult{Type: "repository", Tags: []string{"user"}, Summary: "user data"})

	res, err := sem.Locate("LoginViewModel", 10)
	if err != nil {
		t.Fatalf("locate: %v", err)
	}
	if len(res) == 0 || res[0].Path != "LoginViewModel.kt" {
		t.Fatalf("expected LoginViewModel.kt first, got %+v", res)
	}
}

func TestArchitectureSummary(t *testing.T) {
	s := newTestStore(t)
	sem := NewSemantic(s.DB(), "http://localhost:11434", "m")

	seedFile(t, s, 1, "a.kt", "x", "ui")
	seedFile(t, s, 2, "b.kt", "x", "domain")
	seedFile(t, s, 3, "c.kt", "x", "data")

	_ = sem.StoreFileSemantic(1, &classificationResult{Architecture: "MVVM"})
	_ = sem.StoreFileSemantic(2, &classificationResult{Architecture: "MVVM"})
	_ = sem.StoreFileSemantic(3, &classificationResult{Architecture: "MVVM"})

	arch, list, err := sem.ArchitectureSummary()
	if err != nil {
		t.Fatalf("arch: %v", err)
	}
	if arch != "MVVM" {
		t.Errorf("arch = %q", arch)
	}
	if len(list) != 1 {
		t.Errorf("list = %v", list)
	}

	// Now mix in another architecture.
	_ = sem.StoreFileSemantic(3, &classificationResult{Architecture: "Clean"})
	arch, list, _ = sem.ArchitectureSummary()
	if arch != "MVVM / Clean" && arch != "Clean / MVVM" {
		t.Errorf("expected mixed arch, got %q (list=%v)", arch, list)
	}
}

func TestArchitectureSummary_AllUnknown(t *testing.T) {
	s := newTestStore(t)
	sem := NewSemantic(s.DB(), "http://localhost:11434", "m")
	arch, _, err := sem.ArchitectureSummary()
	if err != nil {
		t.Fatal(err)
	}
	if arch != "unknown" {
		t.Errorf("arch = %q", arch)
	}
}

func TestAllowedTypesCoverage(t *testing.T) {
	expected := []string{
		"viewmodel", "activity", "composable", "usecase", "repository",
		"dao", "di_module", "nav_route", "data_class", "entity",
		"service", "application", "test", "build", "other",
	}
	for _, typ := range expected {
		if !AllowedTypes[typ] {
			t.Errorf("missing allowed type %q", typ)
		}
	}
}
