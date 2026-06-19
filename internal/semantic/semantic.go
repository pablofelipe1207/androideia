package semantic

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// SemanticProvider define el tipo de proveedor semántico.
type SemanticProvider string

const (
	ProviderOllama        SemanticProvider = "ollama"
	ProviderOpenAI        SemanticProvider = "openai"
	ProviderOpenCode      SemanticProvider = "opencode_zen"
	ProviderGoogleGemini  SemanticProvider = "google_gemini"
)

type EmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type EmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

type Semantic struct {
	db             *sql.DB
	ollamaURL      string
	model          string
	embeddingModel string
	apiKey         string
	provider       SemanticProvider
	chatProvider   SemanticProvider
	client         *http.Client
}

func NewSemantic(db *sql.DB, ollamaURL, model string) *Semantic {
	return NewSemanticWithProvider(db, ollamaURL, model, ProviderOllama)
}

func NewSemanticWithProvider(db *sql.DB, baseURL, model string, provider SemanticProvider) *Semantic {
	return &Semantic{
		db:        db,
		ollamaURL: baseURL,
		model:     model,
		provider:  provider,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func NewSemanticWithEmbeddingModel(db *sql.DB, baseURL, model, embeddingModel string, provider SemanticProvider) *Semantic {
	return &Semantic{
		db:             db,
		ollamaURL:      baseURL,
		model:          model,
		embeddingModel: embeddingModel,
		provider:       provider,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func NewSemanticWithAPIKey(db *sql.DB, baseURL, model, embeddingModel, apiKey string, provider SemanticProvider) *Semantic {
	// When using Google Gemini for embeddings, use OpenCode Zen for chat/classification
	// because Gemini free tier has strict rate limits
	chatProvider := provider
	if provider == ProviderGoogleGemini {
		chatProvider = ProviderOpenCode
	}
	return &Semantic{
		db:             db,
		ollamaURL:      baseURL,
		model:          model,
		embeddingModel: embeddingModel,
		apiKey:         apiKey,
		provider:       provider,
		chatProvider:   chatProvider,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (s *Semantic) IsAvailable() bool {
	// Check chat provider availability (used for classification)
	chatProv := s.chatProvider
	if chatProv == "" {
		chatProv = s.provider
	}
	
	switch chatProv {
	case ProviderOllama:
		resp, err := s.client.Get(s.ollamaURL + "/api/tags")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	case ProviderOpenAI, ProviderOpenCode:
		// Para providers OpenAI-compatible, verificar con /v1/models
		url := s.ollamaURL
		if url == "" {
			url = "https://opencode.ai/zen/v1"
		}
		resp, err := s.client.Get(url + "/models")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	case ProviderGoogleGemini:
		// Verify Gemini API key is set
		if s.apiKey == "" {
			return false
		}
		// Test with a simple models list request
		url := "https://generativelanguage.googleapis.com/v1beta/models?key=" + s.apiKey
		resp, err := s.client.Get(url)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	default:
		return false
	}
}

func (s *Semantic) GetEmbedding(text string) ([]float32, error) {
	switch s.provider {
	case ProviderOllama:
		return s.getOllamaEmbedding(text)
	case ProviderOpenCode, ProviderOpenAI:
		return s.getOpenCodeZenEmbedding(text)
	case ProviderGoogleGemini:
		return s.getGoogleGeminiEmbedding(text)
	default:
		return nil, fmt.Errorf("embeddings not supported for provider %s", s.provider)
	}
}

func (s *Semantic) getOllamaEmbedding(text string) ([]float32, error) {
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

func (s *Semantic) getOpenCodeZenEmbedding(text string) ([]float32, error) {
	// Determine base URL
	baseURL := s.ollamaURL
	if baseURL == "" {
		baseURL = "https://opencode.ai/zen/v1"
	}

	// Trim trailing slash
	for len(baseURL) > 0 && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}

	// Use embedding model if set, otherwise fall back to chat model
	model := s.embeddingModel
	if model == "" {
		model = s.model
	}

	// OpenAI-compatible embedding request
	request := map[string]interface{}{
		"model": model,
		"input": text,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling OpenCode Zen embeddings: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenCode Zen embeddings returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse OpenAI-compatible response
	var embResp struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &embResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling embedding response: %w", err)
	}
	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("OpenCode Zen embeddings returned no data")
	}
	return embResp.Data[0].Embedding, nil
}

func (s *Semantic) getGoogleGeminiEmbedding(text string) ([]float32, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("Google Gemini API key not configured")
	}

	// Use embedding model if set, otherwise default to gemini-embedding-001
	model := s.embeddingModel
	if model == "" {
		model = "gemini-embedding-001"
	}

	// Gemini API endpoint for embedContent
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:embedContent?key=%s", model, s.apiKey)

	// Build request body (Gemini format)
	request := map[string]interface{}{
		"model": fmt.Sprintf("models/%s", model),
		"content": map[string]interface{}{
			"parts": []map[string]string{
				{"text": text},
			},
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling Google Gemini embeddings: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google Gemini embeddings returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse Gemini response
	var embResp struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}
	if err := json.Unmarshal(body, &embResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling embedding response: %w", err)
	}
	if len(embResp.Embedding.Values) == 0 {
		return nil, fmt.Errorf("Google Gemini embeddings returned no data")
	}
	return embResp.Embedding.Values, nil
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

// DiscoverFeatures busca ViewModels en file_semantics (clasificación LLM),
// extrae el nombre base (quitando "ViewModel") y devuelve un mapa
// archivo→feature para todos los archivos relacionados.
// No usa LLM — es 100% heurístico sobre la clasificación existente.
func (s *Semantic) DiscoverFeatures() (map[string]string, error) {
	// 1) Buscar archivos clasificados como viewmodel en file_semantics
	rows, err := s.db.Query(`
		SELECT f.path, f.package
		FROM file_semantics fs
		JOIN files f ON f.id = fs.file_id
		WHERE fs.type = 'viewmodel'
	`)
	if err != nil {
		return nil, fmt.Errorf("error querying viewmodels from file_semantics: %w", err)
	}
	defer rows.Close()

	type vmInfo struct {
		Path    string
		Package string
	}
	var viewmodels []vmInfo
	for rows.Next() {
		var v vmInfo
		if err := rows.Scan(&v.Path, &v.Package); err != nil {
			return nil, fmt.Errorf("error scanning viewmodel: %w", err)
		}
		viewmodels = append(viewmodels, v)
	}

	if len(viewmodels) == 0 {
		return nil, nil
	}

	// 2) Para cada ViewModel, extraer nombre base y buscar archivos relacionados
	fileToFeature := make(map[string]string)
	for _, vm := range viewmodels {
		// Extraer nombre del archivo: CounterViewModel.kt -> CounterViewModel
		fileName := filepath.Base(vm.Path)
		fileName = strings.TrimSuffix(fileName, ".kt")
		fileName = strings.TrimSuffix(fileName, ".java")

		// Quitar "ViewModel" del nombre: CounterViewModel -> counter
		base := strings.TrimSuffix(fileName, "ViewModel")
		base = strings.TrimSuffix(base, "vm")
		feature := strings.ToLower(base)
		if feature == "" {
			continue
		}

		// Buscar archivos cuyo nombre empiece con el nombre base
		// (ej: CounterViewModel.kt, CounterEffect.kt, CounterEvent.kt -> feature "counter")
		nameRows, err := s.db.Query(`
			SELECT f.path FROM files f
			WHERE LOWER(f.path) LIKE '%' || LOWER(?) || '%'
		`, base)
		if err != nil {
			continue
		}
		for nameRows.Next() {
			var p string
			if nameRows.Scan(&p) == nil {
				fName := strings.ToLower(filepath.Base(p))
				baseLower := strings.ToLower(base)
				if strings.HasPrefix(fName, baseLower) {
					fileToFeature[p] = feature
				}
			}
		}
		nameRows.Close()
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
