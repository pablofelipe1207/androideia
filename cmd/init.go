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
	initNoIndex         bool
	initNoSemantic      bool
	initNoBrainSeed     bool
	initNoLLMFeatures   bool
	initWithLLMFeatures bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Inicializa la configuración, base de datos, índice y semántica del proyecto",
	Long: `Crea el directorio .androideai/, archivos de configuración y base
de datos del proyecto. Por defecto, además:

  1. Construye el índice de código (androideai index build).
  2. Si Ollama está disponible, corre la clasificación LLM de
     archivos + embeddings (androideai semantic index).
  3. Si Ollama está disponible y se pasa --with-llm-features,
     descubre features con LLM (androideai index build --use-llm).
  4. Si la clasificación produjo convenciones, las guarda en el brain
     como entradas tipo "convention" para que el agente las use
     (brain_search "ViewModel convention", etc.).

Flags:
  --no-index         Salta la construcción del índice de código.
  --no-semantic      Salta la clasificación LLM y los embeddings.
  --no-llm-features  Salta el descubrimiento de features con LLM.
  --no-brain-seed    No siembra el brain con las convenciones detectadas.

Si Ollama no está disponible, los pasos de LLM se omiten
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

		// Crear models.yml (nuevo formato de configuración de modelos)
		// con valores por defecto sensatos: agente con OpenCode Zen
		// (free) y semantic con Ollama local.
		modelsPath := filepath.Join(".androideai", "models.yml")
		if _, err := os.Stat(modelsPath); os.IsNotExist(err) {
			mc := config.DefaultModelsConfig()
			mc.ProjectPath = modelsPath
			if err := mc.Save(modelsPath); err != nil {
				return fmt.Errorf("error creando models.yml: %w", err)
			}
			fmt.Println("  ✓ Created .androideai/models.yml")
			fmt.Println("    (configuración de modelos: agent + semantic index)")
			fmt.Println("    Editá con 'androideai models set' o a mano.")
		} else {
			fmt.Println("  • .androideai/models.yml ya existe, se conserva")
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
		// 4) Descubrimiento de features con LLM (opcional, --with-llm-features)
		//    Requiere que el índice semántico haya corrido primero para
		//    tener los ViewModels y archivos clasificados en la DB.
		// ------------------------------------------------------------
		if initWithLLMFeatures {
			fmt.Println()
			fmt.Println("→ Descubriendo features con LLM (Ollama)...")
			if err := runLLMFeatureDiscoverySilently(); err != nil {
				fmt.Printf("  ⚠️  LLM feature discovery falló: %v\n", err)
			}
		} else {
			fmt.Println("→ Saltando descubrimiento LLM de features (usa --with-llm-features para habilitar)")
		}

		// ------------------------------------------------------------
		// 5) Sembrar el brain con las convenciones detectadas
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

// runLLMFeatureDiscoverySilently descubre features usando LLM
// sobre el índice YA CONSTRUIDO (no re-indexa).
func runLLMFeatureDiscoverySilently() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.Provider == "ollama" {
		if _, _, err := llm.ResolveOllamaModel(cfg.OllamaURL, cfg.EffectiveOllamaModel()); err != nil {
			fmt.Printf("  ⚠️  Ollama no disponible en %s; se omite descubrimiento de features.\n", cfg.OllamaURL)
			return nil
		}
	}

	mc, _, err := config.LoadModelsConfig()
	if err != nil {
		return fmt.Errorf("load models config: %w", err)
	}
	if mc.Semantic.Provider != "ollama" {
		fmt.Println("  Semantic provider no es Ollama; se omite descubrimiento LLM.")
		return nil
	}

	baseURL := mc.Semantic.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	// Auto-resolver el modelo igual que en semantic index
	model := mc.Semantic.ChatModel
	if model == "" {
		model = cfg.EffectiveOllamaModel()
	}
	resolved, autoSelected, err := llm.ResolveOllamaModel(baseURL, model)
	if err != nil {
		fmt.Printf("  ⚠️  No se pudo resolver modelo Ollama: %v\n", err)
		return nil
	}
	if autoSelected {
		fmt.Printf("  Ollama tiene un solo modelo; usando %s (config tenía %s)\n", resolved, model)
	}
	model = resolved

	dbPath := filepath.Join(".androideai", "core.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("error opening database: %w", err)
	}
	defer s.Close()

	sem := semantic.NewSemantic(s.DB(), baseURL, model)
	if !sem.IsAvailable() {
		fmt.Println("  Ollama no disponible; se omite descubrimiento de features.")
		return nil
	}

	fmt.Println("  Analizando archivos con LLM para detectar features MVVM...")
	fileToFeature, err := sem.DiscoverFeatures()
	if err != nil {
		return fmt.Errorf("LLM feature discovery failed: %w", err)
	}

	if len(fileToFeature) == 0 {
		fmt.Println("  No se detectaron features nuevas (quizás el proyecto está vacío o no sigue patrones MVVM)")
		return nil
	}

	tagged, err := sem.TagSymbolsWithFeatures(fileToFeature)
	if err != nil {
		return fmt.Errorf("failed to tag features: %w", err)
	}

	fmt.Printf("  LLM feature discovery: etiquetados %d símbolos en %d archivos\n", tagged, len(fileToFeature))
	for feat := range fileToFeature {
		fmt.Printf("    Descubierto feature: %s\n", feat)
	}

	return nil
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
		if _, _, err := llm.ResolveOllamaModel(cfg.OllamaURL, cfg.EffectiveOllamaModel()); err != nil {
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
		if resolved, _, err := llm.ResolveOllamaModel(cfg.OllamaURL, cfg.EffectiveOllamaModel()); err == nil {
			cfg.OllamaModel = resolved
		}
	}
	sem := semantic.NewSemantic(s.DB(), cfg.OllamaURL, cfg.EffectiveOllamaModel())

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
	initCmd.Flags().BoolVar(&initNoLLMFeatures, "no-llm-features", false, "Saltar el descubrimiento de features con LLM")
	initCmd.Flags().BoolVar(&initWithLLMFeatures, "with-llm-features", false, "Habilitar descubrimiento de features con LLM (requiere Ollama con modelo capaz)")
	initCmd.Flags().BoolVar(&initNoBrainSeed, "no-brain-seed", false, "No sembrar el brain con las convenciones detectadas")
}
