# androideai-core — Plan de Construcción (task.md)

> Para el agente constructor (OpenCode): construye **una fase a la vez**, en orden. Al final de
> cada fase corre la verificación y detente para confirmación. Respeta `agent.md` (reglas del
> repo). Stack: **Go**, `CGO_ENABLED=0`. No reimplementes un LLM: usa la abstracción `internal/llm`.

## Fase 0 — Bootstrap
- [ ] `go mod init github.com/<org>/androideai-core` (Go 1.22+).
- [ ] Añadir deps: `spf13/cobra`, `modernc.org/sqlite`, `gopkg.in/yaml.v3`,
      `github.com/modelcontextprotocol/go-sdk` (MCP, fase posterior).
- [ ] `main.go` que ejecute `cmd.Execute()`.
- [ ] Configurar lint/format: `gofmt`, `go vet`, `golangci-lint`.
- [ ] **Verificación:** `go build ./...` y `go vet ./...` sin errores.

## Fase 1 — Cimientos (config + store + init)
- [ ] `internal/config`: cargar config global (`~/.androideai/config.yml`) y de proyecto
      (`./.androideai/config.yml`), con merge (proyecto gana). Campos: `model`, `ollama_url`,
      `provider`, `approval` (`ask|auto`).
- [ ] `internal/store`: abrir SQLite (`modernc.org/sqlite`) en `./.androideai/core.db`;
      migraciones idempotentes con TODO el esquema de `proposal.md §4` (incluye tablas FTS5).
- [ ] `cmd/init.go`: `androideai init` crea `.androideai/`, config y DB; añade `core.db` a `.gitignore`.
- [ ] **Verificación:** `androideai init` crea la DB; abrir con un test que valide que las tablas
      y las virtual tables FTS5 existen.

## Fase 2 — Índice de código (walker + kotlin + queries básicas)
- [ ] `internal/index/walker.go`: recorre `**/*.kt` y `**/*.kts` respetando `.gitignore`.
- [ ] `internal/index/kotlin.go`: extracción heurística de símbolos (composable, screen,
      viewmodel, usecase, repository, dao, module, route) + inferencia de `module`/`layer`/`package`.
- [ ] `cmd/index.go`: `index build` (incremental por hash) e `index refresh`.
- [ ] `cmd/search.go`: `search <kw>` (FTS5) y `symbol <name|kind>`.
- [ ] Tests con `testdata/` (mini proyecto Android de muestra).
- [ ] **Verificación:** `androideai index build` puebla `files`/`symbols`/`symbols_fts`;
      `androideai search "ViewModel"` devuelve `path:line`.

## Fase 3 — Navegación de features
- [ ] `internal/index/feature.go`: dado un nombre, agrupa capas (Screen, ViewModel, UseCase,
      Repository, módulo DI, ruta nav, tests) usando module/package/kind.
- [ ] `cmd` `feature <nombre>` que imprima el set agrupado.
- [ ] Skill integrada `skills/android-feature/SKILL.md` (frontmatter + flujo).
- [ ] **Verificación:** `androideai feature login` lista las capas de una feature de `testdata`.

## Fase 4 — Memoria (brain)
- [ ] `internal/brain`: CRUD sobre `knowledge_entries` + búsqueda vía `knowledge_fts`.
- [ ] `cmd/brain.go`: `save` (entra `temp`, pide confirmación salvo `--yes`), `search`,
      `review`, `promote`. Export/import a Markdown.
- [ ] **Verificación:** `brain save ...` pide confirmación; `brain search` la recupera;
      `brain promote` cambia el estado.

## Fase 5 — Operaciones Android
- [ ] `internal/android`: wrappers de `./gradlew`, `adb`, `emulator`; parsear salida (primer
      error de compilación estructurado) y registrar en `build_history`.
- [ ] `cmd/android.go`: `gradle <task>`, `test [--unit|--instrumented]`, `emulator [list|start|stop]`.
- [ ] **Verificación:** contra `testdata` (o mocks), `test --unit` ejecuta y resume resultados.

## Fase 6 — Loop del agente (desarrollo guiado)
- [ ] `internal/llm`: interfaz `Provider` (Complete/Chat con tool-calling) + implementación
      `ollama` (HTTP localhost). Stubs para `anthropic`/`openai-compat`.
- [ ] `internal/agent/tools.go`: registro de tools (`read_file`, `index_search`, `index_feature`,
      `brain_search`, `gradle`, `test`, `write_file` con gate de aprobación).
- [ ] `internal/agent/prompt.go`: system prompt con el ciclo explorar→planear→confirmar→
      implementar→verificar→aprender (ver `proposal.md §8`).
- [ ] `internal/agent/loop.go` + `cmd/agent.go`: `androideai agent "<tarea>"`.
- [ ] **Verificación:** con Ollama activo, `agent "añade pantalla X"` reúne contexto, propone un
      plan y **espera aprobación** antes de escribir.

## Fase 7 — Skills dinámicas
- [ ] `internal/skills`: cargador desde `embed.FS` (integradas) + `~/.androideai/skills/` +
      `./.androideai/skills/`; parseo de frontmatter; precedencia proyecto>global>integradas.
- [ ] `cmd/skills.go`: `skills list`, `skills add <path>`, `skills path`.
- [ ] El agente expone las skills disponibles y carga contenido bajo demanda.
- [ ] **Verificación:** crear una carpeta de skill nueva en `./.androideai/skills/` y comprobar
      que `skills list` la muestra **sin recompilar**.

## Fase 8 — Búsqueda semántica (opcional)
- [ ] `internal/semantic`: embeddings vía Ollama (`/api/embeddings`), almacenamiento en
      `embeddings`, similitud coseno (brute-force; opcional `vtab`).
- [ ] Comandos `semantic index` / `semantic search`; degradación a FTS si Ollama no responde.
- [ ] **Verificación:** consulta por significado (no por palabra exacta) devuelve resultados con Ollama.

## Fase 9 — MCP
- [ ] `internal/mcpclient`: cliente MCP (go-sdk) hacia servidores configurados.
- [ ] (Opcional) `cmd/mcp.go`: `androideai mcp serve` que exponga las tools del producto.
- [ ] **Verificación:** conectar a un servidor MCP de prueba y listar/llamar una tool.

## Fase 10 — Precisión del parser
- [ ] Sustituir heurística por tree-sitter-kotlin (`go-tree-sitter` o sidecar).
- [ ] Reindexar `testdata` y comparar conteo de símbolos vs heurística.

---

## Criterios de aceptación globales
- [ ] `go build` produce un binario único; `CGO_ENABLED=0` funciona (cross-compile a 3 OS).
- [ ] Añadir una skill = soltar una carpeta, sin recompilar.
- [ ] `feature <nombre>` devuelve las capas agrupadas de una feature existente.
- [ ] El loop `agent` no escribe sin aprobación (modo `ask`).
- [ ] Todo funciona offline salvo proveedores LLM externos opcionales; Ollama/embeddings por localhost.
- [ ] Cobertura de tests en `store`, `index`, `brain` y `skills`.
