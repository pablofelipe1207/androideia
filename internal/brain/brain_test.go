package brain

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mobiai/androideai-core/internal/store"
)

func TestNewBrain(t *testing.T) {
	// Create a temporary database
	tmpDir, err := os.MkdirTemp("", "brain-test")
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

	// Create brain instance
	b := NewBrain(s.DB())
	if b == nil {
		t.Fatal("Brain instance is nil")
	}
}

func TestSave(t *testing.T) {
	// Create a temporary database
	tmpDir, err := os.MkdirTemp("", "brain-save-test")
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

	// Create brain instance
	b := NewBrain(s.DB())

	// Create entry
	entry := &KnowledgeEntry{
		Type:    "decision",
		Title:   "Use Hilt for DI",
		Content: "We decided to use Hilt for dependency injection",
		Tags:    "di,hilt,android",
		Status:  "temp",
	}

	// Save entry
	id, err := b.Save(entry, false)
	if err != nil {
		t.Fatalf("Error saving entry: %v", err)
	}

	if id == 0 {
		t.Error("Expected non-zero ID")
	}

	// Verify entry was saved
	var count int
	err = s.DB().QueryRow("SELECT COUNT(*) FROM knowledge_entries WHERE id = ?", id).Scan(&count)
	if err != nil {
		t.Fatalf("Error counting entries: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 entry, got %d", count)
	}
}

func TestSearch(t *testing.T) {
	// Create a temporary database
	tmpDir, err := os.MkdirTemp("", "brain-search-test")
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

	// Create brain instance
	b := NewBrain(s.DB())

	// Save entries
	entries := []*KnowledgeEntry{
		{Type: "decision", Title: "Use Hilt", Content: "Use Hilt for DI", Status: "temp"},
		{Type: "pattern", Title: "MVVM Pattern", Content: "Use MVVM architecture", Status: "promoted"},
		{Type: "gotcha", Title: "Memory Leak", Content: "Watch for memory leaks in ViewModels", Status: "temp"},
	}

	for _, entry := range entries {
		if _, err := b.Save(entry, false); err != nil {
			t.Fatalf("Error saving entry: %v", err)
		}
	}

	// Search
	results, err := b.Search("Hilt")
	if err != nil {
		t.Fatalf("Error searching: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if len(results) > 0 && results[0].Title != "Use Hilt" {
		t.Errorf("Expected 'Use Hilt', got '%s'", results[0].Title)
	}
}

func TestReview(t *testing.T) {
	// Create a temporary database
	tmpDir, err := os.MkdirTemp("", "brain-review-test")
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

	// Create brain instance
	b := NewBrain(s.DB())

	// Save entries with different statuses
	entries := []*KnowledgeEntry{
		{Type: "decision", Title: "Temp Entry 1", Content: "Content 1", Status: "temp"},
		{Type: "pattern", Title: "Promoted Entry", Content: "Content 2", Status: "promoted"},
		{Type: "gotcha", Title: "Temp Entry 2", Content: "Content 3", Status: "temp"},
	}

	for _, entry := range entries {
		if _, err := b.Save(entry, false); err != nil {
			t.Fatalf("Error saving entry: %v", err)
		}
	}

	// Review (should only return temp entries)
	tempEntries, err := b.Review()
	if err != nil {
		t.Fatalf("Error reviewing: %v", err)
	}

	if len(tempEntries) != 2 {
		t.Errorf("Expected 2 temp entries, got %d", len(tempEntries))
	}

	// Verify all entries are temp
	for _, entry := range tempEntries {
		if entry.Status != "temp" {
			t.Errorf("Expected status 'temp', got '%s'", entry.Status)
		}
	}
}

func TestPromote(t *testing.T) {
	// Create a temporary database
	tmpDir, err := os.MkdirTemp("", "brain-promote-test")
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

	// Create brain instance
	b := NewBrain(s.DB())

	// Save entry
	entry := &KnowledgeEntry{
		Type:    "decision",
		Title:   "Test Entry",
		Content: "Test content",
		Status:  "temp",
	}

	id, err := b.Save(entry, false)
	if err != nil {
		t.Fatalf("Error saving entry: %v", err)
	}

	// Promote entry
	if err := b.Promote(id); err != nil {
		t.Fatalf("Error promoting entry: %v", err)
	}

	// Verify entry is promoted
	var status string
	err = s.DB().QueryRow("SELECT status FROM knowledge_entries WHERE id = ?", id).Scan(&status)
	if err != nil {
		t.Fatalf("Error getting status: %v", err)
	}

	if status != "promoted" {
		t.Errorf("Expected status 'promoted', got '%s'", status)
	}
}

func TestExportToMarkdown(t *testing.T) {
	// Create a temporary database
	tmpDir, err := os.MkdirTemp("", "brain-export-test")
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

	// Create brain instance
	b := NewBrain(s.DB())

	// Save entries
	entries := []*KnowledgeEntry{
		{Type: "decision", Title: "Decision 1", Content: "Content 1", Status: "promoted", CreatedAt: time.Now().Unix()},
		{Type: "pattern", Title: "Pattern 1", Content: "Content 2", Status: "temp", CreatedAt: time.Now().Unix()},
	}

	for _, entry := range entries {
		if _, err := b.Save(entry, false); err != nil {
			t.Fatalf("Error saving entry: %v", err)
		}
	}

	// Export to markdown
	exportPath := filepath.Join(tmpDir, "knowledge.md")
	if err := b.ExportToMarkdown(exportPath); err != nil {
		t.Fatalf("Error exporting: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(exportPath); os.IsNotExist(err) {
		t.Fatal("Export file was not created")
	}

	// Read and verify content
	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("Error reading export file: %v", err)
	}

	content := string(data)
	if len(content) == 0 {
		t.Error("Export file is empty")
	}

	// Check that it contains expected content
	if !containsString(content, "# Project Knowledge") {
		t.Error("Export file missing header")
	}
	if !containsString(content, "Decision 1") {
		t.Error("Export file missing Decision 1")
	}
	if !containsString(content, "Pattern 1") {
		t.Error("Export file missing Pattern 1")
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
