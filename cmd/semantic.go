package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/pablofelipe1207/androideia/internal/llm"
	"github.com/pablofelipe1207/androideia/internal/semantic"
	"github.com/pablofelipe1207/androideia/internal/store"
	"github.com/spf13/cobra"
)

var semanticCmd = &cobra.Command{
	Use:   "semantic",
	Short: "Búsqueda semántica de código",
	Long: `Realiza búsquedas semánticas usando embeddings y mantiene un índice
LLM-asistido de archivos (tipo, tags, convenciones, arquitectura) que el
agente consulta para ubicarse en el proyecto.`,
}

var semanticIndexCmd = &cobra.Command{
	Use:   "index",
	Short: "Indexa archivos (clasificación LLM + embeddings)",
	Long: `Recorre todos los archivos del proyecto y, para cada uno, le pide al
LLM que lo clasifique (ViewModel, Activity, UseCase, Repository, ...) y
almacene sus tags, convenciones y arquitectura detectada. Después
genera los embeddings por símbolo para búsqueda semántica.

Si el LLM falla, igualmente intenta generar los embeddings de los
símbolos ya indexados.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Indexing project for semantic search...")
		fmt.Println()

		// Load models config (models.yml)
		mc, _, err := config.LoadModelsConfig()
		if err != nil {
			return fmt.Errorf("error loading models config: %w", err)
		}

		// Open store
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' and 'androideai index build' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Auto-resolve model from Ollama (single-model shortcut)
		if mc.Semantic.Provider == "ollama" {
			baseURL := mc.Semantic.BaseURL
			if baseURL == "" {
				baseURL = "http://localhost:11434"
			}
			resolved, autoSelected, err := llm.ResolveOllamaModel(baseURL, mc.Semantic.ChatModel)
			if err != nil {
				return fmt.Errorf("error resolving model: %w", err)
			}
			if autoSelected {
				fmt.Printf("Ollama has a single model installed; using %s (config had %s)\n", resolved, mc.Semantic.ChatModel)
			}
			mc.Semantic.ChatModel = resolved
		}

		// Create semantic instance with provider from models.yml
		provider := semantic.SemanticProvider(mc.Semantic.Provider)
		sem := semantic.NewSemanticWithProvider(s.DB(), mc.Semantic.BaseURL, mc.Semantic.ChatModel, provider)

		// 1) Clasificación por archivo (LLM). Es el paso nuevo: para
		//    cada .kt/.kts del índice, el LLM devuelve {type, tags,
		//    architecture, conventions, summary}.
		if !sem.IsAvailable() {
			fmt.Printf("⚠️  Semantic provider is not available. Skipping LLM classification, only embeddings will be refreshed.\n\n")
		} else {
			fmt.Printf("→ Clasificando archivos con %s (%s) ...\n", mc.Semantic.ChatModel, mc.Semantic.Provider)
			classified, failed, err := sem.ClassifyAllFiles()
			if err != nil {
				return fmt.Errorf("error classifying files: %w", err)
			}
			fmt.Printf("\n  Clasificados: %d   Fallidos: %d\n", classified, failed)

			arch, _, _ := sem.ArchitectureSummary()
			fmt.Printf("  Arquitectura detectada: %s\n\n", arch)
		}

		// 2) Embeddings por símbolo (comportamiento histórico). Es
		//    independiente de la clasificación: si los embeddings ya
		//    existen, no se recalculan.
		fmt.Println("→ Generando embeddings de símbolos...")
		count, err := sem.IndexAll()
		if err != nil {
			return fmt.Errorf("error indexing symbols: %w", err)
		}
		fmt.Printf("  Embeddings nuevos: %d\n\n", count)

		fmt.Println("Semantic index ready. Try:")
		fmt.Println("  androideai semantic locate viewmodel")
		fmt.Println("  androideai semantic locate LoginViewModel")
		fmt.Println("  androideai semantic locate tag:auth")
		return nil
	},
}

var semanticSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Busca código por significado (embeddings)",
	Long:  `Realiza una búsqueda semántica para encontrar código relacionado por significado.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		limit, _ := cmd.Flags().GetInt("limit")

		fmt.Printf("Semantic search for: %s\n\n", query)

		// Load models config (models.yml)
		mc, _, err := config.LoadModelsConfig()
		if err != nil {
			return fmt.Errorf("error loading models config: %w", err)
		}

		// Open store
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai index build' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Auto-resolve embedding model from Ollama (single-model shortcut)
		if mc.Semantic.Provider == "ollama" {
			baseURL := mc.Semantic.BaseURL
			if baseURL == "" {
				baseURL = "http://localhost:11434"
			}
			resolved, autoSelected, err := llm.ResolveOllamaModel(baseURL, mc.Semantic.ChatModel)
			if err != nil {
				return fmt.Errorf("error resolving model: %w", err)
			}
			if autoSelected {
				fmt.Printf("Ollama has a single model installed; using %s (config had %s)\n", resolved, mc.Semantic.ChatModel)
			}
			mc.Semantic.ChatModel = resolved
		}

		// Create semantic instance with provider from models.yml
		provider := semantic.SemanticProvider(mc.Semantic.Provider)
		sem := semantic.NewSemanticWithProvider(s.DB(), mc.Semantic.BaseURL, mc.Semantic.ChatModel, provider)

		// Search
		results, err := sem.Search(query, limit)
		if err != nil {
			return fmt.Errorf("error searching: %w", err)
		}

		if len(results) == 0 {
			fmt.Println("No results found")
			return nil
		}

		fmt.Printf("Found %d results:\n\n", len(results))
		for i, result := range results {
			fmt.Printf("%d. %s (%s)\n", i+1, result.Name, result.Kind)
			fmt.Printf("   📍 %s:%d\n", result.Path, result.Line)
			fmt.Printf("   📊 Similarity: %.2f\n\n", result.Similarity)
		}

		return nil
	},
}

var semanticLocateCmd = &cobra.Command{
	Use:   "locate [query]",
	Short: "Localiza archivos por tipo, tag o nombre (usa el índice LLM)",
	Long: `Pregunta al índice semántico por archivos. Ejemplos:

  androideai semantic locate viewmodel        # todos los ViewModel
  androideai semantic locate usecase           # todos los UseCase
  androideai semantic locate repository        # todos los Repository
  androideai semantic locate LoginViewModel    # un archivo concreto
  androideai semantic locate tag:auth          # cualquier archivo taggeado "auth"
  androideai semantic locate "state flow"      # búsqueda libre sobre conventions/summary

El comando muestra para cada coincidencia: ruta, tipo, tags, capa,
arquitectura, un resumen y un snippet de las convenciones detectadas.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")
		limit, _ := cmd.Flags().GetInt("limit")
		showAll, _ := cmd.Flags().GetBool("all")

		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' and 'androideai index build' first")
		}
		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		mc, _, err := config.LoadModelsConfig()
		if err != nil {
			return fmt.Errorf("error loading models config: %w", err)
		}
		if mc.Semantic.Provider == "ollama" {
			baseURL := mc.Semantic.BaseURL
			if baseURL == "" {
				baseURL = "http://localhost:11434"
			}
			resolved, autoSelected, err := llm.ResolveOllamaModel(baseURL, mc.Semantic.ChatModel)
			if err != nil {
				return fmt.Errorf("error resolving model: %w", err)
			}
			if autoSelected {
				fmt.Printf("Ollama has a single model installed; using %s (config had %s)\n", resolved, mc.Semantic.ChatModel)
			}
			mc.Semantic.ChatModel = resolved
		}
		provider := semantic.SemanticProvider(mc.Semantic.Provider)
		sem := semantic.NewSemanticWithProvider(s.DB(), mc.Semantic.BaseURL, mc.Semantic.ChatModel, provider)

		if showAll {
			limit = 200
		}

		results, err := sem.Locate(query, limit)
		if err != nil {
			return fmt.Errorf("error locating files: %w", err)
		}

		if len(results) == 0 {
			fmt.Printf("Sin coincidencias para %q. ¿Ya corriste 'androideai semantic index'?\n", query)
			return nil
		}

		fmt.Printf("🔎 %d coincidencia(s) para %q:\n\n", len(results), query)
		for i, r := range results {
			fmt.Printf("%d. %s\n", i+1, r.Path)
			fmt.Printf("   📦 package : %s\n", emptyAs(r.Package, "—"))
			fmt.Printf("   🏷  type    : %s\n", r.Type)
			fmt.Printf("   🏷  tags    : %s\n", strings.Join(r.Tags, ", "))
			fmt.Printf("   🏗  layer   : %s   module: %s   arch: %s\n", emptyAs(r.Layer, "—"), emptyAs(r.Module, "—"), r.Architecture)
			if r.Summary != "" {
				fmt.Printf("   📝 summary : %s\n", r.Summary)
			}
			if r.Conventions != "" {
				fmt.Printf("   📐 conv.   : %s\n", r.Conventions)
			}
			fmt.Printf("   🔍 match   : %s\n\n", r.MatchReason)
		}

		arch, _, _ := sem.ArchitectureSummary()
		fmt.Printf("Arquitectura del proyecto: %s\n", arch)
		return nil
	},
}

var semanticStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Muestra el estado de la búsqueda semántica",
	Long:  `Muestra información sobre los embeddings, las clasificaciones LLM y la conexión con Ollama.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		mc, _, err := config.LoadModelsConfig()
		if err != nil {
			return fmt.Errorf("error loading models config: %w", err)
		}

		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		if mc.Semantic.Provider == "ollama" {
			baseURL := mc.Semantic.BaseURL
			if baseURL == "" {
				baseURL = "http://localhost:11434"
			}
			resolved, autoSelected, err := llm.ResolveOllamaModel(baseURL, mc.Semantic.ChatModel)
			if err != nil {
				return fmt.Errorf("error resolving model: %w", err)
			}
			if autoSelected {
				fmt.Printf("Ollama has a single model installed; using %s (config had %s)\n", resolved, mc.Semantic.ChatModel)
			}
			mc.Semantic.ChatModel = resolved
		}

		provider := semantic.SemanticProvider(mc.Semantic.Provider)
		sem := semantic.NewSemanticWithProvider(s.DB(), mc.Semantic.BaseURL, mc.Semantic.ChatModel, provider)

		fmt.Println("Semantic Search Status")
		fmt.Println("=====================")
		fmt.Printf("Provider:   %s\n", mc.Semantic.Provider)
		fmt.Printf("Base URL:   %s\n", mc.Semantic.BaseURL)
		fmt.Printf("Chat Model: %s\n", mc.Semantic.ChatModel)
		fmt.Printf("Emb Model:  %s\n", mc.Semantic.EmbeddingModel)
		if sem.IsAvailable() {
			fmt.Println("Status:     ✅ Available")
		} else {
			fmt.Println("Status:     ❌ Not available")
		}

		var embCount int
		_ = s.DB().QueryRow("SELECT COUNT(*) FROM embeddings").Scan(&embCount)
		fmt.Printf("Embeddings: %d\n", embCount)

		var symCount int
		_ = s.DB().QueryRow("SELECT COUNT(*) FROM symbols").Scan(&symCount)
		fmt.Printf("Symbols:    %d\n", symCount)

		var fileCount int
		_ = s.DB().QueryRow("SELECT COUNT(*) FROM files").Scan(&fileCount)
		fmt.Printf("Files:      %d\n", fileCount)

		var classCount int
		_ = s.DB().QueryRow("SELECT COUNT(*) FROM file_semantics").Scan(&classCount)
		fmt.Printf("Classified: %d / %d files\n", classCount, fileCount)

		arch, arches, _ := sem.ArchitectureSummary()
		fmt.Printf("Architecture: %s\n", arch)
		if len(arches) > 0 {
			fmt.Printf("  (per-layer hits: %s)\n", strings.Join(arches, ", "))
		}

		// Top types
		rows, err := s.DB().Query(
			`SELECT type, COUNT(*) AS n FROM file_semantics WHERE type IS NOT NULL AND type != ''
			 GROUP BY type ORDER BY n DESC LIMIT 8`,
		)
		if err == nil {
			defer rows.Close()
			fmt.Println("\nTop types:")
			for rows.Next() {
				var t string
				var n int
				if err := rows.Scan(&t, &n); err == nil {
					fmt.Printf("  - %-12s %d\n", t, n)
				}
			}
		}
		return nil
	},
}

func emptyAs(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

var semanticGraphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Show the feature graph (files grouped by feature with relationships)",
	Long: `Builds and displays the feature graph from the semantic index.
Shows all features and their files, or a single feature's subgraph
with types, dependencies, and missing layers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		feature, _ := cmd.Flags().GetString("feature")

		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}
		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		mc, _, err := config.LoadModelsConfig()
		if err != nil {
			return fmt.Errorf("error loading models config: %w", err)
		}
		provider := semantic.SemanticProvider(mc.Semantic.Provider)
		sem := semantic.NewSemanticWithProvider(s.DB(), mc.Semantic.BaseURL, mc.Semantic.ChatModel, provider)

		graph, err := sem.BuildFeatureGraph()
		if err != nil {
			return fmt.Errorf("error building feature graph: %w", err)
		}

		if feature != "" {
			fmt.Print(graph.FormatSubgraph(feature))
		} else {
			summary := graph.Summary()
			fmt.Printf("Feature Graph: %d files, %d relationships\n\n", summary.TotalFiles, summary.TotalEdges)
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
				fmt.Printf("  %s (%d files: %s)\n", name, len(nodes), strings.Join(typeParts, ", "))
			}
			fmt.Printf("\nArchitecture layers: %s\n", strings.Join(summary.ArchLayers, ", "))
		}
		return nil
	},
}

var semanticDepsCmd = &cobra.Command{
	Use:   "deps [path]",
	Short: "Show dependencies for a file (what it depends on, what depends on it)",
	Long: `Analyzes the feature graph to show architectural dependencies for a
specific file. Shows outgoing dependencies (what this file needs),
incoming dependencies (what needs this file), and the impact chain
if this file changes.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}
		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		mc, _, err := config.LoadModelsConfig()
		if err != nil {
			return fmt.Errorf("error loading models config: %w", err)
		}
		provider := semantic.SemanticProvider(mc.Semantic.Provider)
		sem := semantic.NewSemanticWithProvider(s.DB(), mc.Semantic.BaseURL, mc.Semantic.ChatModel, provider)

		graph, err := sem.BuildFeatureGraph()
		if err != nil {
			return fmt.Errorf("error building feature graph: %w", err)
		}

		var nodeID int64
		for id, n := range graph.Nodes {
			if n.Path == path || strings.HasSuffix(n.Path, "/"+path) || strings.HasSuffix(path, "/"+n.Path) {
				nodeID = id
				break
			}
		}
		if nodeID == 0 {
			return fmt.Errorf("file %q not found in the feature graph", path)
		}

		node := graph.GetNode(nodeID)
		fmt.Printf("Dependencies for: %s (type: %s)\n\n", node.Path, node.Type)

		deps := graph.GetDependencies(nodeID)
		if len(deps) > 0 {
			fmt.Println("This file depends on:")
			for _, e := range deps {
				if target, ok := graph.Nodes[e.Target]; ok {
					fmt.Printf("  -> %s (%s) [%s]\n", target.Path, target.Type, e.Reason)
				}
			}
		} else {
			fmt.Println("This file has no architectural dependencies.")
		}

		dependents := graph.GetDependents(nodeID)
		if len(dependents) > 0 {
			fmt.Println("\nFiles that depend on this file:")
			for _, e := range dependents {
				if source, ok := graph.Nodes[e.Source]; ok {
					fmt.Printf("  <- %s (%s) [%s]\n", source.Path, source.Type, e.Reason)
				}
			}
		} else {
			fmt.Println("\nNo files depend on this file.")
		}

		impact := graph.GetImpact(nodeID)
		if len(impact) > 0 {
			fmt.Printf("\nImpact if this file changes (%d files affected):\n", len(impact))
			for _, n := range impact {
				fmt.Printf("  ! %s (%s)\n", n.Path, n.Type)
			}
		}

		return nil
	},
}

var semanticSuggestCmd = &cobra.Command{
	Use:   "suggest [feature]",
	Short: "Suggest what files to create or modify for a feature",
	Long: `Analyzes the feature graph to identify missing architectural layers
and files that need review. Use this when planning a new feature or
refactoring an existing one.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		feature := args[0]

		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}
		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		mc, _, err := config.LoadModelsConfig()
		if err != nil {
			return fmt.Errorf("error loading models config: %w", err)
		}
		provider := semantic.SemanticProvider(mc.Semantic.Provider)
		sem := semantic.NewSemanticWithProvider(s.DB(), mc.Semantic.BaseURL, mc.Semantic.ChatModel, provider)

		graph, err := sem.BuildFeatureGraph()
		if err != nil {
			return fmt.Errorf("error building feature graph: %w", err)
		}

		fmt.Print(graph.FormatSubgraph(feature))

		suggestions := graph.SuggestForFeature(feature)
		if len(suggestions) == 0 {
			fmt.Println("\nNo suggestions — the feature appears complete.")
			return nil
		}

		fmt.Printf("\nSuggestions (%d):\n", len(suggestions))
		for i, s := range suggestions {
			fmt.Printf("%d. [%s] %s: %s\n", i+1, strings.ToUpper(s.Action), s.Type, s.Reason)
			if s.Path != "" {
				fmt.Printf("   file: %s\n", s.Path)
			}
			if s.Context != "" {
				fmt.Printf("   %s\n", s.Context)
			}
		}

		return nil
	},
}

func init() {
	semanticSearchCmd.Flags().IntP("limit", "l", 10, "Maximum number of results")
	semanticLocateCmd.Flags().IntP("limit", "l", 10, "Maximum number of results")
	semanticLocateCmd.Flags().BoolP("all", "a", false, "Show up to 200 results")
	semanticGraphCmd.Flags().StringP("feature", "f", "", "Feature name to inspect (omit for all)")

	semanticCmd.AddCommand(semanticIndexCmd)
	semanticCmd.AddCommand(semanticSearchCmd)
	semanticCmd.AddCommand(semanticLocateCmd)
	semanticCmd.AddCommand(semanticStatusCmd)
	semanticCmd.AddCommand(semanticGraphCmd)
	semanticCmd.AddCommand(semanticDepsCmd)
	semanticCmd.AddCommand(semanticSuggestCmd)
}
