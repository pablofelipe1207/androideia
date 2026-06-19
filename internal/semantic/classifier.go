package semantic

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Tipos canónicos que el clasificador puede asignar. Se mantienen cortos
// para que el prompt del LLM no se descontrole y para que el agente
// pueda filtrar por tipo con confianza.
var AllowedTypes = map[string]bool{
	"viewmodel":   true,
	"activity":    true,
	"composable":  true,
	"usecase":     true,
	"repository":  true,
	"dao":         true,
	"di_module":   true,
	"nav_route":   true,
	"data_class":  true,
	"entity":      true,
	"service":     true,
	"application": true,
	"test":        true,
	"build":       true,
	"other":       true,
}

// FileSemantic es la fila que el agente y la CLI consumen cuando
// preguntan "¿dónde está X?", "¿cómo se escriben los ViewModels?", etc.
type FileSemantic struct {
	FileID       int64    `json:"file_id"`
	Path         string   `json:"path"`
	Package      string   `json:"package"`
	Module       string   `json:"module"`
	Layer        string   `json:"layer"`
	Type         string   `json:"type"`
	Tags         []string `json:"tags"`
	Architecture string   `json:"architecture"`
	Conventions  string   `json:"conventions"`
	Summary      string   `json:"summary"`
	Model        string   `json:"model"`
	UpdatedAt    int64    `json:"updated_at"`
}

// classificationResult es la forma JSON que pedimos al LLM. Vive en
// este archivo (no en semantic.go) porque sólo la usa el clasificador.
type classificationResult struct {
	Type         string   `json:"type"`
	Tags         []string `json:"tags"`
	Architecture string   `json:"architecture"`
	Conventions  string   `json:"conventions"`
	Summary      string   `json:"summary"`
}

// chatMessage es el subconjunto de Message que necesitamos para
// /api/chat; lo declaramos local para no acoplarnos al paquete llm.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatEnvelope es el cuerpo de /api/chat en formato Ollama.
type chatEnvelope struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Format   string        `json:"format,omitempty"`
}

// openaiMessage es el formato de mensaje para la API de OpenAI-compatible.
type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openaiRequest es el cuerpo de la petición para /v1/chat/completions.
type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
}

// openaiResponse es la respuesta de la API de OpenAI-compatible.
type openaiResponse struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type chatResponse struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// maxFileBytes para enviar al LLM. 6 KB es suficiente para que el modelo
// "vea" los imports, el package, las anotaciones, la firma de la clase y
// las primeras funciones. Archivos más grandes se truncan con un
// marcador explícito.
const maxFileBytes = 6000

// classifyPromptTemplate es el prompt que se le envía al LLM. El
// marcador %s aparece dos veces: primero la ruta del archivo, después
// el contenido. Pedimos JSON estricto (sin fences) y limitamos el
// tamaño de los campos para que la respuesta no explote el contexto
// cuando iteramos sobre cientos de archivos.
const classifyPromptTemplate = `You are an expert Android/Kotlin code classifier.

Task: classify the file at path "%s" and return ONLY a strict JSON object
(no markdown, no code fence, no prose before/after).

Required JSON shape (every key is mandatory, return {} for any field you
cannot infer):
{
  "type":        "<one of: viewmodel | activity | composable | usecase | repository | dao | di_module | nav_route | data_class | entity | service | application | test | build | other>",
  "tags":        ["<kebab-case>", "..."],
  "architecture":"<MVVM | MVI | Clean | MVP | unknown | other>",
  "conventions": "<one short paragraph (max 220 chars) describing how this file is written: DI style, state holder, async, navigation, etc.>",
  "summary":     "<one sentence (8-22 words) describing the file's purpose>"
}

Rules:
- type MUST be lowercase and one of the values listed above.
- tags: 1 to 6 entries, lowercase, kebab-case. Focus on the FEATURE/ROLE
  (e.g. "login", "auth", "hilt-injection", "stateflow", "navigation",
  "room", "retrofit", "pagination"). Avoid generic words like "kotlin".
- architecture: try to infer the project-level pattern from the file
  itself (e.g. if the file is a ViewModel exposing StateFlow, that hints
  at MVVM). Use "unknown" when you cannot tell.
- conventions: be SPECIFIC. Examples of good values:
    "Hilt constructor injection, exposes StateFlow<UiState>, no Android dependencies"
    "@Composable with no ViewModel; consumes data via collectAsStateWithLifecycle"
    "Singleton object with @Provides methods for Hilt SingletonComponent"
- summary: imperative mood, no file extension in the name.

File content (may be truncated):
"""%s"""
`

// ClassifyFile llama al LLM por chat (no embeddings) para clasificar un
// único archivo. Devuelve un classificationResult saneado: type se
// normaliza a uno de AllowedTypes y tags se limpia.
func (s *Semantic) ClassifyFile(path, content string) (*classificationResult, error) {
	truncated := content
	if len(truncated) > maxFileBytes {
		truncated = truncated[:maxFileBytes] + "\n// ... (truncated)"
	}

	prompt := fmt.Sprintf(classifyPromptTemplate, path, truncated)

	// Use chatProvider for classification (may differ from provider for embeddings)
	chatProv := s.chatProvider
	if chatProv == "" {
		chatProv = s.provider
	}

	// Seleccionar el provider apropiado
	switch chatProv {
	case ProviderOpenAI, ProviderOpenCode:
		return s.classifyFileOpenAI(prompt)
	case ProviderGoogleGemini:
		return s.classifyFileGoogleGemini(prompt)
	default:
		return s.classifyFileOllama(prompt)
	}
}

// classifyFileOllama usa la API de Ollama para clasificar.
func (s *Semantic) classifyFileOllama(prompt string) (*classificationResult, error) {
	envelope := chatEnvelope{
		Model: s.model,
		Messages: []chatMessage{
			{Role: "system", Content: "You are an expert Android code classifier. You always reply with strict JSON only, no markdown, no prose."},
			{Role: "user", Content: prompt},
		},
		Stream: false,
		// "json" fuerza al modelo a devolver JSON válido (Ollama >=0.5).
		Format: "json",
	}

	body, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("error marshaling chat envelope: %w", err)
	}

	// Reusamos el http.Client del Semantic pero con un timeout más
	// generoso: clasificar un archivo puede tardar varios segundos con
	// modelos 7B en CPU. Si el cliente ya tiene un timeout corto, lo
	// duplicamos para no afectar a GetEmbedding.
	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Post(s.ollamaURL+"/api/chat", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("error calling Ollama chat: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading chat response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama chat returned %d: %s", resp.StatusCode, string(raw))
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return nil, fmt.Errorf("error unmarshaling chat response: %w (raw: %s)", err, string(raw))
	}

	parsed, err := parseClassificationContent(cr.Message.Content)
	if err != nil {
		return nil, err
	}
	sanitizeClassification(parsed)
	return parsed, nil
}

// classifyFileOpenAI usa la API OpenAI-compatible para clasificar.
func (s *Semantic) classifyFileOpenAI(prompt string) (*classificationResult, error) {
	req := openaiRequest{
		Model: s.model,
		Messages: []openaiMessage{
			{Role: "system", Content: "You are an expert Android code classifier. You always reply with strict JSON only, no markdown, no prose."},
			{Role: "user", Content: prompt},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("error marshaling openai request: %w", err)
	}

	// Determinar la URL base
	baseURL := s.ollamaURL
	if baseURL == "" {
		baseURL = "https://opencode.ai/zen/v1"
	}

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Post(baseURL+"/chat/completions", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("error calling OpenAI-compatible API: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(raw))
	}

	var oaiResp openaiResponse
	if err := json.Unmarshal(raw, &oaiResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w (raw: %s)", err, string(raw))
	}

	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	parsed, err := parseClassificationContent(oaiResp.Choices[0].Message.Content)
	if err != nil {
		return nil, err
	}
	sanitizeClassification(parsed)
	return parsed, nil
}

// classifyFileGoogleGemini usa la API de Google Gemini para clasificar.
func (s *Semantic) classifyFileGoogleGemini(prompt string) (*classificationResult, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("Google Gemini API key not configured")
	}

	// Use chat model if set, otherwise default to gemini-2.0-flash
	model := s.model
	if model == "" {
		model = "gemini-2.0-flash"
	}

	// Gemini API endpoint for generateContent
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, s.apiKey)

	// Build request body (Gemini format)
	request := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": "You are an expert Android code classifier. You always reply with strict JSON only, no markdown, no prose.\n\n" + prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"responseMimeType": "application/json",
		},
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	client := &http.Client{Timeout: 90 * time.Second}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("error building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling Google Gemini API: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google Gemini API returned %d: %s", resp.StatusCode, string(raw))
	}

	// Parse Gemini response
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &geminiResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w (raw: %s)", err, string(raw))
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no candidates in Gemini response")
	}

	content := geminiResp.Candidates[0].Content.Parts[0].Text
	parsed, err := parseClassificationContent(content)
	if err != nil {
		return nil, err
	}
	sanitizeClassification(parsed)
	return parsed, nil
}

// parseClassificationContent es tolerante: a veces el LLM aún devuelve
// fences ```json ... ``` aunque se lo pidamos estricto, o mete texto
// antes/después. Buscamos el primer '{' y el último '}' balanceados.
func parseClassificationContent(content string) (*classificationResult, error) {
	cleaned := strings.TrimSpace(content)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```JSON")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	// Si el modelo metió prosa, recortamos hasta el primer '{' y desde
	// el último '}' para quedarnos sólo con el objeto JSON.
	if first := strings.Index(cleaned, "{"); first >= 0 {
		if last := strings.LastIndex(cleaned, "}"); last > first {
			cleaned = cleaned[first : last+1]
		}
	}

	var out classificationResult
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		return nil, fmt.Errorf("error parsing classifier JSON: %w (raw: %q)", err, cleaned)
	}
	return &out, nil
}

// sanitizeClassification aplica las invariantes que el resto del código
// asume: type ∈ AllowedTypes, tags en kebab-case, sin vacíos, máximo 6.
func sanitizeClassification(r *classificationResult) {
	r.Type = strings.ToLower(strings.TrimSpace(r.Type))
	if _, ok := AllowedTypes[r.Type]; !ok {
		r.Type = "other"
	}
	r.Architecture = strings.TrimSpace(r.Architecture)
	if r.Architecture == "" {
		r.Architecture = "unknown"
	}
	r.Conventions = strings.TrimSpace(r.Conventions)
	r.Summary = strings.TrimSpace(r.Summary)

	cleaned := make([]string, 0, len(r.Tags))
	for _, t := range r.Tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		t = kebabify(t)
		if t == "" {
			continue
		}
		cleaned = append(cleaned, t)
		if len(cleaned) >= 6 {
			break
		}
	}
	if len(cleaned) == 0 {
		cleaned = []string{"untagged"}
	}
	r.Tags = cleaned
}

// kebabify convierte "kebab-case" / "snake_case" / "camelCase" /
// "CamelCase" / "with spaces" a kebab-case ASCII.
func kebabify(s string) string {
	var b strings.Builder
	prevLower := false
	for i, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			if i > 0 && prevLower {
				b.WriteRune('-')
			}
			b.WriteRune(r + 32)
			prevLower = false
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevLower = true
		default:
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
				b.WriteRune('-')
			}
			prevLower = false
		}
	}
	out := strings.Trim(b.String(), "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	return out
}

// StoreFileSemantic persiste la clasificación. Si la fila ya existe
// (mismo file_id) la sobreescribe: así `androideai semantic index` es
// idempotente y puede re-ejecutarse cuando cambia el código.
func (s *Semantic) StoreFileSemantic(fileID int64, r *classificationResult) error {
	tagsJSON, err := json.Marshal(r.Tags)
	if err != nil {
		return fmt.Errorf("error marshaling tags: %w", err)
	}

	now := time.Now().Unix()

	_, err = s.db.Exec(
		`INSERT INTO file_semantics (file_id, type, tags, architecture, conventions, summary, model, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(file_id) DO UPDATE SET
		   type         = excluded.type,
		   tags         = excluded.tags,
		   architecture = excluded.architecture,
		   conventions  = excluded.conventions,
		   summary      = excluded.summary,
		   model        = excluded.model,
		   updated_at   = excluded.updated_at`,
		fileID, r.Type, string(tagsJSON), r.Architecture, r.Conventions, r.Summary, s.model, now,
	)
	if err != nil {
		return fmt.Errorf("error storing file_semantic: %w", err)
	}

	// Espejo en la tabla FTS para búsquedas rápidas tipo LIKE/FTS.
	_, err = s.db.Exec(
		`INSERT INTO file_semantics_fts (path, type, tags, architecture, summary, conventions)
		 SELECT f.path, ?, ?, ?, ?, ?
		 FROM files f WHERE f.id = ?`,
		r.Type, string(tagsJSON), r.Architecture, r.Summary, r.Conventions, fileID,
	)
	if err != nil {
		return fmt.Errorf("error inserting file_semantic into FTS: %w", err)
	}

	return nil
}

// ClassifyAllFiles recorre todos los archivos del índice y los manda
// uno a uno al LLM. Devuelve cuántos se clasificaron con éxito y
// cuántos fallaron. Es seguro re-ejecutarlo: sobreescribe filas
// existentes.
//
// Esta función es la que `androideai semantic index` invoca antes de
// generar los embeddings.
func (s *Semantic) ClassifyAllFiles() (classified, failed int, err error) {
	rows, err := s.db.Query(
		`SELECT id, path FROM files ORDER BY id`,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("error querying files: %w", err)
	}
	defer rows.Close()

	type pending struct {
		id   int64
		path string
	}
	var batch []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.path); err != nil {
			return 0, 0, fmt.Errorf("error scanning file row: %w", err)
		}
		batch = append(batch, p)
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}
	rows.Close()

	for _, p := range batch {
		content, err := os.ReadFile(p.path)
		if err != nil {
			failed++
			fmt.Printf("  ⚠️  cannot read %s: %v\n", p.path, err)
			continue
		}

		result, err := s.ClassifyFile(p.path, string(content))
		if err != nil {
			failed++
			fmt.Printf("  ⚠️  classify failed for %s: %v\n", p.path, err)
			continue
		}

		if err := s.StoreFileSemantic(p.id, result); err != nil {
			failed++
			fmt.Printf("  ⚠️  store failed for %s: %v\n", p.path, err)
			continue
		}

		classified++
		fmt.Printf("  ✓ %s → %s [%s]\n", p.path, result.Type, strings.Join(result.Tags, ","))
	}

	return classified, failed, nil
}

// FileLocation es el resultado de Locate: una coincidencia con la
// metadata semántica. Sólo exponemos lo que el agente necesita para
// decidir si abrir o no el archivo.
type FileLocation struct {
	FileSemantic
	MatchReason string `json:"match_reason"`
}

// Locate busca en file_semantics archivos que coincidan con query. La
// búsqueda es tolerante: si la query es exactamente un tipo conocido
// (p.ej. "viewmodel") filtra por type; si parece un nombre de clase
// (empieza con mayúscula) busca por path; en otros casos hace LIKE
// sobre type/tags/architecture/summary/conventions/path.
//
// Devuelve hasta `limit` resultados ordenados por relevancia
// aproximada.
func (s *Semantic) Locate(query string, limit int) ([]FileLocation, error) {
	if limit <= 0 {
		limit = 10
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}

	qLower := strings.ToLower(q)

	var (
		rows *sql.Rows
		err  error
	)

	switch {
	case AllowedTypes[qLower]:
		rows, err = s.db.Query(
			`SELECT fs.file_id, fs.type, fs.tags, fs.architecture, fs.conventions, fs.summary, fs.model, fs.updated_at,
			        f.path, f.package, f.module, f.layer
			 FROM file_semantics fs
			 JOIN files f ON f.id = fs.file_id
			 WHERE fs.type = ?
			 ORDER BY f.path
			 LIMIT ?`,
			qLower, limit,
		)
	case strings.HasPrefix(query, "@") || strings.HasPrefix(qLower, "tag:"):
		tag := strings.TrimPrefix(strings.TrimPrefix(qLower, "tag:"), "@")
		rows, err = s.db.Query(
			`SELECT fs.file_id, fs.type, fs.tags, fs.architecture, fs.conventions, fs.summary, fs.model, fs.updated_at,
			        f.path, f.package, f.module, f.layer
			 FROM file_semantics fs
			 JOIN files f ON f.id = fs.file_id
			 WHERE LOWER(fs.tags) LIKE ?
			 ORDER BY f.path
			 LIMIT ?`,
			"%"+tag+"%", limit,
		)
	default:
		like := "%" + qLower + "%"
		rows, err = s.db.Query(
			`SELECT fs.file_id, fs.type, fs.tags, fs.architecture, fs.conventions, fs.summary, fs.model, fs.updated_at,
			        f.path, f.package, f.module, f.layer
			 FROM file_semantics fs
			 JOIN files f ON f.id = fs.file_id
			 WHERE LOWER(f.path)  LIKE ?
			    OR LOWER(fs.type)        LIKE ?
			    OR LOWER(fs.tags)        LIKE ?
			    OR LOWER(fs.architecture)LIKE ?
			    OR LOWER(fs.summary)     LIKE ?
			    OR LOWER(fs.conventions) LIKE ?
			 ORDER BY
			    CASE WHEN LOWER(f.path) LIKE ? THEN 0 ELSE 1 END,
			    LENGTH(f.path)
			 LIMIT ?`,
			like, like, like, like, like, like, like, limit,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("error locating files: %w", err)
	}
	defer rows.Close()

	var out []FileLocation
	for rows.Next() {
		var loc FileLocation
		var tagsJSON string
		if err := rows.Scan(
			&loc.FileID, &loc.Type, &tagsJSON, &loc.Architecture,
			&loc.Conventions, &loc.Summary, &loc.Model, &loc.UpdatedAt,
			&loc.Path, &loc.Package, &loc.Module, &loc.Layer,
		); err != nil {
			return nil, fmt.Errorf("error scanning location: %w", err)
		}
		if err := json.Unmarshal([]byte(tagsJSON), &loc.Tags); err != nil {
			loc.Tags = []string{tagsJSON}
		}
		loc.MatchReason = matchReasonFor(qLower, q, &loc)
		out = append(out, loc)
	}
	return out, rows.Err()
}

func matchReasonFor(qLower, qOriginal string, loc *FileLocation) string {
	path := strings.ToLower(loc.Path)
	switch {
	case AllowedTypes[qLower] && loc.Type == qLower:
		return fmt.Sprintf("type=%s", loc.Type)
	case strings.Contains(path, qLower):
		return "path match"
	case len(loc.Tags) > 0 && strings.Contains(strings.ToLower(strings.Join(loc.Tags, ",")), qLower):
		return "tag match"
	case strings.Contains(strings.ToLower(loc.Architecture), qLower):
		return "architecture match"
	case strings.Contains(strings.ToLower(loc.Summary), qLower):
		return "summary match"
	case strings.Contains(strings.ToLower(loc.Conventions), qLower):
		return "conventions match"
	default:
		return fmt.Sprintf("query=%q", qOriginal)
	}
}

// ArchitectureSummary agrega los valores distintos de architecture
// presentes en file_semantics para responder "¿qué arquitectura usa el
// proyecto?". Si todo coincide devuelve ese único valor; si hay
// varios, devuelve los ordenados por frecuencia.
func (s *Semantic) ArchitectureSummary() (string, []string, error) {
	rows, err := s.db.Query(
		`SELECT architecture, COUNT(*) AS n
		 FROM file_semantics
		 WHERE architecture IS NOT NULL AND architecture != '' AND architecture != 'unknown'
		 GROUP BY architecture
		 ORDER BY n DESC`,
	)
	if err != nil {
		return "", nil, fmt.Errorf("error aggregating architecture: %w", err)
	}
	defer rows.Close()

	var (
		arches []string
		total  int
	)
	for rows.Next() {
		var a string
		var n int
		if err := rows.Scan(&a, &n); err != nil {
			return "", nil, err
		}
		arches = append(arches, a)
		total += n
	}
	if err := rows.Err(); err != nil {
		return "", nil, err
	}
	if len(arches) == 0 {
		return "unknown", nil, nil
	}
	if len(arches) == 1 {
		return arches[0], arches, nil
	}
	return strings.Join(arches, " / "), arches, nil
}
