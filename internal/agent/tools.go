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
	"github.com/pablofelipe1207/androideia/internal/scaffold"
	"github.com/pablofelipe1207/androideia/internal/semantic"
)

type ToolRegistry struct {
	tools    []llm.Tool
	db       *sql.DB
	android  *android.Android
	semantic *semantic.Semantic
	approval string
}

func NewToolRegistry(db *sql.DB) *ToolRegistry {
	cfg, _ := config.LoadConfig()
	return NewToolRegistryWithConfig(db, cfg)
}

func NewToolRegistryWithConfig(db *sql.DB, cfg *config.Config) *ToolRegistry {
	var sem *semantic.Semantic
	if cfg != nil {
		sem = semantic.NewSemantic(db, cfg.OllamaURL, cfg.EffectiveOllamaModel())
	}
	
	approval := "ask"
	if cfg != nil {
		approval = cfg.Approval
	}
	
	registry := &ToolRegistry{
		db:       db,
		android:  android.NewAndroid(db),
		semantic: sem,
		approval: approval,
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
				Name:        "semantic_locate",
				Description: "Locate existing files in the project using the LLM-built semantic index. Use this BEFORE writing any file to know: (a) whether a file with that role (ViewModel, Activity, UseCase, Repository, ...) already exists and where it lives, (b) how files of that role are written in this project (conventions), and (c) which architecture the project uses. Examples of queries: 'viewmodel' (lists all ViewModels), 'usecase', 'LoginViewModel' (a specific file), 'tag:auth' (any file tagged 'auth').",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "What to look for. Either a type (viewmodel, usecase, repository, dao, di_module, activity, composable, nav_route, data_class, entity, service, application, test, build), a tag prefix 'tag:<name>', or a substring of a file path/class name.",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum number of results (default: 10).",
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
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "android_scaffold",
				Description: "Get a canonical Android Kotlin template for a given role (viewmodel, composable, activity, usecase, repository, dao, di_module, data_class, entity, nav_route) AND/OR check whether files of that role already exist in the project via the semantic index. ALWAYS call this BEFORE write_file when creating a new .kt/.kts file. Actions: 'check' (search semantic for existing files of role), 'template' (return the canonical code template filled with the given name), 'both' (default: do check + template).",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"role": map[string]interface{}{
							"type":        "string",
							"description": "Component role. One of: viewmodel, composable, activity, usecase, repository, dao, di_module, data_class, entity, nav_route.",
							"enum":        []string{"viewmodel", "composable", "activity", "usecase", "repository", "dao", "di_module", "data_class", "entity", "nav_route"},
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "PascalCase name of the component (e.g. 'Login' for LoginViewModel / LoginScreen / LoginUseCase).",
						},
						"action": map[string]interface{}{
							"type":        "string",
							"description": "What to return: 'check' = search semantic for existing files of the role; 'template' = return the canonical code template; 'both' (default) = do check first, then template (existing files listed at the top so you can mirror their style).",
							"enum":        []string{"check", "template", "both"},
						},
						"feature": map[string]interface{}{
							"type":        "string",
							"description": "Lowercase feature slug used inside the template (e.g. 'login', 'checkout').",
						},
						"package": map[string]interface{}{
							"type":        "string",
							"description": "Target Kotlin package, e.g. 'com.example.app.ui.login'. The CLI will derive a sensible default if omitted.",
						},
						"app_package": map[string]interface{}{
							"type":        "string",
							"description": "Application's root package (e.g. 'com.example.app'). Only used by the activity template.",
						},
						"repository_package": map[string]interface{}{
							"type":        "string",
							"description": "Package of the repository the use case depends on. Only used by the usecase template.",
						},
						"repository_name": map[string]interface{}{
							"type":        "string",
							"description": "Repository name (PascalCase, no 'Repository' suffix) used inside the usecase template.",
						},
						"entity_name": map[string]interface{}{
							"type":        "string",
							"description": "Entity name (PascalCase) used inside the dao template.",
						},
						"table": map[string]interface{}{
							"type":        "string",
							"description": "SQL table name used inside the dao / entity templates.",
						},
						"return_type": map[string]interface{}{
							"type":        "string",
							"description": "Generic return type used inside the usecase template (e.g. 'User', 'List<Product>').",
						},
					},
					"required": []string{"role", "name"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "validate_kotlin",
				Description: "Validate a Kotlin/Android file against the contract of a given role (viewmodel, composable, activity, usecase, repository, dao, di_module, data_class, entity, nav_route). Returns a list of errors and warnings. ALWAYS call this AFTER write_file and BEFORE confirm_plan so you catch missing UiState/UiEvent/UiEffect, missing hiltViewModel(), missing Hilt annotations, etc. before showing the plan to the user.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Path of the file to validate (relative to the project root or absolute).",
						},
						"role": map[string]interface{}{
							"type":        "string",
							"description": "Component role. One of: viewmodel, composable, activity, usecase, repository, dao, di_module, data_class, entity, nav_route.",
							"enum":        []string{"viewmodel", "composable", "activity", "usecase", "repository", "dao", "di_module", "data_class", "entity", "nav_route"},
						},
					},
					"required": []string{"path", "role"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "feature_graph",
				Description: "Show the feature graph: all files in the project grouped by feature, with their types and relationships. Use this BEFORE creating or modifying files to understand the full structure. Call with a feature name to see just that feature, or without arguments to see all features.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"feature": map[string]interface{}{
							"type":        "string",
							"description": "Feature name to inspect (e.g. 'login', 'checkout'). If omitted, returns all features.",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "feature_deps",
				Description: "Show dependency graph for a file: what it depends on, what depends on it, and the full impact chain if it changes. Use this BEFORE modifying a file to understand the blast radius.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Path of the file to analyze dependencies for.",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "feature_suggest",
				Description: "Suggest what files to create or modify for a feature. Shows missing architectural layers and files needing review. Use this when planning a new feature or refactoring an existing one.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"feature": map[string]interface{}{
							"type":        "string",
							"description": "Feature name to analyze (e.g. 'login', 'checkout').",
						},
					},
					"required": []string{"feature"},
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
	case "semantic_locate":
		return r.semanticLocate(args)
	case "find_similar_files":
		return r.findSimilarFiles(args)
	case "android_scaffold":
		return r.androidScaffold(args)
	case "validate_kotlin":
		return r.validateKotlin(args)
	case "confirm_plan":
		return r.confirmPlan(args)
	case "ask_user":
		return r.askUser(args)
	case "feature_graph":
		return r.featureGraph(args)
	case "feature_deps":
		return r.featureDeps(args)
	case "feature_suggest":
		return r.featureSuggest(args)
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

// semanticLocate consulta el índice LLM de archivos. El agente debe
// usarlo ANTES de crear un archivo nuevo para verificar si ya existe
// alguno con el mismo rol (ViewModel, UseCase, ...) y para conocer
// las convenciones del proyecto (arquitectura, dependencias, etc.).
func (r *ToolRegistry) semanticLocate(args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok {
		return "", fmt.Errorf("query is required")
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	if r.semantic == nil {
		return "", fmt.Errorf("semantic index not available. Make sure Ollama is running and that you have run 'androideai semantic index'")
	}

	results, err := r.semantic.Locate(query, limit)
	if err != nil {
		return "", fmt.Errorf("error locating files: %w", err)
	}

	if len(results) == 0 {
		arch, _, _ := r.semantic.ArchitectureSummary()
		return fmt.Sprintf("Sin coincidencias para %q en el índice semántico. (Arquitectura detectada: %s)\nSi esperabas resultados, asegúrate de haber corrido 'androideai semantic index' al menos una vez.", query, arch), nil
	}

	var out []string
	out = append(out, fmt.Sprintf("📚 %d coincidencia(s) en el índice semántico para %q:\n", len(results), query))
	for i, loc := range results {
		out = append(out, fmt.Sprintf("%d. %s", i+1, loc.Path))
		if loc.Package != "" {
			out = append(out, fmt.Sprintf("   package: %s   layer: %s   module: %s", loc.Package, loc.Layer, loc.Module))
		} else {
			out = append(out, fmt.Sprintf("   layer: %s   module: %s", loc.Layer, loc.Module))
		}
		out = append(out, fmt.Sprintf("   type: %s   tags: %s", loc.Type, strings.Join(loc.Tags, ", ")))
		if loc.Summary != "" {
			out = append(out, fmt.Sprintf("   summary: %s", loc.Summary))
		}
		if loc.Conventions != "" {
			out = append(out, fmt.Sprintf("   conventions: %s", loc.Conventions))
		}
		out = append(out, "")
	}

	arch, _, _ := r.semantic.ArchitectureSummary()
	out = append(out, fmt.Sprintf("Arquitectura del proyecto: %s", arch))
	return strings.Join(out, "\n"), nil
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

// androidScaffold combina la búsqueda semántica con la entrega de la
// plantilla canónica del rol. Por defecto hace las dos cosas: primero
// lista los archivos existentes del rol (para que el agente los
// inspeccione con read_file y copie convenciones) y luego devuelve la
// plantilla lista para rellenar.
func (r *ToolRegistry) androidScaffold(args map[string]interface{}) (string, error) {
	roleStr, _ := args["role"].(string)
	name, _ := args["name"].(string)
	action, _ := args["action"].(string)
	if action == "" {
		action = "both"
	}

	if roleStr == "" || name == "" {
		return "", fmt.Errorf("role and name are required")
	}
	role := scaffold.Role(roleStr)
	if !scaffold.IsValidRole(role) {
		return "", fmt.Errorf("unknown role %q (supported: %s)", roleStr, strings.Join(rolesAsStrings(), ", "))
	}

	spec, err := scaffold.SpecFor(role)
	if err != nil {
		return "", err
	}

	var out []string

	// ---- (1) Buscar referencias existentes en el índice semántico ----
	if action == "check" || action == "both" {
		existing := r.locateExisting(roleStr, 5)
		if len(existing) > 0 {
			out = append(out, fmt.Sprintf("📚 %d archivo(s) existente(s) con rol %q en el proyecto:", len(existing), role))
			for _, e := range existing {
				out = append(out, fmt.Sprintf("   • %s", e.Path))
				if e.Package != "" {
					out = append(out, fmt.Sprintf("     package: %s   layer: %s", e.Package, e.Layer))
				}
				if e.Summary != "" {
					out = append(out, fmt.Sprintf("     summary: %s", e.Summary))
				}
				if e.Conventions != "" {
					out = append(out, fmt.Sprintf("     conventions: %s", e.Conventions))
				}
			}
			out = append(out, "")
			out = append(out, "👉 Recomendación: usa read_file sobre uno de estos para copiar convenciones exactas del proyecto, ANTES de generar el tuyo desde la plantilla.")
			out = append(out, "")
		} else {
			out = append(out, fmt.Sprintf("📚 No hay archivos existentes con rol %q en el índice semántico.", role))
			if action == "both" {
				out = append(out, "   (Usa la plantilla de abajo como punto de partida.)")
				out = append(out, "")
			}
		}
	}

	// ---- (2) Devolver la plantilla canónica ----
	if action == "template" || action == "both" {
		vars := scaffold.TemplateVars{
			Package:           defaultString(args["package"], defaultPackage(name)),
			AppPackage:        defaultString(args["app_package"], defaultPackage(name)),
			RepositoryPackage: defaultString(args["repository_package"], defaultPackage(name)),
			Name:              name,
			Feature:           defaultString(args["feature"], strings.ToLower(name)),
			UseCaseName:       defaultString(getStringArg(args, "use_case_name"), name),
			UseCaseCamel:      strings.ToLower(name),
			RepositoryName:    defaultString(args["repository_name"], name+"Repository"),
			EntityName:        defaultString(args["entity_name"], name+"Entity"),
			Table:             defaultString(args["table"], strings.ToLower(name)+"s"),
			ReturnType:        defaultString(args["return_type"], "Unit"),
		}
		rendered := scaffold.RenderTemplate(spec.Template, vars)

		out = append(out, fmt.Sprintf("📐 Plantilla canónica para %s (%s):", role, spec.DisplayName))
		out = append(out, fmt.Sprintf("   FileNameHint: %s", spec.FileNameHint))
		out = append(out, "```kotlin")
		out = append(out, rendered)
		out = append(out, "```")
		out = append(out, "")
		out = append(out, "📏 Reglas de validación que se aplicarán con validate_kotlin:")
		for _, rule := range spec.Rules {
			out = append(out, fmt.Sprintf("   - %s", rule.Description))
		}
		out = append(out, "")
		out = append(out, "👉 Siguiente paso: rellena los TODO, llama write_file y luego validate_kotlin.")
	}

	return strings.Join(out, "\n"), nil
}

func (r *ToolRegistry) locateExisting(roleStr string, limit int) []semantic.FileLocation {
	if r.semantic == nil {
		// Sin índice semántico (Ollama caído o nunca corriste
		// `androideai semantic index`): intentamos al menos un LIKE
		// directo a la BD sobre el path.
		if r.db == nil {
			return nil
		}
		rows, err := r.db.Query(
			`SELECT path, package, layer, '' FROM files
			 WHERE LOWER(path) LIKE LOWER(?)
			 ORDER BY path LIMIT ?`,
			"%"+roleStr+"%", limit,
		)
		if err != nil {
			return nil
		}
		defer rows.Close()
		var out []semantic.FileLocation
		for rows.Next() {
			var l semantic.FileLocation
			if err := rows.Scan(&l.Path, &l.Package, &l.Layer); err != nil {
				continue
			}
			out = append(out, l)
		}
		return out
	}

	res, err := r.semantic.Locate(roleStr, limit)
	if err != nil {
		return nil
	}
	return res
}

func (r *ToolRegistry) validateKotlin(args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	roleStr, _ := args["role"].(string)
	if path == "" || roleStr == "" {
		return "", fmt.Errorf("path and role are required")
	}
	role := scaffold.Role(roleStr)
	if !scaffold.IsValidRole(role) {
		return "", fmt.Errorf("unknown role %q (supported: %s)", roleStr, strings.Join(rolesAsStrings(), ", "))
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf(
				"el archivo %s no existe todavía. validate_kotlin solo puede "+
					"validar contenido ya escrito en disco; primero tenés que "+
					"llamar a write_file con el contenido completo y sin TODOs, "+
					"y después volver a llamar a validate_kotlin",
				path,
			)
		}
		return "", fmt.Errorf("error reading %s: %w", path, err)
	}

	issues := scaffold.Validate(string(content), role)
	var out []string
	out = append(out, fmt.Sprintf("🔎 %s — %s", path, scaffold.Summary(issues)))
	if len(issues) == 0 {
		return strings.Join(out, "\n"), nil
	}

	for _, i := range issues {
		marker := "✗"
		if i.Severity == "warning" {
			marker = "!"
		}
		loc := ""
		if i.Line > 0 {
			loc = fmt.Sprintf(" (line %d)", i.Line)
		}
		out = append(out, fmt.Sprintf("  %s [%s] %s%s", marker, i.Rule, i.Message, loc))
	}
	out = append(out, "")
	out = append(out, "👉 Corrige los errores y vuelve a llamar validate_kotlin. Llama confirm_plan sólo cuando el resultado sea OK.")
	return strings.Join(out, "\n"), nil
}

func defaultString(v interface{}, fallback string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return fallback
}

func getStringArg(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func defaultPackage(name string) string {
	return "com.example.app.feature." + strings.ToLower(name)
}

func rolesAsStrings() []string {
	out := make([]string, 0)
	for _, r := range scaffold.AllRoles() {
		out = append(out, string(r))
	}
	return out
}

func (r *ToolRegistry) featureGraph(args map[string]interface{}) (string, error) {
	if r.semantic == nil {
		return "", fmt.Errorf("semantic index not available. Run 'androideai semantic index' first")
	}

	graph, err := r.semantic.BuildFeatureGraph()
	if err != nil {
		return "", fmt.Errorf("error building feature graph: %w", err)
	}

	feature, _ := args["feature"].(string)

	if feature != "" {
		return graph.FormatSubgraph(feature), nil
	}

	// Summary of all features
	summary := graph.Summary()
	var out []string
	out = append(out, fmt.Sprintf("Feature Graph: %d files, %d relationships\n", summary.TotalFiles, summary.TotalEdges))

	// List features
	for name, nodes := range summary.Features {
		types := make(map[string]int)
		for _, n := range nodes {
			t := n.Type
			if t == "" {
				t = "other"
			}
			types[t]++
		}
		typeParts := make([]string, 0, len(types))
		for t, c := range types {
			typeParts = append(typeParts, fmt.Sprintf("%s:%d", t, c))
		}
		out = append(out, fmt.Sprintf("  %s (%d files: %s)", name, len(nodes), strings.Join(typeParts, ", ")))
	}

	out = append(out, fmt.Sprintf("\nArchitecture layers: %s", strings.Join(summary.ArchLayers, ", ")))
	return strings.Join(out, "\n"), nil
}

func (r *ToolRegistry) featureDeps(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("path is required")
	}

	if r.semantic == nil {
		return "", fmt.Errorf("semantic index not available. Run 'androideai semantic index' first")
	}

	graph, err := r.semantic.BuildFeatureGraph()
	if err != nil {
		return "", fmt.Errorf("error building feature graph: %w", err)
	}

	// Find node by path
	var nodeID int64
	for id, n := range graph.Nodes {
		if n.Path == path || strings.HasSuffix(n.Path, "/"+path) || strings.HasSuffix(path, "/"+n.Path) {
			nodeID = id
			break
		}
	}
	if nodeID == 0 {
		return fmt.Sprintf("File %q not found in the feature graph. Make sure the file is indexed (run 'androideai semantic index').", path), nil
	}

	node := graph.GetNode(nodeID)
	var out []string
	out = append(out, fmt.Sprintf("Dependencies for: %s (type: %s)\n", node.Path, node.Type))

	// What this file depends on (outgoing)
	deps := graph.GetDependencies(nodeID)
	if len(deps) > 0 {
		out = append(out, "This file depends on:")
		for _, e := range deps {
			if target, ok := graph.Nodes[e.Target]; ok {
				out = append(out, fmt.Sprintf("  -> %s (%s) [%s]", target.Path, target.Type, e.Reason))
			}
		}
	} else {
		out = append(out, "This file has no architectural dependencies.")
	}

	// What depends on this file (incoming)
	dependents := graph.GetDependents(nodeID)
	if len(dependents) > 0 {
		out = append(out, "\nFiles that depend on this file:")
		for _, e := range dependents {
			if source, ok := graph.Nodes[e.Source]; ok {
				out = append(out, fmt.Sprintf("  <- %s (%s) [%s]", source.Path, source.Type, e.Reason))
			}
		}
	} else {
		out = append(out, "\nNo files depend on this file.")
	}

	// Impact analysis
	impact := graph.GetImpact(nodeID)
	if len(impact) > 0 {
		out = append(out, fmt.Sprintf("\nImpact if this file changes (%d files affected):", len(impact)))
		for _, n := range impact {
			out = append(out, fmt.Sprintf("  ! %s (%s)", n.Path, n.Type))
		}
	}

	return strings.Join(out, "\n"), nil
}

func (r *ToolRegistry) featureSuggest(args map[string]interface{}) (string, error) {
	feature, ok := args["feature"].(string)
	if !ok || feature == "" {
		return "", fmt.Errorf("feature is required")
	}

	if r.semantic == nil {
		return "", fmt.Errorf("semantic index not available. Run 'androideai semantic index' first")
	}

	graph, err := r.semantic.BuildFeatureGraph()
	if err != nil {
		return "", fmt.Errorf("error building feature graph: %w", err)
	}

	suggestions := graph.SuggestForFeature(feature)
	subgraph := graph.FormatSubgraph(feature)

	var out []string
	out = append(out, subgraph)

	if len(suggestions) == 0 {
		out = append(out, "\nNo suggestions — the feature appears complete.")
		return strings.Join(out, "\n"), nil
	}

	out = append(out, fmt.Sprintf("\nSuggestions (%d):", len(suggestions)))
	for i, s := range suggestions {
		out = append(out, fmt.Sprintf("%d. [%s] %s: %s", i+1, strings.ToUpper(s.Action), s.Type, s.Reason))
		if s.Path != "" {
			out = append(out, fmt.Sprintf("   file: %s", s.Path))
		}
		if s.Context != "" {
			out = append(out, fmt.Sprintf("   %s", s.Context))
		}
	}

	return strings.Join(out, "\n"), nil
}

// confirmPlan solicita al usuario que confirme un plan antes de ejecutarlo.
// Lee una respuesta interactiva y devuelve un resultado estructurado:
//   - "approved" si el usuario acepta (puede incluir feedback)
//   - "denied" si el usuario rechaza
//   - "edit:<nuevo plan>" si el usuario quiere ajustar
func (r *ToolRegistry) confirmPlan(args map[string]interface{}) (string, error) {
	if r.approval == "auto" {
		return "approved", nil
	}

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

	if r.approval == "auto" {
		return "", nil
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
