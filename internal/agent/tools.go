package agent

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/pablofelipe1207/androideia/internal/android"
	"github.com/pablofelipe1207/androideia/internal/brain"
	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/pablofelipe1207/androideia/internal/index"
	"github.com/pablofelipe1207/androideia/internal/llm"
	"github.com/pablofelipe1207/androideia/internal/semantic"
)

type ToolRegistry struct {
	tools   []llm.Tool
	db      *sql.DB
	android *android.Android
	semantic *semantic.Semantic
}

func NewToolRegistry(db *sql.DB) *ToolRegistry {
	cfg, _ := config.LoadConfig()
	
	var sem *semantic.Semantic
	if cfg != nil {
		sem = semantic.NewSemantic(db, cfg.OllamaURL, cfg.Model)
	}
	
	registry := &ToolRegistry{
		db:      db,
		android: android.NewAndroid(db),
		semantic: sem,
	}
	registry.registerDefaultTools()
	return registry
}

func (r *ToolRegistry) registerDefaultTools() {
	r.tools = []llm.Tool{
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "read_file",
				Description: "Read the contents of a file",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Path to the file to read",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "write_file",
				Description: "Write content to a file (requires approval)",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Path to the file to write",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "Content to write to the file",
						},
					},
					"required": []string{"path", "content"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "index_search",
				Description: "Search the code index for symbols or keywords",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Search query",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "index_feature",
				Description: "Get all layers of a feature",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Feature name",
						},
					},
					"required": []string{"name"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "brain_search",
				Description: "Search project knowledge base",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Search query",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "gradle",
				Description: "Execute a Gradle task",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"task": map[string]interface{}{
							"type":        "string",
							"description": "Gradle task to execute",
						},
					},
					"required": []string{"task"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "test",
				Description: "Run Android tests",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{
							"type":        "string",
							"description": "Test type: unit or instrumented",
							"enum":        []string{"unit", "instrumented"},
						},
					},
					"required": []string{"type"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "emulator",
				Description: "Manage Android emulator (list, start, stop, status, install, launch)",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action": map[string]interface{}{
							"type":        "string",
							"description": "Action to perform: list, start, stop, status, install, launch",
							"enum":        []string{"list", "start", "stop", "status", "install", "launch"},
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Emulator name (for start action)",
						},
						"apk_path": map[string]interface{}{
							"type":        "string",
							"description": "APK path (for install action)",
						},
						"package_name": map[string]interface{}{
							"type":        "string",
							"description": "Package name (for launch action)",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "semantic_search",
				Description: "Search code by meaning using semantic embeddings. Use this to find similar code patterns, implementations, or understand code structure.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Search query describing what you're looking for (e.g., 'user login implementation', 'data class for user')",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum number of results (default: 10)",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "confirm_plan",
				Description: "Solicita confirmación explícita al usuario antes de proceder con un plan, cambio destructivo o escritura de archivos. SIEMPRE usa esta herramienta cuando vayas a escribir código, modificar archivos o ejecutar una acción importante, en lugar de pedir confirmación en texto plano. Devuelve 'approved' (con cualquier feedback opcional), 'denied' si el usuario rechaza, o 'edit:<nuevo_plan>' si el usuario pide ajustes.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"plan": map[string]interface{}{
							"type":        "string",
							"description": "Descripción clara y detallada del plan que se va a ejecutar. Incluye archivos a crear/modificar y el razonamiento.",
						},
					},
					"required": []string{"plan"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "ask_user",
				Description: "Hace una pregunta al usuario y espera su respuesta en texto libre. Úsala para pedir aclaraciones, decisiones de diseño o feedback. La respuesta del usuario se devuelve como string.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"question": map[string]interface{}{
							"type":        "string",
							"description": "La pregunta que se va a hacer al usuario.",
						},
					},
					"required": []string{"question"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "find_similar_files",
				Description: "Find similar files to understand project structure and where to place new files",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file_type": map[string]interface{}{
							"type":        "string",
							"description": "Type of file to find (e.g., 'ViewModel', 'Repository', 'UseCase', 'Screen')",
						},
					},
					"required": []string{"file_type"},
				},
			},
		},
	}
}

func (r *ToolRegistry) GetTools() []llm.Tool {
	return r.tools
}

func (r *ToolRegistry) ExecuteTool(name string, args map[string]interface{}) (string, error) {
	switch name {
	case "read_file":
		return r.readFile(args)
	case "write_file":
		return r.writeFile(args)
	case "index_search":
		return r.indexSearch(args)
	case "index_feature":
		return r.indexFeature(args)
	case "brain_search":
		return r.brainSearch(args)
	case "gradle":
		return r.gradle(args)
	case "test":
		return r.test(args)
	case "emulator":
		return r.emulator(args)
	case "semantic_search":
		return r.semanticSearch(args)
	case "find_similar_files":
		return r.findSimilarFiles(args)
	case "confirm_plan":
		return r.confirmPlan(args)
	case "ask_user":
		return r.askUser(args)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func (r *ToolRegistry) readFile(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path is required")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	return string(content), nil
}

func (r *ToolRegistry) writeFile(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path is required")
	}

	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content is required")
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("error creating directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("error writing file: %w", err)
	}

	// Index the new file in semantic search
	r.indexNewFile(path, content)

	return fmt.Sprintf("File written: %s", path), nil
}

func (r *ToolRegistry) indexNewFile(path, content string) {
	if r.db == nil || r.semantic == nil {
		return
	}

	// Extract file metadata
	extractor := index.NewTreeSitterExtractor()
	package_name := extractor.InferPackage(content)
	module := extractor.InferModule(path)
	layer := extractor.InferLayer(path, content)

	// Insert file
	result, err := r.db.Exec(
		"INSERT OR REPLACE INTO files (path, package, module, layer, hash, updated_at) VALUES (?, ?, ?, ?, ?, strftime('%s', 'now'))",
		path, package_name, module, layer, "",
	)
	if err != nil {
		return
	}

	fileID, err := result.LastInsertId()
	if err != nil {
		return
	}

	// Extract symbols
	symbols := extractor.ExtractSymbols(path, content)
	for _, sym := range symbols {
		r.db.Exec(
			"INSERT INTO symbols (file_id, name, kind, signature, line, feature) VALUES (?, ?, ?, ?, ?, ?)",
			fileID, sym.Name, sym.Kind, sym.Signature, sym.Line, sym.Feature,
		)

		r.db.Exec(
			"INSERT INTO symbols_fts (name, signature, package, path, doc) VALUES (?, ?, ?, ?, ?)",
			sym.Name, sym.Signature, package_name, path, sym.Signature,
		)
	}
}

func (r *ToolRegistry) indexSearch(args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok {
		return "", fmt.Errorf("query is required")
	}

	// Search using FTS
	rows, err := r.db.Query(
		`SELECT f.path, s.line, s.name, s.kind
		FROM symbols_fts
		JOIN symbols s ON s.name = symbols_fts.name
		JOIN files f ON s.file_id = f.id
		WHERE symbols_fts MATCH ?
		ORDER BY rank
		LIMIT 10`,
		query,
	)
	if err != nil {
		return "", fmt.Errorf("error searching: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var path, name, kind string
		var line int
		if err := rows.Scan(&path, &line, &name, &kind); err != nil {
			return "", fmt.Errorf("error scanning row: %w", err)
		}
		results = append(results, fmt.Sprintf("%s:%d %s (%s)", path, line, name, kind))
	}

	if len(results) == 0 {
		return "No results found", nil
	}

	return strings.Join(results, "\n"), nil
}

func (r *ToolRegistry) indexFeature(args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok {
		return "", fmt.Errorf("name is required")
	}

	feature, err := index.GetFeatureByName(r.db, name)
	if err != nil {
		return "", fmt.Errorf("error getting feature: %w", err)
	}

	return feature.Format(), nil
}

func (r *ToolRegistry) brainSearch(args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok {
		return "", fmt.Errorf("query is required")
	}

	b := brain.NewBrain(r.db)
	entries, err := b.Search(query)
	if err != nil {
		return "", fmt.Errorf("error searching: %w", err)
	}

	if len(entries) == 0 {
		return "No knowledge entries found", nil
	}

	var results []string
	for _, entry := range entries {
		results = append(results, fmt.Sprintf("[%s] %s: %s", entry.Type, entry.Title, entry.Content))
	}

	return strings.Join(results, "\n"), nil
}

func (r *ToolRegistry) gradle(args map[string]interface{}) (string, error) {
	task, ok := args["task"].(string)
	if !ok {
		return "", fmt.Errorf("task is required")
	}

	result, err := r.android.Gradle(task)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Build %s: %s\nDuration: %s\n%s", result.Status, result.Task, result.Duration, result.Log), nil
}

func (r *ToolRegistry) test(args map[string]interface{}) (string, error) {
	testType, ok := args["type"].(string)
	if !ok {
		return "", fmt.Errorf("type is required")
	}

	unit := testType == "unit"
	result, err := r.android.Test(unit)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Tests %s: %s\nDuration: %s\n%s", result.Status, result.Task, result.Duration, result.Log), nil
}

func (r *ToolRegistry) emulator(args map[string]interface{}) (string, error) {
	action, ok := args["action"].(string)
	if !ok {
		return "", fmt.Errorf("action is required")
	}

	var result string
	var err error

	switch action {
	case "list":
		result, err = r.android.Emulator("list")
	case "start":
		name, _ := args["name"].(string)
		if name == "" {
			return "", fmt.Errorf("emulator name required for start")
		}
		result, err = r.android.Emulator("start", name)
	case "stop":
		result, err = r.android.Emulator("stop")
	case "status":
		result, err = r.android.Emulator("status")
	case "install":
		apkPath, _ := args["apk_path"].(string)
		if apkPath == "" {
			return "", fmt.Errorf("APK path required for install")
		}
		result, err = r.android.Emulator("install", apkPath)
	case "launch":
		packageName, _ := args["package_name"].(string)
		if packageName == "" {
			return "", fmt.Errorf("package name required for launch")
		}
		result, err = r.android.Emulator("launch", packageName)
	default:
		return "", fmt.Errorf("unknown emulator action: %s", action)
	}

	if err != nil {
		return "", err
	}

	return result, nil
}

func (r *ToolRegistry) semanticSearch(args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok {
		return "", fmt.Errorf("query is required")
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	if r.semantic == nil {
		return "", fmt.Errorf("semantic search not available. Make sure Ollama is running")
	}

	results, err := r.semantic.Search(query, limit)
	if err != nil {
		return "", fmt.Errorf("error searching: %w", err)
	}

	if len(results) == 0 {
		return "No similar code found", nil
	}

	var output []string
	output = append(output, fmt.Sprintf("Found %d similar code snippets:\n", len(results)))
	for i, result := range results {
		output = append(output, fmt.Sprintf("%d. %s (%s)", i+1, result.Name, result.Kind))
		output = append(output, fmt.Sprintf("   Location: %s:%d", result.Path, result.Line))
		output = append(output, fmt.Sprintf("   Similarity: %.2f\n", result.Similarity))
	}

	return strings.Join(output, "\n"), nil
}

func (r *ToolRegistry) findSimilarFiles(args map[string]interface{}) (string, error) {
	fileType, ok := args["file_type"].(string)
	if !ok {
		return "", fmt.Errorf("file_type is required")
	}

	// Search for files with similar names
	rows, err := r.db.Query(
		`SELECT DISTINCT f.path, f.layer, f.module
		FROM files f
		JOIN symbols s ON s.file_id = f.id
		WHERE s.name LIKE ? OR f.path LIKE ?
		LIMIT 10`,
		"%"+fileType+"%",
		"%"+fileType+"%",
	)
	if err != nil {
		return "", fmt.Errorf("error searching files: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var path, layer, module string
		if err := rows.Scan(&path, &layer, &module); err != nil {
			continue
		}
		results = append(results, fmt.Sprintf("- %s (layer: %s, module: %s)", path, layer, module))
	}

	if len(results) == 0 {
		return fmt.Sprintf("No files found matching '%s'", fileType), nil
	}

	return fmt.Sprintf("Found %d similar files:\n%s", len(results), strings.Join(results, "\n")), nil
}

// confirmPlan solicita al usuario que confirme un plan antes de ejecutarlo.
// Lee una respuesta interactiva y devuelve un resultado estructurado:
//   - "approved" si el usuario acepta (puede incluir feedback)
//   - "denied" si el usuario rechaza
//   - "edit:<nuevo plan>" si el usuario quiere ajustar
func (r *ToolRegistry) confirmPlan(args map[string]interface{}) (string, error) {
	plan, _ := args["plan"].(string)
	if plan == "" {
		return "", fmt.Errorf("plan is required")
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("  PLAN PROPUESTO — requiere confirmación")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println(plan)
	fmt.Println(strings.Repeat("─", 60))
	fmt.Print("¿Apruebas este plan? [y=aprobar / n=rechazar / e=editar / feedback libre]: ")

	response := readUserResponse()
	response = strings.TrimSpace(response)
	lower := strings.ToLower(response)

	switch {
	case lower == "" || lower == "n" || lower == "no":
		return "denied", nil
	case lower == "y" || lower == "yes" || lower == "s" || lower == "si" || lower == "sí":
		return "approved", nil
	case lower == "e" || lower == "edit" || lower == "editar":
		fmt.Print("Nuevo plan: ")
		newPlan := readUserResponse()
		if strings.TrimSpace(newPlan) == "" {
			return "denied", nil
		}
		return "edit:" + newPlan, nil
	default:
		// Cualquier otro input se trata como feedback libre y se aprueba con la nota.
		if response == "" {
			return "denied", nil
		}
		return "approved (feedback: " + response + ")", nil
	}
}

// askUser hace una pregunta al usuario y devuelve su respuesta en texto libre.
func (r *ToolRegistry) askUser(args map[string]interface{}) (string, error) {
	question, _ := args["question"].(string)
	if question == "" {
		return "", fmt.Errorf("question is required")
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("  PREGUNTA DEL AGENTE")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println(question)
	fmt.Println(strings.Repeat("─", 60))
	fmt.Print("Tu respuesta: ")

	return strings.TrimSpace(readUserResponse()), nil
}

// readUserResponse lee una línea de stdin. Si no hay TTY disponible
// (por ejemplo, en tests), devuelve una cadena vacía.
//
// Se apoya en un bufio.Reader compartido para que llamadas consecutivas
// (p. ej. confirm_plan → "e" → segundo read para "Nuevo plan:") sigan
// sobre la misma cola de bytes. Si el reader es reemplazado por tests
// (swapStdinReader), se recrea el bufio.
var stdinReader = func() interface{ Read(p []byte) (n int, err error) } {
	return os.Stdin
}

var (
	stdinMu     sync.Mutex
	stdinBuf    *bufio.Reader
	stdinSrcKey uintptr // identidad del reader actual
)

func readUserResponse() string {
	stdinMu.Lock()
	defer stdinMu.Unlock()

	r := stdinReader()
	// Si el reader cambió, descartamos el buffer.
	if stdinBuf == nil || !sameReaderKey(stdinSrcKey, r) {
		stdinBuf = bufio.NewReader(r)
		stdinSrcKey = readerKey(r)
	}
	line, err := stdinBuf.ReadString('\n')
	if err != nil {
		// Si el reader está vacío (EOF), descartamos el buffer para
		// que la siguiente llamada pueda reintentar con un reader
		// nuevo en lugar de quedarse pegada en EOF.
		if err.Error() == "EOF" {
			stdinBuf = nil
		}
		return ""
	}
	return strings.TrimRight(line, "\r\n")
}

// readerKey devuelve una clave que identifica al reader.
// Como Go no permite comparar interfaces de forma directa con ==
// para tipos diferentes, usamos reflect para obtener el puntero.
func readerKey(r interface{}) uintptr {
	v := reflect.ValueOf(r)
	if v.Kind() == reflect.Ptr || v.Kind() == reflect.UnsafePointer {
		return v.Pointer()
	}
	return 0
}

func sameReaderKey(prev uintptr, r interface{}) bool {
	return prev == readerKey(r)
}

func parseToolCall(toolCall llm.ToolCall) (string, map[string]interface{}, error) {
	var args map[string]interface{}
	
	// Handle both string and object arguments
	switch v := toolCall.Function.Arguments.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &args); err != nil {
			return "", nil, fmt.Errorf("error parsing tool arguments: %w", err)
		}
	case map[string]interface{}:
		args = v
	default:
		return "", nil, fmt.Errorf("unexpected arguments type: %T", toolCall.Function.Arguments)
	}
	
	return toolCall.Function.Name, args, nil
}
