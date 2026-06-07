package semantic

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

type EmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type EmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

type Semantic struct {
	db        *sql.DB
	ollamaURL string
	model     string
	client    *http.Client
}

func NewSemantic(db *sql.DB, ollamaURL, model string) *Semantic {
	return &Semantic{
		db:        db,
		ollamaURL: ollamaURL,
		model:     model,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *Semantic) IsAvailable() bool {
	resp, err := s.client.Get(s.ollamaURL + "/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (s *Semantic) GetEmbedding(text string) ([]float32, error) {
	request := EmbeddingRequest{
		Model:  s.model,
		Prompt: text,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	resp, err := s.client.Post(s.ollamaURL+"/api/embeddings", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error calling Ollama embeddings: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var embeddingResp EmbeddingResponse
	if err := json.Unmarshal(body, &embeddingResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w", err)
	}

	return embeddingResp.Embedding, nil
}

func (s *Semantic) StoreEmbedding(symbolID int64, model string, vector []float32) error {
	// Convert float32 slice to bytes (little-endian)
	vectorBytes := make([]byte, len(vector)*4)
	for i, v := range vector {
		bits := math.Float32bits(v)
		vectorBytes[i*4] = byte(bits)
		vectorBytes[i*4+1] = byte(bits >> 8)
		vectorBytes[i*4+2] = byte(bits >> 16)
		vectorBytes[i*4+3] = byte(bits >> 24)
	}

	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO embeddings (symbol_id, model, dim, vector) VALUES (?, ?, ?, ?)`,
		symbolID, model, len(vector), vectorBytes,
	)
	if err != nil {
		return fmt.Errorf("error storing embedding: %w", err)
	}

	return nil
}

func (s *Semantic) GetEmbeddingBySymbolID(symbolID int64) ([]float32, error) {
	var dim int
	var vectorBytes []byte

	err := s.db.QueryRow(
		`SELECT dim, vector FROM embeddings WHERE symbol_id = ?`,
		symbolID,
	).Scan(&dim, &vectorBytes)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("error getting embedding: %w", err)
	}

	// Convert bytes to float32 slice
	vector := make([]float32, dim)
	for i := 0; i < dim; i++ {
		vector[i] = math.Float32frombits(uint32(vectorBytes[i*4]) | uint32(vectorBytes[i*4+1])<<8 | uint32(vectorBytes[i*4+2])<<16 | uint32(vectorBytes[i*4+3])<<24)
	}

	return vector, nil
}

func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

type SearchResult struct {
	SymbolID   int64
	Name       string
	Kind       string
	Path       string
	Line       int
	Similarity float32
}

func (s *Semantic) Search(query string, limit int) ([]SearchResult, error) {
	// Get query embedding
	queryEmbedding, err := s.GetEmbedding(query)
	if err != nil {
		// Fallback to FTS if embeddings fail
		return s.searchFTS(query, limit)
	}

	// Get all embeddings
	rows, err := s.db.Query(
		`SELECT e.symbol_id, e.vector, e.dim, s.name, s.kind, f.path, s.line
		FROM embeddings e
		JOIN symbols s ON e.symbol_id = s.id
		JOIN files f ON s.file_id = f.id`,
	)
	if err != nil {
		return nil, fmt.Errorf("error querying embeddings: %w", err)
	}
	defer rows.Close()

	var results []SearchResult

	for rows.Next() {
		var symbolID int64
		var vectorBytes []byte
		var dim int
		var name, kind, path string
		var line int

		if err := rows.Scan(&symbolID, &vectorBytes, &dim, &name, &kind, &path, &line); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		// Convert bytes to float32 slice
		vector := make([]float32, dim)
		for i := 0; i < dim; i++ {
			vector[i] = math.Float32frombits(uint32(vectorBytes[i*4]) | uint32(vectorBytes[i*4+1])<<8 | uint32(vectorBytes[i*4+2])<<16 | uint32(vectorBytes[i*4+3])<<24)
		}

		// Calculate similarity
		similarity := CosineSimilarity(queryEmbedding, vector)

		results = append(results, SearchResult{
			SymbolID:   symbolID,
			Name:       name,
			Kind:       kind,
			Path:       path,
			Line:       line,
			Similarity: similarity,
		})
	}

	// Sort by similarity (descending)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Similarity < results[j].Similarity {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Limit results
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func (s *Semantic) searchFTS(query string, limit int) ([]SearchResult, error) {
	rows, err := s.db.Query(
		`SELECT s.id, s.name, s.kind, f.path, s.line
		FROM symbols_fts
		JOIN symbols s ON s.name = symbols_fts.name
		JOIN files f ON s.file_id = f.id
		WHERE symbols_fts MATCH ?
		ORDER BY rank
		LIMIT ?`,
		query, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("error searching FTS: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var symbolID int64
		var name, kind, path string
		var line int

		if err := rows.Scan(&symbolID, &name, &kind, &path, &line); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		results = append(results, SearchResult{
			SymbolID:   symbolID,
			Name:       name,
			Kind:       kind,
			Path:       path,
			Line:       line,
			Similarity: 1.0, // FTS results get max similarity
		})
	}

	return results, nil
}

func (s *Semantic) IndexAll() (int, error) {
	// Get all symbols
	rows, err := s.db.Query(
		`SELECT s.id, s.name, s.signature, s.kind
		FROM symbols s
		WHERE s.id NOT IN (SELECT symbol_id FROM embeddings)`,
	)
	if err != nil {
		return 0, fmt.Errorf("error querying symbols: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int64
		var name, signature, kind string

		if err := rows.Scan(&id, &name, &signature, &kind); err != nil {
			return 0, fmt.Errorf("error scanning row: %w", err)
		}

		// Create text for embedding
		text := fmt.Sprintf("%s %s %s", name, kind, signature)

		// Get embedding
		embedding, err := s.GetEmbedding(text)
		if err != nil {
			fmt.Printf("Warning: Error getting embedding for %s: %v\n", name, err)
			continue
		}

		// Store embedding
		if err := s.StoreEmbedding(id, s.model, embedding); err != nil {
			fmt.Printf("Warning: Error storing embedding for %s: %v\n", name, err)
			continue
		}

		count++
	}

	return count, nil
}

// FeatureInfo representa la información de una feature detectada
type FeatureInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
}

// DiscoverFeatures usa Ollama para analizar el código y descubrir
// features del proyecto. Agrupa archivos/símbolos en features coherentes.
func (s *Semantic) DiscoverFeatures() (map[string]string, error) {
	// Obtener todos los archivos con sus símbolos
	rows, err := s.db.Query(`
		SELECT f.path, f.package, f.module, f.layer,
		       GROUP_CONCAT(s.name || ':' || s.kind, ';') as symbols
		FROM files f
		LEFT JOIN symbols s ON s.file_id = f.id
		GROUP BY f.id, f.path, f.package, f.module, f.layer
		ORDER BY f.path
	`)
	if err != nil {
		return nil, fmt.Errorf("error querying files: %w", err)
	}
	defer rows.Close()

	type FileInfo struct {
		Path    string
		Package string
		Module  string
		Layer   string
		Symbols string
	}

	var files []FileInfo
	for rows.Next() {
		var fi FileInfo
		if err := rows.Scan(&fi.Path, &fi.Package, &fi.Module, &fi.Layer, &fi.Symbols); err != nil {
			return nil, fmt.Errorf("error scanning file: %w", err)
		}
		files = append(files, fi)
	}

	if len(files) == 0 {
		return nil, nil
	}

	// Construir prompt para Ollama
	var prompt strings.Builder
	prompt.WriteString("Eres un experto en arquitectura Android (MVVM, Jetpack Compose, Hilt, Room).\n")
	prompt.WriteString("Analiza este proyecto y agrupa los archivos en FEATURES (funcionalidades completas).\n\n")
	prompt.WriteString("CRITERIOS para detectar una feature:\n")
	prompt.WriteString("- Una feature TIENE al menos: Screen/Composable + ViewModel + UseCase/Repository\n")
	prompt.WriteString("- Patrones típicos MVVM:\n")
	prompt.WriteString("  * Screen: *Screen.kt, *Composable.kt (UI con @Composable)\n")
	prompt.WriteString("  * ViewModel: *ViewModel.kt (con @HiltViewModel, UiState/UiEvent)\n")
	prompt.WriteString("  * UseCase: *UseCase.kt (operator invoke, @Inject)\n")
	prompt.WriteString("  * Repository: *Repository.kt (interface + Impl, sin android.*)\n")
	prompt.WriteString("  * Module: *Module.kt (@Module @InstallIn Hilt)\n")
	prompt.WriteString("  * Route: *Routes.kt (composable routes)\n\n")
	prompt.WriteString("Devuelve SOLO JSON válido con este formato:\n")
	prompt.WriteString(`{"features": [{"name": "login", "description": "Autenticación de usuarios (login, registro, recuperación)", "files": ["app/src/main/java/.../ui/login/LoginScreen.kt", "app/src/main/java/.../ui/login/LoginViewModel.kt", "app/src/main/java/.../domain/usecase/LoginUseCase.kt", "app/src/main/java/.../data/repository/UserRepository.kt"]}]}\n\n`)
	prompt.WriteString("Archivos y símbolos:\n\n")

	for _, fi := range files {
		prompt.WriteString(fmt.Sprintf("Archivo: %s\n", fi.Path))
		prompt.WriteString(fmt.Sprintf("  Paquete: %s\n", fi.Package))
		prompt.WriteString(fmt.Sprintf("  Módulo: %s\n", fi.Module))
		prompt.WriteString(fmt.Sprintf("  Capa: %s\n", fi.Layer))
		if fi.Symbols != "" {
			prompt.WriteString(fmt.Sprintf("  Símbolos: %s\n", fi.Symbols))
		}
		prompt.WriteString("\n")
	}

	// Llamar a Ollama chat API
	request := map[string]interface{}{
		"model":  s.model,
		"prompt": prompt.String(),
		"stream": false,
		"format": "json",
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	resp, err := s.client.Post(s.ollamaURL+"/api/generate", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error calling Ollama: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parsear respuesta (Ollama devuelve JSON por líneas en streaming, pero con stream=false es un solo JSON)
	var ollamaResp struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling Ollama response: %w", err)
	}

	// Parsear el JSON de la respuesta
	var result struct {
		Features []FeatureInfo `json:"features"`
	}
	if err := json.Unmarshal([]byte(ollamaResp.Response), &result); err != nil {
		return nil, fmt.Errorf("error parsing features JSON: %w", err)
	}

	// Construir mapa archivo -> feature
	fileToFeature := make(map[string]string)
	for _, feat := range result.Features {
		for _, file := range feat.Files {
			fileToFeature[file] = feat.Name
		}
	}

	return fileToFeature, nil
}

// TagSymbolsWithFeatures etiqueta los símbolos con sus features usando
// el descubrimiento LLM. Devuelve el número de símbolos etiquetados.
func (s *Semantic) TagSymbolsWithFeatures(fileToFeature map[string]string) (int, error) {
	if len(fileToFeature) == 0 {
		return 0, nil
	}

	count := 0
	for filePath, featureName := range fileToFeature {
		result, err := s.db.Exec(`
			UPDATE symbols
			SET feature = ?
			WHERE file_id IN (SELECT id FROM files WHERE path = ?)
		`, featureName, filePath)
		if err != nil {
			continue
		}
		affected, _ := result.RowsAffected()
		count += int(affected)
	}

	return count, nil
}
