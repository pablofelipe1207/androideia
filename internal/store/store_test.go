package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore(t *testing.T) {
	// Crear directorio temporal
	tmpDir, err := os.MkdirTemp("", "store-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Crear store
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Error creating store: %v", err)
	}
	defer s.Close()

	// Verificar que el archivo de base de datos existe
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("Database file was not created")
	}
}

func TestMigrations(t *testing.T) {
	// Crear directorio temporal
	tmpDir, err := os.MkdirTemp("", "store-migration-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Crear store
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Error creating store: %v", err)
	}
	defer s.Close()

	// Verificar que las tablas existen
	tables := []string{"files", "symbols", "knowledge_entries", "embeddings", "build_history"}
	for _, table := range tables {
		exists, err := s.TableExists(table)
		if err != nil {
			t.Errorf("Error checking table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("Table %s does not exist", table)
		}
	}

	// Verificar que las tablas virtuales FTS5 existen
	ftsTables := []string{"symbols_fts", "knowledge_fts"}
	for _, table := range ftsTables {
		exists, err := s.VirtualTableExists(table)
		if err != nil {
			t.Errorf("Error checking virtual table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("Virtual table %s does not exist", table)
		}
	}
}

func TestIdempotentMigrations(t *testing.T) {
	// Crear directorio temporal
	tmpDir, err := os.MkdirTemp("", "store-idempotent-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Crear store dos veces
	dbPath := filepath.Join(tmpDir, "test.db")
	s1, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Error creating first store: %v", err)
	}
	s1.Close()

	s2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Error creating second store: %v", err)
	}
	defer s2.Close()

	// Verificar que las tablas existen
	exists, err := s2.TableExists("files")
	if err != nil {
		t.Errorf("Error checking table files: %v", err)
	}
	if !exists {
		t.Error("Table files does not exist after second migration")
	}
}
