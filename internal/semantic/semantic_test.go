package semantic

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/mobiai/androideai-core/internal/store"
)

func TestNewSemantic(t *testing.T) {
	// Create a temporary database
	tmpDir, err := os.MkdirTemp("", "semantic-test")
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

	// Create semantic instance
	semantic := NewSemantic(s.DB(), "http://localhost:11434", "nomic-embed-text")
	if semantic == nil {
		t.Fatal("Semantic instance is nil")
	}
}

func TestCosineSimilarity(t *testing.T) {
	// Test identical vectors
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	similarity := CosineSimilarity(a, b)
	if math.Abs(float64(similarity-1.0)) > 0.0001 {
		t.Errorf("Expected similarity 1.0 for identical vectors, got %f", similarity)
	}

	// Test orthogonal vectors
	a = []float32{1, 0, 0}
	b = []float32{0, 1, 0}
	similarity = CosineSimilarity(a, b)
	if math.Abs(float64(similarity)) > 0.0001 {
		t.Errorf("Expected similarity 0.0 for orthogonal vectors, got %f", similarity)
	}

	// Test opposite vectors
	a = []float32{1, 0}
	b = []float32{-1, 0}
	similarity = CosineSimilarity(a, b)
	if math.Abs(float64(similarity-(-1.0))) > 0.0001 {
		t.Errorf("Expected similarity -1.0 for opposite vectors, got %f", similarity)
	}

	// Test different length vectors
	a = []float32{1, 0}
	b = []float32{1, 0, 0}
	similarity = CosineSimilarity(a, b)
	if similarity != 0 {
		t.Errorf("Expected similarity 0.0 for different length vectors, got %f", similarity)
	}

	// Test zero vectors
	a = []float32{0, 0}
	b = []float32{0, 0}
	similarity = CosineSimilarity(a, b)
	if similarity != 0 {
		t.Errorf("Expected similarity 0.0 for zero vectors, got %f", similarity)
	}
}

func TestStoreAndGetEmbedding(t *testing.T) {
	// Create a temporary database
	tmpDir, err := os.MkdirTemp("", "semantic-store-test")
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

	// Create semantic instance
	semantic := NewSemantic(s.DB(), "http://localhost:11434", "nomic-embed-text")

	// Store embedding
	embedding := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	if err := semantic.StoreEmbedding(1, "test-model", embedding); err != nil {
		t.Fatalf("Error storing embedding: %v", err)
	}

	// Get embedding
	retrieved, err := semantic.GetEmbeddingBySymbolID(1)
	if err != nil {
		t.Fatalf("Error getting embedding: %v", err)
	}

	if len(retrieved) != len(embedding) {
		t.Errorf("Expected embedding length %d, got %d", len(embedding), len(retrieved))
	}

	for i := range embedding {
		if math.Abs(float64(retrieved[i]-embedding[i])) > 0.0001 {
			t.Errorf("Embedding mismatch at index %d: expected %f, got %f", i, embedding[i], retrieved[i])
		}
	}
}

func TestSearchFTS(t *testing.T) {
	// Create a temporary database
	tmpDir, err := os.MkdirTemp("", "semantic-fts-test")
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

	// Create semantic instance
	semantic := NewSemantic(s.DB(), "http://localhost:11434", "nomic-embed-text")

	// Search using FTS fallback
	results, err := semantic.searchFTS("LoginScreen", 10)
	if err != nil {
		t.Fatalf("Error searching FTS: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected search results, got none")
	}

	if len(results) > 0 && results[0].Name != "LoginScreen" {
		t.Errorf("Expected 'LoginScreen', got '%s'", results[0].Name)
	}
}
