# agent.md — Reglas del proyecto para OpenCode

> **Propósito:** este archivo es el **contexto persistente** que OpenCode debe mantener mientras
> construye **androideai-core** (equivale a `AGENTS.md`/reglas del repo). Si tu config de OpenCode
> usa `AGENTS.md`, copia o referencia este contenido vía `instructions` en `opencode.json`.
> No confundir con el system prompt del agente *del producto*, que se especifica en
> `proposal.md §8` y se implementa en `internal/agent/prompt.go`.

## Qué estamos construyendo
androideai-core: un CLI **independiente en Go** que es un agente de desarrollo Android
offline-first (índice SQLite, memoria de proyecto, ops Gradle/adb, loop de agente con Ollama,
skills extensibles). Lee `proposal.md` para el diseño y `task.md` para el plan por fases.

## Stack y restricciones (no negociables)
- **Lenguaje:** Go 1.22+. **No** Node, **no** Python en el core.
- **CGO_ENABLED=0** siempre. Driver SQLite: **`modernc.org/sqlite`** (puro Go, FTS5 incluido).
  Nunca `mattn/go-sqlite3` ni nada que requiera CGO (rompería la compilación cruzada).
- **CLI:** `spf13/cobra`. **Config:** YAML (`gopkg.in/yaml.v3`).
- **MCP:** SDK oficial `github.com/modelcontextprotocol/go-sdk`.
- **LLM:** abstracción `internal/llm` con interfaz `Provider`. Implementación por defecto: Ollama
  (HTTP localhost). **No reimplementes un modelo**; solo orquestas llamadas + tool-calling.
- Sin dependencias de red en tiempo de ejecución salvo proveedores LLM externos opcionales.
  Ollama y embeddings van por `localhost`.

## Estructura del repo (respétala)
- `cmd/` = capa fina de Cobra; **sin lógica de negocio**. Toda la lógica vive en `internal/`.
- `internal/{config,store,index,semantic,brain,android,llm,agent,skills,mcpclient}`.
- `skills/` = skills integradas, embebidas con `embed.FS`.
- `testdata/` = mini proyecto Android para tests del indexador.
- Datos de runtime: global en `~/.androideai/`, proyecto en `./.androideai/` (DB en `core.db`,
  añadida a `.gitignore`).

## Convenciones de código Go
- `gofmt` + `go vet` + `golangci-lint` deben pasar limpios. Sin warnings.
- Errores: envolver con `fmt.Errorf("...: %w", err)`; no entierres errores.
- Acepta `context.Context` en operaciones que hagan I/O (DB, exec, HTTP).
- SQL parametrizado siempre (nada de concatenar strings en queries).
- Nombres claros en inglés para identificadores; comentarios pueden ir en español.
- Paquetes pequeños y con una sola responsabilidad; evita ciclos de import.

## Diseño que debe cumplirse
- **Migraciones** idempotentes y versionadas en `internal/store`.
- **Indexado incremental** por hash de archivo (no reindexar lo que no cambió).
- **Skills extensibles sin recompilar:** el loader descubre carpetas en
  proyecto/global/embebidas (precedencia proyecto > global > integradas). Añadir una skill = soltar
  una carpeta con `SKILL.md`. Verifica esto explícitamente en la Fase 7.
- **Aprobación:** toda escritura del agente (`write_file`) y `brain save` pasan por confirmación
  cuando `approval = ask` (default). `--yes`/`auto` solo para CI.
- **Degradación elegante:** si Ollama no está disponible, `semantic` cae a FTS; el resto del CLI
  sigue funcionando.

## Flujo de trabajo de construcción (OpenCode)
1. Trabaja por fases según `task.md`, en orden. No saltes fases.
2. Antes de codificar una fase: explora el código existente, propón el plan, espera aprobación.
3. Implementa con tests. Cada fase tiene una verificación; córrela antes de cerrarla.
4. Ejecuta `go build ./...`, `go vet ./...`, `golangci-lint run` y `go test ./...` antes de
   dar una fase por terminada.
5. Commits pequeños y descriptivos por unidad de trabajo.

## Definición de "hecho" (por fase)
- Compila (`go build ./...`), pasa `vet`/lint y `go test ./...`.
- La verificación de la fase en `task.md` se cumple de forma reproducible.
- Sin TODOs colgando que rompan la funcionalidad de la fase.

## Tests
- `store`, `index`, `brain`, `skills` deben tener cobertura de unit tests.
- Usa `testdata/` para el indexador. Mockea `exec` de Gradle/adb donde sea posible.
- Tests deben correr con `CGO_ENABLED=0`.

## Cosas que NO debes hacer
- No introducir CGO ni drivers SQLite basados en C.
- No meter lógica de negocio en `cmd/`.
- No reimplementar capacidades de modelo/inferencia: usa `internal/llm`.
- No hacer que el agente escriba archivos sin pasar por el gate de aprobación.
- No reconstruir un framework de agente externo (OpenCode es solo el constructor).

## Comandos útiles
```bash
go build ./...
go vet ./...
golangci-lint run
go test ./...
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/androideai.exe   # cross-compile check
```
