package semantic

import (
	"path/filepath"
	"testing"

	"github.com/pablofelipe1207/androideia/internal/store"
)

func newAggTestStore(t *testing.T) *store.Store {
	t.Helper()
	tmp := t.TempDir()
	s, err := store.NewStore(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func seedFileRow(t *testing.T, s *store.Store, id int64, path, pkg, layer string) {
	t.Helper()
	_, err := s.DB().Exec(
		`INSERT INTO files (id, path, package, module, layer, hash, updated_at)
		 VALUES (?, ?, ?, 'app', ?, 'h', strftime('%s','now'))`,
		id, path, pkg, layer,
	)
	if err != nil {
		t.Fatalf("insert file %d: %v", id, err)
	}
}

func seedSemantic(t *testing.T, s *store.Store, fileID int64, typ, arch, conv, summary, tagsJSON string) {
	t.Helper()
	_, err := s.DB().Exec(
		`INSERT INTO file_semantics (file_id, type, tags, architecture, conventions, summary, model, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, 'test-model', 0)`,
		fileID, typ, tagsJSON, arch, conv, summary,
	)
	if err != nil {
		t.Fatalf("insert semantic: %v", err)
	}
}

func TestAggregateConventions_Empty(t *testing.T) {
	s := newAggTestStore(t)
	sem := NewSemantic(s.DB(), "http://localhost:11434", "m")
	aggs, err := sem.AggregateConventions()
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(aggs) != 0 {
		t.Errorf("expected empty, got %d", len(aggs))
	}
}

func TestAggregateConventions_GroupsByType(t *testing.T) {
	s := newAggTestStore(t)
	sem := NewSemantic(s.DB(), "http://localhost:11434", "m")

	seedFileRow(t, s, 1, "LoginViewModel.kt", "x", "domain")
	seedFileRow(t, s, 2, "ProfileViewModel.kt", "x", "domain")
	seedFileRow(t, s, 3, "LoginUseCase.kt", "x", "domain")
	seedFileRow(t, s, 4, "UserRepositoryImpl.kt", "x", "data")

	seedSemantic(t, s, 1, "viewmodel", "MVVM", "Hilt + StateFlow", "login VM", `["login","auth","hilt-injection"]`)
	seedSemantic(t, s, 2, "viewmodel", "MVVM", "Hilt + StateFlow", "profile VM", `["profile","hilt-injection"]`)
	seedSemantic(t, s, 3, "usecase", "MVVM", "suspend + Result", "login UC", `["login","auth"]`)
	seedSemantic(t, s, 4, "repository", "MVVM", "interface + impl", "user repo", `["user","data"]`)

	aggs, err := sem.AggregateConventions()
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(aggs) != 3 {
		t.Fatalf("expected 3 roles, got %d: %+v", len(aggs), aggs)
	}

	byType := map[string]ConventionAggregate{}
	for _, a := range aggs {
		byType[a.Role] = a
	}

	vm := byType["viewmodel"]
	if vm.FileCount != 2 {
		t.Errorf("viewmodel count = %d", vm.FileCount)
	}
	if vm.Architecture != "MVVM" {
		t.Errorf("viewmodel arch = %q", vm.Architecture)
	}
	if vm.Conventions != "Hilt + StateFlow" {
		t.Errorf("viewmodel conv = %q", vm.Conventions)
	}
	if len(vm.SampleFiles) != 2 {
		t.Errorf("viewmodel sample files = %d", len(vm.SampleFiles))
	}
	// hilt-injection aparece 2 veces; login, profile, auth 1 vez
	if !containsTag(vm.Tags, "hilt-injection") {
		t.Errorf("expected hilt-injection in tags, got %v", vm.Tags)
	}

	uc := byType["usecase"]
	if uc.FileCount != 1 || uc.Conventions != "suspend + Result" {
		t.Errorf("usecase = %+v", uc)
	}
}

func TestAggregateConventions_IgnoresUnknownArch(t *testing.T) {
	s := newAggTestStore(t)
	sem := NewSemantic(s.DB(), "http://localhost:11434", "m")
	seedFileRow(t, s, 1, "A.kt", "x", "ui")
	seedFileRow(t, s, 2, "B.kt", "x", "ui")
	seedSemantic(t, s, 1, "composable", "unknown", "x", "y", `[]`)
	seedSemantic(t, s, 2, "composable", "", "x", "y", `[]`)

	aggs, _ := sem.AggregateConventions()
	if len(aggs) != 1 {
		t.Fatalf("expected 1, got %+v", aggs)
	}
	if aggs[0].Architecture != "" {
		t.Errorf("expected empty arch (all unknown/blank), got %q", aggs[0].Architecture)
	}
}

func TestAggregateConventions_DeduplicatesFiles(t *testing.T) {
	s := newAggTestStore(t)
	sem := NewSemantic(s.DB(), "http://localhost:11434", "m")
	seedFileRow(t, s, 1, "A.kt", "x", "ui")
	// In a real run we should never insert two semantics rows for
	// the same file (PRIMARY KEY on file_id), but defend anyway: if
	// the schema is altered, we still want the sample list deduped.
	_, _ = s.DB().Exec(`INSERT OR IGNORE INTO file_semantics (file_id, type, tags, architecture, conventions, summary, model, updated_at)
		VALUES (1, 'composable', '[]', 'MVVM', 'a', 'b', 'm', 0)`)
	_, _ = s.DB().Exec(`INSERT OR IGNORE INTO file_semantics (file_id, type, tags, architecture, conventions, summary, model, updated_at)
		VALUES (1, 'composable', '[]', 'MVVM', 'a', 'b', 'm', 0)`)

	aggs, _ := sem.AggregateConventions()
	if len(aggs[0].SampleFiles) != 1 {
		t.Errorf("expected deduped sample files, got %v", aggs[0].SampleFiles)
	}
}

func TestAggregateConventions_SortedByRole(t *testing.T) {
	s := newAggTestStore(t)
	sem := NewSemantic(s.DB(), "http://localhost:11434", "m")
	seedFileRow(t, s, 1, "Z.kt", "x", "ui")
	seedFileRow(t, s, 2, "A.kt", "x", "ui")
	seedSemantic(t, s, 1, "viewmodel", "", "x", "y", `[]`)
	seedSemantic(t, s, 2, "activity", "", "x", "y", `[]`)

	aggs, _ := sem.AggregateConventions()
	if len(aggs) != 2 {
		t.Fatalf("expected 2, got %d", len(aggs))
	}
	if aggs[0].Role != "activity" || aggs[1].Role != "viewmodel" {
		t.Errorf("not sorted: %+v", aggs)
	}
}

func TestConventionAggregate_BrainEntry(t *testing.T) {
	a := ConventionAggregate{
		Role:         "viewmodel",
		Architecture: "MVVM",
		Conventions:  "Hilt + StateFlow",
		Summary:      "Login ViewModel",
		SampleFiles:  []string{"a.kt", "b.kt"},
		FileCount:    2,
		Tags:         []string{"login", "auth"},
	}
	typ, title, content, tags, fileRefs := a.BrainEntry()
	if typ != "convention" {
		t.Errorf("type = %q", typ)
	}
	if title != "Viewmodel convention" {
		t.Errorf("title = %q", title)
	}
	for _, want := range []string{"Viewmodel", "MVVM", "Hilt + StateFlow", "Login ViewModel", "a.kt", "login", "auth"} {
		if !contains(content, want) {
			t.Errorf("content missing %q\n--- content ---\n%s", want, content)
		}
	}
	if !contains(tags, "viewmodel") || !contains(tags, "mvvm") {
		t.Errorf("tags = %q", tags)
	}
	if !contains(fileRefs, "a.kt") || !contains(fileRefs, "b.kt") {
		t.Errorf("fileRefs = %q", fileRefs)
	}
}

func TestMostVoted(t *testing.T) {
	if got := mostVoted(map[string]int{}, "x"); got != "x" {
		t.Errorf("fallback = %q", got)
	}
	if got := mostVoted(map[string]int{"a": 1, "b": 3, "c": 2}, ""); got != "b" {
		t.Errorf("winner = %q", got)
	}
}

func TestTopNVoted(t *testing.T) {
	m := map[string]int{"a": 1, "b": 3, "c": 2, "d": 5}
	got := topNVoted(m, 2)
	if len(got) != 2 || got[0] != "d" || got[1] != "b" {
		t.Errorf("top2 = %v", got)
	}
	all := topNVoted(m, 0)
	if len(all) != 4 {
		t.Errorf("all = %v", all)
	}
}

func TestUniqueLower(t *testing.T) {
	got := uniqueLower([]string{"ViewModel", "viewmodel", "MVVM", "", "  ", "mvvm"})
	want := []string{"viewmodel", "mvvm"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func containsTag(tags []string, want string) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}
