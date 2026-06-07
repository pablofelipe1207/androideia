package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pablofelipe1207/androideia/internal/brain"
	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/pablofelipe1207/androideia/internal/llm"
	"github.com/pablofelipe1207/androideia/internal/semantic"
	"github.com/pablofelipe1207/androideia/internal/store"
	"github.com/spf13/cobra"
)

var (
	initNoIndex     bool
	initNoSemantic  bool
	initNoBrainSeed bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Inicializa la configuración, base de datos, índice y semántica del proyecto",
	Long: `Crea el directorio .androideai/, archivos de configuración y base
de datos del proyecto. Por defecto, además:

  1. Construye el índice de código (androideai index build).
  2. Si Ollama está disponible, corre la clasificación LLM de
     archivos + embeddings (androideai semantic index).
  3. Si la clasificación produjo convenciones, las guarda en el brain
     como entradas tipo "convention" para que el agente las use
     (brain_search "ViewModel convention", etc.).

Flags:
  --no-index       Salta la construcción del índice de código.
  --no-semantic    Salta la clasificación LLM y los embeddings.
  --no-brain-seed  No siembra el brain con las convenciones detectadas.

Si Ollama no está disponible, el paso de semantic se omite
silenciosamente (con un aviso) y el resto del init sigue.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Inicializando androideai-core...")

		// ------------------------------------------------------------
		// 1) FS: directorio .androideai, .gitignore, config.yml, DB
		// ------------------------------------------------------------
		if err := os.MkdirAll(".androideai", 0755); err != nil {
			return fmt.Errorf("error creando directorio .androideai: %w", err)
		}

		gitignorePath := ".gitignore"
		if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
			if err := os.WriteFile(gitignorePath, []byte(".androideai/core.db\n"), 0644); err != nil {
				return fmt.Errorf("error creando .gitignore: %w", err)
			}
			fmt.Println("  ✓ Created .gitignore")
		} else {
			fmt.Println("  • .gitignore ya existe, se conserva")
		}

		configPath := filepath.Join(".androideai", "config.yml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			cfg := config.DefaultConfig()
			if err := cfg.Save(configPath); err != nil {
				return fmt.Errorf("error creando config.yml: %w", err)
			}
			fmt.Println("  ✓ Created .androideai/config.yml")
		} else {
			fmt.Println("  • .androideai/config.yml ya existe, se conserva")
		}

		dbPath := filepath.Join(".androideai", "core.db")
		dbExisted := true
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			s, err := store.NewStore(dbPath)
			if err != nil {
				return fmt.Errorf("error creating database: %w", err)
			}
			_ = s.Close()
			dbExisted = false
			fmt.Println("  ✓ Created .androideai/core.db with schema")
		} else {
			fmt.Println("  • .androideai/core.db ya existe, se conserva")
		}

		// ------------------------------------------------------------
		// 2) Índice de código (index build)
		// ------------------------------------------------------------
		if !initNoIndex {
			fmt.Println()
			fmt.Println("→ Construyendo índice de código...")
			if err := runIndexBuildSilently(); err != nil {
				fmt.Printf("  ⚠️  index build falló: %v\n", err)
			}
		} else {
			fmt.Println("→ Saltando index build (--no-index)")
		}

		// ------------------------------------------------------------
		// 3) Semántica (LLM classify + embeddings)
		// ------------------------------------------------------------
		classifiedSomething := false
		if !initNoSemantic {
			fmt.Println()
			fmt.Println("→ Corriendo índice semántico (LLM classify + embeddings)...")
			ok, err := runSemanticIndexSilently()
			if err != nil {
				fmt.Printf("  ⚠️  semantic index falló: %v\n", err)
			} else if ok {
				classifiedSomething = true
			}
		} else {
			fmt.Println("→ Saltando semantic index (--no-semantic)")
		}

		// ------------------------------------------------------------
		// 4) Sembrar el brain con las convenciones detectadas
		// ------------------------------------------------------------
		if !initNoBrainSeed {
			fmt.Println()
			fmt.Println("→ Sembrando brain con convenciones detectadas...")
			if !dbExisted && !classifiedSomething {
				fmt.Println("  • No hay datos en file_semantics aún; salta el seed.")
				fmt.Println("    (corre 'androideai semantic index' primero si quieres poblar el brain)")
			} else {
				if err := seedBrainFromSemantic(); err != nil {
					fmt.Printf("  ⚠️  brain seed falló: %v\n", err)
				}
			}
		} else {
			fmt.Println("→ Saltando brain seed (--no-brain-seed)")
		}

		fmt.Println()
		fmt.Println("Initialization complete!")
		fmt.Println("Siguiente paso sugerido:")
		fmt.Println("  androideai agent \"Crea una feature de login con MVVM\"")
		return nil
	},
}

// runIndexBuildSilently invoca la lógica de `index build` desde el
// comando init. Reutilizamos el RunE existente: silenciamos su salida
// y dejamos que el método imprima sus propios progress (Found N files,
// Indexed X, etc.).
func runIndexBuildSilently() error {
	// El RunE de indexBuildCmd espera existir y abrir la DB; si la DB
	// no existe fallaría con un mensaje claro, así que la creamos
	// defensivamente aquí. store.NewStore es idempotente porque las
	// migraciones son CREATE TABLE IF NOT EXISTS.
	dbPath := filepath.Join(".androideai", "core.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		s, err := store.NewStore(dbPath)
		if err != nil {
			return err
		}
		_ = s.Close()
	}
	return indexBuildCmd.RunE(indexBuildCmd, nil)
}

// runSemanticIndexSilently invoca `semantic index` y devuelve (true,
// nil) si clasificó al menos un archivo, (false, nil) si Ollama no
// estaba disponible o si no había archivos que clasificar, y
// (_, err) si algo falló.
func runSemanticIndexSilently() (bool, error) {
	// Cargar config para resolver el modelo (Ollama auto-select).
	cfg, err := config.LoadConfig()
	if err != nil {
		return false, fmt.Errorf("load config: %w", err)
	}
	if cfg.Provider == "ollama" {
		if _, _, err := llm.ResolveOllamaModel(cfg.OllamaURL, cfg.Model); err != nil {
			// No Ollama / no modelos: avisamos y seguimos sin
			// fallar el init.
			fmt.Printf("  ⚠️  Ollama no disponible en %s; se omite la clasificación LLM.\n", cfg.OllamaURL)
			fmt.Println("     (el resto del init sigue; corre 'androideai semantic index' cuando Ollama esté arriba)")
			return false, nil
		}
	}

	// Llamamos directamente al RunE de semanticIndexCmd. Su salida ya
	// es informativa (Found N, Clasificados, etc.), así que dejamos
	// que imprima.

	// Para saber si clasificó algo, miramos file_semantics antes y
	// después.
	before := countFileSemantics()

	if err := semanticIndexCmd.RunE(semanticIndexCmd, nil); err != nil {
		return false, err
	}

	after := countFileSemantics()
	return after > before, nil
}

func countFileSemantics() int {
	dbPath := filepath.Join(".androideai", "core.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		return 0
	}
	defer s.Close()
	var n int
	_ = s.DB().QueryRow(`SELECT COUNT(*) FROM file_semantics`).Scan(&n)
	return n
}

// seedBrainFromSemantic lee las convenciones agregadas de file_semantics
// y guarda una entrada en el brain por cada rol. Usa SaveIfNotExists
// para que correr `init` varias veces NO duplique las entradas.
func seedBrainFromSemantic() error {
	dbPath := filepath.Join(".androideai", "core.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		return err
	}
	defer s.Close()

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	if cfg.Provider == "ollama" {
		if resolved, _, err := llm.ResolveOllamaModel(cfg.OllamaURL, cfg.Model); err == nil {
			cfg.Model = resolved
		}
	}
	sem := semantic.NewSemantic(s.DB(), cfg.OllamaURL, cfg.Model)

	aggregates, err := sem.AggregateConventions()
	if err != nil {
		return err
	}
	if len(aggregates) == 0 {
		fmt.Println("  • file_semantics está vacío: nada que sembrar.")
		fmt.Println("    (corre 'androideai semantic index' primero)")
		return nil
	}

	b := brain.NewBrain(s.DB())
	inserted, skipped := 0, 0
	for _, agg := range aggregates {
		entryType, title, content, tags, fileRefs := agg.BrainEntry()
		entry := &brain.KnowledgeEntry{
			Type:     entryType,
			Title:    title,
			Content:  content,
			Tags:     tags,
			FileRefs: fileRefs,
			Status:   "promoted",
		}
		_, created, err := b.SaveIfNotExists(entry)
		if err != nil {
			fmt.Printf("  ⚠️  %s: %v\n", title, err)
			continue
		}
		if created {
			inserted++
			fmt.Printf("  ✓ %s (%d archivo(s) de muestra)\n", title, len(agg.SampleFiles))
		} else {
			skipped++
		}
	}
	if inserted == 0 && skipped > 0 {
		fmt.Println("  • Las convenciones ya estaban en el brain; nada nuevo que añadir.")
		fmt.Println("    (bórralas con 'androideai brain delete <id>' si quieres regenerarlas)")
	} else {
		fmt.Printf("  %d nueva(s), %d ya existente(s).\n", inserted, skipped)
		fmt.Println("  El agente ahora puede hacer brain_search \"<rol> convention\" antes de crear archivos.")
	}
	return nil
}

func init() {
	initCmd.Flags().BoolVar(&initNoIndex, "no-index", false, "Saltar la construcción del índice de código")
	initCmd.Flags().BoolVar(&initNoSemantic, "no-semantic", false, "Saltar la clasificación LLM y los embeddings")
	initCmd.Flags().BoolVar(&initNoBrainSeed, "no-brain-seed", false, "No sembrar el brain con las convenciones detectadas")
}
