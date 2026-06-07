package semantic

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ConventionAggregate resume las convenciones detectadas por el LLM
// para un mismo `type` (viewmodel, usecase, repository, ...). Es la
// unidad que se guarda en el brain para que el agente pueda hacer
// `brain_search "ViewModel convention"` y obtener una respuesta útil
// sin tener que releer cada archivo del proyecto.
type ConventionAggregate struct {
	Role         string   `json:"role"`
	Architecture string   `json:"architecture"`
	Conventions  string   `json:"conventions"`  // la más votada
	Summary      string   `json:"summary"`      // la más votada
	SampleFiles  []string `json:"sample_files"` // hasta 5 paths representativos
	FileCount    int      `json:"file_count"`
	Tags         []string `json:"tags"` // unión de los top-N tags
}

// AggregateConventions recorre file_semantics, agrupa por `type` y
// devuelve un ConventionAggregate por rol con:
//
//   - la architecture más votada (excluyendo "unknown" / vacío),
//   - la convention y el summary más frecuentes (los que el LLM repitió
//     más veces para ese rol),
//   - hasta 5 paths de archivos de muestra,
//   - el top-10 de tags ordenados por frecuencia.
//
// Devuelve un slice vacío (no error) si file_semantics está vacío: el
// llamador decide si eso es OK (proyecto sin clasificar) o un aviso.
func (s *Semantic) AggregateConventions() ([]ConventionAggregate, error) {
	rows, err := s.db.Query(
		`SELECT fs.type, fs.architecture, fs.conventions, fs.summary, fs.tags, f.path
		 FROM file_semantics fs
		 JOIN files f ON f.id = fs.file_id
		 WHERE fs.type IS NOT NULL AND fs.type != ''`,
	)
	if err != nil {
		return nil, fmt.Errorf("error querying file_semantics: %w", err)
	}
	defer rows.Close()

	type bucket struct {
		archVotes   map[string]int
		convVotes   map[string]int
		summaryVot  map[string]int
		files       []string
		fileSet     map[string]bool
		tagVotes    map[string]int
		fileCount   int
	}
	m := map[string]*bucket{}

	for rows.Next() {
		var typ, arch, conv, summary, tagsJSON, path string
		if err := rows.Scan(&typ, &arch, &conv, &summary, &tagsJSON, &path); err != nil {
			return nil, fmt.Errorf("error scanning file_semantic row: %w", err)
		}
		b, ok := m[typ]
		if !ok {
			b = &bucket{
				archVotes:  map[string]int{},
				convVotes:  map[string]int{},
				summaryVot: map[string]int{},
				fileSet:    map[string]bool{},
				tagVotes:   map[string]int{},
			}
			m[typ] = b
		}
		b.fileCount++

		if arch != "" && arch != "unknown" {
			b.archVotes[arch]++
		}
		if conv != "" {
			b.convVotes[conv]++
		}
		if summary != "" {
			b.summaryVot[summary]++
		}
		if path != "" && !b.fileSet[path] && len(b.files) < 5 {
			b.fileSet[path] = true
			b.files = append(b.files, path)
		}
		var tags []string
		if err := json.Unmarshal([]byte(tagsJSON), &tags); err == nil {
			for _, t := range tags {
				b.tagVotes[t]++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]ConventionAggregate, 0, len(m))
	for typ, b := range m {
		agg := ConventionAggregate{
			Role:        typ,
			Architecture: mostVoted(b.archVotes, ""),
			Conventions:  mostVoted(b.convVotes, ""),
			Summary:      mostVoted(b.summaryVot, ""),
			SampleFiles:  b.files,
			FileCount:    b.fileCount,
			Tags:         topNVoted(b.tagVotes, 10),
		}
		out = append(out, agg)
	}

	// Orden estable: por nombre de rol para que la salida (y los IDs
	// derivados) no bailen entre ejecuciones.
	sort.Slice(out, func(i, j int) bool { return out[i].Role < out[j].Role })
	return out, nil
}

// BrainEntry convierte el aggregate en una entrada lista para
// brain.Save. Se guarda como `type=convention`, `status=promoted` (es
// un hecho derivado del código existente, no una opinión del usuario),
// y como `file_refs` se incluyen los paths de muestra para que el
// agente pueda ir a leerlos.
func (a ConventionAggregate) BrainEntry() (entryType, title, content, tags, fileRefs string) {
	entryType = "convention"
	title = fmt.Sprintf("%s convention", titleCase(a.Role))

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Convención detectada para archivos de tipo **%s** (%d archivos en el proyecto).\n\n", titleCase(a.Role), a.FileCount))
	if a.Architecture != "" {
		b.WriteString(fmt.Sprintf("- Arquitectura inferida: `%s`\n", a.Architecture))
	}
	if a.Summary != "" {
		b.WriteString(fmt.Sprintf("- Propósito típico: %s\n", a.Summary))
	}
	if a.Conventions != "" {
		b.WriteString(fmt.Sprintf("- Cómo se escriben: %s\n", a.Conventions))
	}
	if len(a.Tags) > 0 {
		b.WriteString(fmt.Sprintf("- Tags frecuentes: %s\n", strings.Join(a.Tags, ", ")))
	}
	if len(a.SampleFiles) > 0 {
		b.WriteString(fmt.Sprintf("- Archivos de muestra: %s\n", strings.Join(a.SampleFiles, ", ")))
	}
	b.WriteString("\nEsta entrada fue generada automáticamente por `androideai init` a partir de la clasificación LLM (`file_semantics`). Vuelve a correr `androideai init` o `androideai brain save` para actualizarla.\n")
	content = b.String()

	tagSet := []string{a.Role}
	if a.Architecture != "" {
		tagSet = append(tagSet, a.Architecture)
	}
	tagSet = append(tagSet, a.Tags...)
	tags = strings.Join(uniqueLower(tagSet), ",")
	fileRefs = strings.Join(a.SampleFiles, ",")
	return
}

// uniqueLower devuelve s sin duplicados, en minúsculas, preservando el
// orden. Sirve para que `tags` no tenga "viewmodel" tres veces.
func uniqueLower(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// titleCase devuelve s con la primera letra en mayúscula y el resto en
// minúscula. A diferencia de `strings.Title` (deprecado en Go 1.18+),
// esta versión es determinista y no interpreta reglas Unicode raras.
func titleCase(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// mostVoted devuelve la clave con más votos del mapa m. Si m está
// vacío devuelve fallback. Si hay empate, gana la que apareció primero
// en el map (orden no determinista, pero irrelevante: las conventions
// suelen ser únicas por rol).
func mostVoted(m map[string]int, fallback string) string {
	if len(m) == 0 {
		return fallback
	}
	best, bestN := "", -1
	for k, n := range m {
		if n > bestN {
			best, bestN = k, n
		}
	}
	return best
}

// topNVoted devuelve las N claves de m con más votos, ordenadas de
// mayor a menor. n <= 0 devuelve todas.
func topNVoted(m map[string]int, n int) []string {
	if len(m) == 0 {
		return nil
	}
	type kv struct {
		k string
		v int
	}
	pairs := make([]kv, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].v != pairs[j].v {
			return pairs[i].v > pairs[j].v
		}
		return pairs[i].k < pairs[j].k
	})
	if n > 0 && len(pairs) > n {
		pairs = pairs[:n]
	}
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = p.k
	}
	return out
}
