package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("error creating database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}

	// Habilitar WAL mode para mejor concurrencia
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("error setting WAL mode: %w", err)
	}

	// Habilitar foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("error enabling foreign keys: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("error running migrations: %w", err)
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) migrate() error {
	migrations := []string{
		// Índice de código
		`CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY,
			path TEXT UNIQUE NOT NULL,
			package TEXT,
			module TEXT,
			layer TEXT,
			hash TEXT,
			updated_at INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS symbols (
			id INTEGER PRIMARY KEY,
			file_id INTEGER REFERENCES files(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			kind TEXT NOT NULL,
			signature TEXT,
			line INTEGER,
			feature TEXT
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS symbols_fts USING fts5(name, signature, package, path, doc)`,

		// Memoria del proyecto
		`CREATE TABLE IF NOT EXISTS knowledge_entries (
			id INTEGER PRIMARY KEY,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			tags TEXT,
			file_refs TEXT,
			status TEXT DEFAULT 'temp',
			created_at INTEGER
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS knowledge_fts USING fts5(title, content, tags)`,

		// Embeddings opcionales
		`CREATE TABLE IF NOT EXISTS embeddings (
			symbol_id INTEGER REFERENCES symbols(id) ON DELETE CASCADE,
			model TEXT,
			dim INTEGER,
			vector BLOB
		)`,

		// Clasificación semántica por archivo: el LLM revisa cada .kt/.kts
		// indexado, lo etiqueta (viewmodel, activity, usecase, ...) y
		// guarda convenciones y arquitectura detectada. El agente usa
		// esta tabla para localizar archivos existentes sin re-leer
		// todo el proyecto.
		`CREATE TABLE IF NOT EXISTS file_semantics (
			file_id INTEGER PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
			type TEXT,
			tags TEXT,
			architecture TEXT,
			conventions TEXT,
			summary TEXT,
			model TEXT,
			updated_at INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_file_semantics_type ON file_semantics(type)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS file_semantics_fts USING fts5(path, type, tags, architecture, summary, conventions)`,

		// Operativos
		`CREATE TABLE IF NOT EXISTS build_history (
			id INTEGER PRIMARY KEY,
			task TEXT,
			status TEXT,
			log TEXT,
			ts INTEGER
		)`,

		// Memoria del agente: sesiones de conversación persistentes
		`CREATE TABLE IF NOT EXISTS conversations (
			id INTEGER PRIMARY KEY,
			title TEXT,
			task TEXT NOT NULL,
			status TEXT DEFAULT 'active',
			approval_mode TEXT,
			provider TEXT,
			model TEXT,
			created_at INTEGER,
			updated_at INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY,
			conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			role TEXT NOT NULL,
			content TEXT,
			tool_calls TEXT,
			tool_call_id TEXT,
			tool_name TEXT,
			created_at INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, id)`,

		// Historial de entrevistas
		`CREATE TABLE IF NOT EXISTS interview_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task TEXT,
			score TEXT,
			total INTEGER,
			correct INTEGER,
			percentage REAL,
			grade TEXT,
			category TEXT,
			difficulty TEXT,
			created_at INTEGER
		)`,

		// Gestor de tareas (task queue)
		`CREATE TABLE IF NOT EXISTS tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT,
			priority INTEGER DEFAULT 0,
			status TEXT DEFAULT 'pending',
			type TEXT,
			result TEXT,
			error TEXT,
			conversation_id INTEGER,
			created_at INTEGER,
			updated_at INTEGER,
			started_at INTEGER,
			completed_at INTEGER
		)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("error executing migration: %w", err)
		}
	}

	return nil
}

func (s *Store) TableExists(tableName string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) VirtualTableExists(tableName string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=? AND sql LIKE '%fts5%'", tableName).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
