package semantic

import (
	"fmt"
	"path/filepath"
	"strings"
)

// EdgeType defines the type of relationship between files.
type EdgeType string

const (
	EdgeFeatureMember EdgeType = "feature_member" // files in the same feature
	EdgeDependsOn     EdgeType = "depends_on"     // architectural dependency (vm->usecase->repo->dao)
	EdgeProvidesFor   EdgeType = "provides_for"   // DI module provides dependency
	EdgeCalls         EdgeType = "calls"          // symbol calls symbol in another file
)

// GraphNode represents a file in the feature graph.
type GraphNode struct {
	ID           int64    `json:"id"`
	Path         string   `json:"path"`
	Package      string   `json:"package"`
	Module       string   `json:"module"`
	Type         string   `json:"type"`
	Tags         []string `json:"tags"`
	Architecture string   `json:"architecture"`
	Summary      string   `json:"summary"`
	Layer        int      `json:"layer"` // architectural layer order
}

// GraphEdge represents a relationship between two files.
type GraphEdge struct {
	Source     int64    `json:"source"`     // source file ID
	Target     int64    `json:"target"`     // target file ID
	EdgeType   EdgeType `json:"edge_type"`  // relationship type
	Reason     string   `json:"reason"`     // human-readable explanation
	Confidence float64  `json:"confidence"` // 0.0-1.0
}

// FeatureGraph is an in-memory graph of file relationships.
type FeatureGraph struct {
	Nodes map[int64]*GraphNode `json:"nodes"`
	Edges []*GraphEdge         `json:"edges"`
	// adjacency list: source_id -> edges from that source
	adj map[int64][]*GraphEdge
}

// FeatureGraphSummary is a condensed view of the graph for the agent.
type FeatureGraphSummary struct {
	Features   map[string][]*GraphNode `json:"features"`   // feature name -> nodes
	ArchLayers []string                `json:"arch_layers"` // detected layers
	TotalFiles int                     `json:"total_files"`
	TotalEdges int                     `json:"total_edges"`
}

var layerOrder = map[string]int{
	"activity":    1,
	"composable":  2,
	"viewmodel":   3,
	"usecase":     4,
	"repository":  5,
	"dao":         6,
	"di_module":   7,
	"nav_route":   8,
	"service":     9,
	"entity":      10,
	"data_class":  11,
	"application": 12,
	"test":        13,
	"build":       14,
	"other":       15,
}

// BuildFeatureGraph constructs the full feature graph from the semantic index.
func (s *Semantic) BuildFeatureGraph() (*FeatureGraph, error) {
	g := &FeatureGraph{
		Nodes: make(map[int64]*GraphNode),
		Edges: make([]*GraphEdge, 0),
		adj:   make(map[int64][]*GraphEdge),
	}

	// 1) Load all files with their semantic classification
	rows, err := s.db.Query(`
		SELECT f.id, f.path, COALESCE(f.package,''), COALESCE(f.module,''),
		       COALESCE(fs.type,''), COALESCE(fs.tags,''),
		       COALESCE(fs.architecture,''), COALESCE(fs.summary,'')
		FROM files f
		LEFT JOIN file_semantics fs ON fs.file_id = f.id
		WHERE f.path LIKE '%.kt' OR f.path LIKE '%.kts'
	`)
	if err != nil {
		return nil, fmt.Errorf("querying files: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var n GraphNode
		var tagsRaw string
		if err := rows.Scan(&n.ID, &n.Path, &n.Package, &n.Module,
			&n.Type, &tagsRaw, &n.Architecture, &n.Summary); err != nil {
			return nil, fmt.Errorf("scanning file: %w", err)
		}
		n.Layer = layerForType(n.Type)
		n.Tags = parseTags(tagsRaw)
		g.Nodes[n.ID] = &n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 2) Build edges
	s.buildFeatureMemberEdges(g)
	s.buildArchDependencyEdges(g)
	s.buildPackageEdges(g)

	// 3) Build adjacency list
	for _, e := range g.Edges {
		g.adj[e.Source] = append(g.adj[e.Source], e)
	}

	return g, nil
}

// buildFeatureMemberEdges groups files by feature name.
// Feature is derived from: tags, path segments, file name.
func (s *Semantic) buildFeatureMemberEdges(g *FeatureGraph) {
	featureGroups := make(map[string][]int64) // feature -> file IDs

	for id, n := range g.Nodes {
		features := extractFeatureNames(n)
		for _, f := range features {
			featureGroups[f] = append(featureGroups[f], id)
		}
	}

	// Create edges between files in the same feature
	for feature, ids := range featureGroups {
		for i := 0; i < len(ids); i++ {
			for j := i + 1; j < len(ids); j++ {
				g.Edges = append(g.Edges, &GraphEdge{
					Source:     ids[i],
					Target:     ids[j],
					EdgeType:   EdgeFeatureMember,
					Reason:     fmt.Sprintf("both belong to feature '%s'", feature),
					Confidence: 0.8,
				})
				g.Edges = append(g.Edges, &GraphEdge{
					Source:     ids[j],
					Target:     ids[i],
					EdgeType:   EdgeFeatureMember,
					Reason:     fmt.Sprintf("both belong to feature '%s'", feature),
					Confidence: 0.8,
				})
			}
		}
	}
}

// buildArchDependencyEdges infers architectural dependencies based on file types.
// e.g., ViewModel depends on UseCase, UseCase depends on Repository, etc.
func (s *Semantic) buildArchDependencyEdges(g *FeatureGraph) {
	// Group nodes by feature and package for smarter matching
	type fileGroup struct {
		feature string
		pkg     string
		files   []*GraphNode
	}
	groups := make(map[string]*fileGroup) // "feature|package" -> group

	for _, n := range g.Nodes {
		features := extractFeatureNames(n)
		for _, f := range features {
			key := f + "|" + n.Package
			if _, ok := groups[key]; !ok {
				groups[key] = &fileGroup{feature: f, pkg: n.Package}
			}
			groups[key].files = append(groups[key].files, n)
		}
	}

	// Define dependency chains: source type depends on target type
	depChains := map[string][]string{
		"activity":   {"viewmodel"},
		"composable": {"viewmodel"},
		"viewmodel":  {"usecase", "repository"},
		"usecase":    {"repository"},
		"repository": {"dao", "service"},
	}

	for _, grp := range groups {
		// Index files by type within this group
		byType := make(map[string][]*GraphNode)
		for _, f := range grp.files {
			if f.Type != "" {
				byType[f.Type] = append(byType[f.Type], f)
			}
		}

		// Create dependency edges
		for srcType, targetTypes := range depChains {
			sources := byType[srcType]
			if len(sources) == 0 {
				continue
			}
			for _, tgtType := range targetTypes {
				targets := byType[tgtType]
				for _, src := range sources {
					for _, tgt := range targets {
						g.Edges = append(g.Edges, &GraphEdge{
							Source:     src.ID,
							Target:     tgt.ID,
							EdgeType:   EdgeDependsOn,
							Reason:     fmt.Sprintf("%s typically depends on %s", srcType, tgtType),
							Confidence: 0.7,
						})
					}
				}
			}
		}
	}
}

// buildPackageEdges connects files in the same package (likely related).
func (s *Semantic) buildPackageEdges(g *FeatureGraph) {
	byPkg := make(map[string][]int64)
	for id, n := range g.Nodes {
		if n.Package != "" {
			byPkg[n.Package] = append(byPkg[n.Package], id)
		}
	}

	// Only create edges for small packages (< 15 files) to avoid noise
	for _, ids := range byPkg {
		if len(ids) > 15 || len(ids) < 2 {
			continue
		}
		for i := 0; i < len(ids); i++ {
			for j := i + 1; j < len(ids); j++ {
				// Avoid duplicate edges if feature_member already exists
				if !hasEdgeBetween(g, ids[i], ids[j]) {
					g.Edges = append(g.Edges, &GraphEdge{
						Source:     ids[i],
						Target:     ids[j],
						EdgeType:   EdgeFeatureMember,
						Reason:     "same package",
						Confidence: 0.5,
					})
				}
			}
		}
	}
}

// GetNode returns a node by ID.
func (g *FeatureGraph) GetNode(id int64) *GraphNode {
	return g.Nodes[id]
}

// GetNeighbors returns all nodes connected to the given node (outgoing edges).
func (g *FeatureGraph) GetNeighbors(id int64) []*GraphEdge {
	return g.adj[id]
}

// GetFeatureNodes returns all nodes belonging to a feature.
func (g *FeatureGraph) GetFeatureNodes(featureName string) []*GraphNode {
	var result []*GraphNode
	lower := strings.ToLower(featureName)
	for _, n := range g.Nodes {
		for _, f := range extractFeatureNames(n) {
			if strings.ToLower(f) == lower {
				result = append(result, n)
				break
			}
		}
	}
	return result
}

// GetDependencies returns files that the given file depends on (depth=1).
func (g *FeatureGraph) GetDependencies(fileID int64) []*GraphEdge {
	var result []*GraphEdge
	for _, e := range g.adj[fileID] {
		if e.EdgeType == EdgeDependsOn || e.EdgeType == EdgeProvidesFor {
			result = append(result, e)
		}
	}
	return result
}

// GetDependents returns files that depend on the given file (reverse edges).
func (g *FeatureGraph) GetDependents(fileID int64) []*GraphEdge {
	var result []*GraphEdge
	for _, edges := range g.adj {
		for _, e := range edges {
			if e.Target == fileID && (e.EdgeType == EdgeDependsOn || e.EdgeType == EdgeProvidesFor) {
				result = append(result, e)
			}
		}
	}
	return result
}

// GetImpact returns all files affected if the given file changes (transitive dependents).
func (g *FeatureGraph) GetImpact(fileID int64) []*GraphNode {
	visited := make(map[int64]bool)
	var result []*GraphNode

	var walk func(id int64)
	walk = func(id int64) {
		if visited[id] {
			return
		}
		visited[id] = true
		for _, edges := range g.adj {
			for _, e := range edges {
				if e.Target == id && (e.EdgeType == EdgeDependsOn || e.EdgeType == EdgeProvidesFor) {
					if !visited[e.Source] {
						if n, ok := g.Nodes[e.Source]; ok {
							result = append(result, n)
						}
						walk(e.Source)
					}
				}
			}
		}
	}
	walk(fileID)
	return result
}

// GetMissingLayers returns architectural layers that are expected but missing for a feature.
func (g *FeatureGraph) GetMissingLayers(featureName string) []string {
	nodes := g.GetFeatureNodes(featureName)
	if len(nodes) == 0 {
		return nil
	}

	present := make(map[string]bool)
	for _, n := range nodes {
		if n.Type != "" && n.Type != "other" {
			present[n.Type] = true
		}
	}

	// Determine which layers are expected based on what's present
	expected := make(map[string]bool)
	for t := range present {
		switch t {
		case "activity", "composable":
			expected["viewmodel"] = true
			expected["usecase"] = true
			expected["repository"] = true
		case "viewmodel":
			expected["usecase"] = true
			expected["repository"] = true
		case "usecase":
			expected["repository"] = true
		case "repository":
			expected["dao"] = true
		}
	}

	var missing []string
	for layer := range expected {
		if !present[layer] {
			missing = append(missing, layer)
		}
	}
	return missing
}

// SuggestForFeature suggests what files to create or modify for a feature.
func (g *FeatureGraph) SuggestForFeature(featureName string) []*GraphSuggestion {
	nodes := g.GetFeatureNodes(featureName)
	missing := g.GetMissingLayers(featureName)

	var suggestions []*GraphSuggestion

	// Suggest creating missing layers
	layerDescriptions := map[string]string{
		"viewmodel":  "ViewModel to hold UI state",
		"usecase":    "UseCase to encapsulate business logic",
		"repository": "Repository to abstract data sources",
		"dao":        "DAO for local database access",
		"activity":   "Activity as the entry point",
		"composable": "Composable UI screen",
		"di_module":  "Hilt/Dagger DI module",
		"nav_route":  "Navigation route definition",
		"test":       "Unit/UI tests",
	}

	for _, layer := range missing {
		suggestions = append(suggestions, &GraphSuggestion{
			Action:  "create",
			Type:    layer,
			Reason:  fmt.Sprintf("feature '%s' is missing %s layer", featureName, layer),
			Context: layerDescriptions[layer],
		})
	}

	// Suggest modifying existing files if they have weak conventions
	for _, n := range nodes {
		if n.Summary == "" || n.Architecture == "unknown" {
			suggestions = append(suggestions, &GraphSuggestion{
				Action:  "review",
				Type:    n.Type,
				Path:    n.Path,
				Reason:  "file lacks semantic classification or has unknown architecture",
				Context: fmt.Sprintf("summary: %q, arch: %q", n.Summary, n.Architecture),
			})
		}
	}

	return suggestions
}

// FeatureNames returns all detected feature names in the project.
func (g *FeatureGraph) FeatureNames() []string {
	seen := make(map[string]bool)
	var result []string
	for _, n := range g.Nodes {
		for _, f := range extractFeatureNames(n) {
			if !seen[f] {
				seen[f] = true
				result = append(result, f)
			}
		}
	}
	return result
}

// Summary returns a condensed summary of the graph.
func (g *FeatureGraph) Summary() *FeatureGraphSummary {
	summary := &FeatureGraphSummary{
		Features:   make(map[string][]*GraphNode),
		TotalFiles: len(g.Nodes),
		TotalEdges: len(g.Edges),
	}

	seenLayers := make(map[string]bool)
	for _, n := range g.Nodes {
		for _, f := range extractFeatureNames(n) {
			summary.Features[f] = append(summary.Features[f], n)
		}
		if n.Type != "" {
			seenLayers[n.Type] = true
		}
	}
	for layer := range seenLayers {
		summary.ArchLayers = append(summary.ArchLayers, layer)
	}

	return summary
}

// FormatSubgraph returns a human-readable representation of a subgraph.
func (g *FeatureGraph) FormatSubgraph(featureName string) string {
	var sb strings.Builder
	nodes := g.GetFeatureNodes(featureName)
	if len(nodes) == 0 {
		sb.WriteString(fmt.Sprintf("No files found for feature '%s'\n", featureName))
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("Feature: %s (%d files)\n", featureName, len(nodes)))
	sb.WriteString(strings.Repeat("=", 40) + "\n\n")

	// Group by type
	byType := make(map[string][]*GraphNode)
	for _, n := range nodes {
		t := n.Type
		if t == "" {
			t = "other"
		}
		byType[t] = append(byType[t], n)
	}

	// Show in architectural order
	for _, t := range []string{"activity", "composable", "viewmodel", "usecase", "repository", "dao", "di_module", "nav_route", "test", "other"} {
		group := byType[t]
		if len(group) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("  %s (%d)\n", strings.ToUpper(t), len(group)))
		for _, n := range group {
			sb.WriteString(fmt.Sprintf("    %s\n", filepath.Base(n.Path)))
			if n.Summary != "" {
				sb.WriteString(fmt.Sprintf("      %s\n", n.Summary))
			}
		}
	}

	// Show dependencies
	deps := make([]string, 0)
	for _, n := range nodes {
		for _, e := range g.GetDependencies(n.ID) {
			if target, ok := g.Nodes[e.Target]; ok {
				deps = append(deps, fmt.Sprintf("  %s -> %s (%s)",
					filepath.Base(n.Path), filepath.Base(target.Path), e.Reason))
			}
		}
	}
	if len(deps) > 0 {
		sb.WriteString("\nDependencies:\n")
		for _, d := range deps {
			sb.WriteString(d + "\n")
		}
	}

	// Show missing layers
	missing := g.GetMissingLayers(featureName)
	if len(missing) > 0 {
		sb.WriteString(fmt.Sprintf("\nMissing layers: %s\n", strings.Join(missing, ", ")))
	}

	return sb.String()
}

// GraphSuggestion is a suggestion for the agent about a feature.
type GraphSuggestion struct {
	Action  string `json:"action"`  // "create", "review", "modify"
	Type    string `json:"type"`    // file type (viewmodel, usecase, etc.)
	Path    string `json:"path"`    // existing file path (for modify/review)
	Reason  string `json:"reason"`  // why this suggestion
	Context string `json:"context"` // additional context
}

// --- helpers ---

func layerForType(t string) int {
	if l, ok := layerOrder[t]; ok {
		return l
	}
	return 100
}

func parseTags(raw string) []string {
	raw = strings.Trim(raw, "[]")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var tags []string
	for _, p := range parts {
		p = strings.Trim(strings.TrimSpace(p), `"`)
		if p != "" {
			tags = append(tags, p)
		}
	}
	return tags
}

// extractFeatureNames derives feature names from a node's metadata.
func extractFeatureNames(n *GraphNode) []string {
	seen := make(map[string]bool)
	var result []string

	add := func(f string) {
		f = strings.ToLower(strings.TrimSpace(f))
		if f != "" && !seen[f] && len(f) > 2 {
			seen[f] = true
			result = append(result, f)
		}
	}

	// From tags
	for _, tag := range n.Tags {
		add(tag)
	}

	// From file path segments
	parts := strings.Split(n.Path, "/")
	for _, p := range parts {
		base := strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(p, ".kt"), ".kts"))
		if base != "" && base != "src" && base != "main" && base != "java" && base != "kotlin" && len(base) > 2 {
			add(base)
		}
	}

	// From file name prefix (e.g., LoginViewModel -> login)
	base := filepath.Base(n.Path)
	base = strings.TrimSuffix(strings.TrimSuffix(base, ".kt"), ".kts")
	if strings.HasSuffix(base, "ViewModel") || strings.HasSuffix(base, "vm") {
		add(strings.TrimSuffix(strings.TrimSuffix(base, "ViewModel"), "vm"))
	} else if strings.HasSuffix(base, "Screen") {
		add(strings.TrimSuffix(base, "Screen"))
	} else if strings.HasSuffix(base, "Activity") {
		add(strings.TrimSuffix(base, "Activity"))
	} else if strings.HasSuffix(base, "UseCase") {
		add(strings.TrimSuffix(base, "UseCase"))
	} else if strings.HasSuffix(base, "Repository") {
		add(strings.TrimSuffix(base, "Repository"))
	} else if strings.HasSuffix(base, "Dao") {
		add(strings.TrimSuffix(base, "Dao"))
	}

	return result
}

func hasEdgeBetween(g *FeatureGraph, a, b int64) bool {
	for _, e := range g.adj[a] {
		if e.Target == b {
			return true
		}
	}
	return false
}
