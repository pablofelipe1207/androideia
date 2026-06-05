package index

import (
	"database/sql"
	"fmt"
	"strings"
)

type FeatureLayer struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	Path     string   `json:"path"`
	Line     int      `json:"line"`
	Module   string   `json:"module"`
	Package  string   `json:"package"`
	Signature string  `json:"signature"`
}

type Feature struct {
	Name      string          `json:"name"`
	Layers    map[string][]*FeatureLayer `json:"layers"`
	Order     []string        `json:"order"`
}

func NewFeature(name string) *Feature {
	return &Feature{
		Name:   name,
		Layers: make(map[string][]*FeatureLayer),
		Order:  []string{},
	}
}

func (f *Feature) AddLayer(layer *FeatureLayer) {
	kind := layer.Kind
	if _, exists := f.Layers[kind]; !exists {
		f.Layers[kind] = []*FeatureLayer{}
		f.Order = append(f.Order, kind)
	}
	f.Layers[kind] = append(f.Layers[kind], layer)
}

func (f *Feature) GetLayerOrder() []string {
	// Define a logical order for Android layers
	orderMap := map[string]int{
		"screen":     1,
		"composable": 2,
		"viewmodel":  3,
		"usecase":    4,
		"repository": 5,
		"dao":        6,
		"module":     7,
		"route":      8,
		"test":       9,
	}

	var ordered []string
	for kind := range f.Layers {
		ordered = append(ordered, kind)
	}

	// Sort by predefined order
	for i := 0; i < len(ordered); i++ {
		for j := i + 1; j < len(ordered); j++ {
			orderI := orderMap[ordered[i]]
			orderJ := orderMap[ordered[j]]
			if orderI == 0 {
				orderI = 100
			}
			if orderJ == 0 {
				orderJ = 100
			}
			if orderI > orderJ {
				ordered[i], ordered[j] = ordered[j], ordered[i]
			}
		}
	}

	return ordered
}

func GetFeatureByName(db *sql.DB, name string) (*Feature, error) {
	// Search for symbols related to the feature name
	// This is a heuristic search - looks for symbols that contain the feature name
	query := `
		SELECT s.name, s.kind, s.signature, s.line, f.path, f.module, f.package
		FROM symbols s
		JOIN files f ON s.file_id = f.id
		WHERE LOWER(s.name) LIKE LOWER(?)
		   OR LOWER(f.path) LIKE LOWER(?)
		   OR LOWER(s.feature) LIKE LOWER(?)
		ORDER BY s.kind, s.name
	`

	featureName := "%" + name + "%"
	rows, err := db.Query(query, featureName, featureName, featureName)
	if err != nil {
		return nil, fmt.Errorf("error querying feature: %w", err)
	}
	defer rows.Close()

	feature := NewFeature(name)

	for rows.Next() {
		var symName, kind, signature, path, module, pkg string
		var line int

		if err := rows.Scan(&symName, &kind, &signature, &line, &path, &module, &pkg); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		layer := &FeatureLayer{
			Name:      symName,
			Kind:      kind,
			Path:      path,
			Line:      line,
			Module:    module,
			Package:   pkg,
			Signature: signature,
		}

		feature.AddLayer(layer)
	}

	if len(feature.Layers) == 0 {
		return nil, fmt.Errorf("feature '%s' not found", name)
	}

	return feature, nil
}

func (f *Feature) Format() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Feature: %s\n", f.Name))
	sb.WriteString(strings.Repeat("=", 40) + "\n\n")

	for _, kind := range f.GetLayerOrder() {
		layers := f.Layers[kind]
		sb.WriteString(fmt.Sprintf("📁 %s (%d)\n", strings.ToUpper(kind), len(layers)))

		for _, layer := range layers {
			sb.WriteString(fmt.Sprintf("   └─ %s\n", layer.Name))
			sb.WriteString(fmt.Sprintf("      📍 %s:%d\n", layer.Path, layer.Line))
			if layer.Module != "" {
				sb.WriteString(fmt.Sprintf("      📦 %s\n", layer.Module))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
