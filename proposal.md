# androideai-core — Propuesta Técnica

> **Qué es esto:** especificación del producto **androideai-core**, una herramienta CLI
> **independiente** (binario Go) que actúa como agente de desarrollo Android offline-first.
> Este documento, junto con `task.md` y `agent.md`, se entrega a OpenCode para que **construya**
> androideai-core. OpenCode es el asistente de construcción; androideai-core es el producto.

---

## 1. Visión

androideai-core es un **agente de desarrollo guiado para Android** (Kotlin / Jetpack Compose),
distribuible como **un único binario** y capaz de operar **sin red**. Combina:

- **Exploración semántica del código** sobre un índice SQLite (navegar features Android rápido).
- **Memoria del proyecto** (decisiones, patrones, workarounds) con flujo de aprobación.
- **Operaciones Android** (Gradle, adb, emulador, tests).
- **Generación de código guiada** mediante un loop de agente respaldado por LLM local (Ollama)
  o proveedores externos.
- **Sistema de skills extensible**: añadir una skill = soltar una carpeta, sin recompilar.

El objetivo central es **reunir el contexto correcto antes de actuar** y **acelerar la creación
de features** ubicando automáticamente las capas (Screen → ViewModel → UseCase → Repository →
módulo DI → ruta de navegación → tests).

---

## 2. Decisión de stack: Go

| Requisito | Cómo lo cumple Go |
|---|---|
| Offline-first, distribución simple | Binario estático único, sin runtime. `go build` cruza a macOS/Linux/Windows. |
| SQLite local + FTS5 | `modernc.org/sqlite` (puro Go, **sin CGO**) incluye FTS5, RTree, JSON1. |
| Búsqueda vectorial sin C | `modernc.org/sqlite/vtab` permite módulos de tabla virtual en Go (índice vectorial). |
| MCP cliente/servidor | SDK oficial `github.com/modelcontextprotocol/go-sdk/mcp`. |
| CLI con subcomandos y autocompletado | `spf13/cobra`. |
| Indexado concurrente | goroutines. |
| IA local | Ollama por HTTP (`localhost:11434`), agnóstico al lenguaje. |

> **Alternativa Node/TS:** válida (ecosistema AI más amplio, iteración rápida), pero la
> distribución requiere empaquetar runtime y el binario pesa más. Para un CLI con índice SQLite,
> Go es el default recomendado. La arquitectura lógica de abajo es portable a Node si se decide.

Restricción de construcción: **mantener `CGO_ENABLED=0`** (usar SIEMPRE `modernc.org/sqlite`,
nunca `mattn/go-sqlite3`) para preservar la compilación cruzada.

---

## 3. Arquitectura

```
androideai-core/
├── go.mod
├── main.go
├── cmd/                       # Subcomandos Cobra (capa fina; lógica en internal/)
│   ├── root.go
│   ├── init.go                # inicializa config + DB del proyecto
│   ├── index.go               # androideai index [build|refresh]
│   ├── search.go              # androideai search | symbol | feature
│   ├── brain.go               # androideai brain save|search|review|promote
│   ├── android.go             # androideai gradle|emulator|test
│   ├── agent.go               # androideai agent  (loop de desarrollo guiado)
│   ├── skills.go              # androideai skills list|add|path
│   └── mcp.go                 # androideai mcp serve  (modo servidor MCP, opcional)
├── internal/
│   ├── config/                # carga config global (~/.androideai) y de proyecto (.androideai)
│   ├── store/                 # SQLite: schema, migraciones, queries (modernc.org/sqlite)
│   ├── index/
│   │   ├── walker.go          # recorre **/*.kt, **/*.kts (respeta .gitignore)
│   │   ├── kotlin.go          # extracción de símbolos (heurística v1 / tree-sitter v2)
│   │   └── feature.go         # mapeo de feature → capas
│   ├── semantic/              # embeddings (Ollama) + vector store (vtab o brute-force)
│   ├── brain/                 # operaciones de memoria del proyecto
│   ├── android/               # ejecución de gradlew / adb / emulator + parseo de salida
│   ├── llm/                   # abstracción de proveedor: ollama | anthropic | openai-compat
│   ├── agent/
│   │   ├── loop.go            # ciclo explorar→planear→confirmar→implementar→verificar→aprender
│   │   ├── tools.go           # registro de tools que el agente puede invocar
│   │   └── prompt.go          # system prompt del agente (ver §8)
│   ├── skills/                # cargador/registro dinámico de skills
│   └── mcpclient/             # cliente MCP (go-sdk) hacia servidores externos
├── skills/                    # skills integradas, embebidas con embed.FS
│   ├── android-feature/SKILL.md
│   ├── android-debug/SKILL.md
│   └── android-test/SKILL.md
└── testdata/                  # proyecto Android de muestra para tests del indexador
```

Datos en runtime:
- Global: `~/.androideai/config.yml`, `~/.androideai/skills/`
- Por proyecto: `./.androideai/config.yml`, `./.androideai/core.db`, `./.androideai/skills/`
- `./.androideai/core.db` va al `.gitignore` (o solo se versiona conocimiento promovido vía export).

---

## 4. Esquema SQLite

```sql
-- Índice de código
CREATE TABLE files (
  id INTEGER PRIMARY KEY, path TEXT UNIQUE NOT NULL,
  package TEXT, module TEXT, layer TEXT,   -- ui|domain|data|di|nav|test
  hash TEXT, updated_at INTEGER
);
CREATE TABLE symbols (
  id INTEGER PRIMARY KEY,
  file_id INTEGER REFERENCES files(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,        -- composable|screen|viewmodel|usecase|repository|dao|module|route|class|fun
  signature TEXT, line INTEGER, feature TEXT
);
CREATE VIRTUAL TABLE symbols_fts USING fts5(name, signature, package, path, doc, content='');

-- Memoria del proyecto
CREATE TABLE knowledge_entries (
  id INTEGER PRIMARY KEY,
  type TEXT NOT NULL,        -- decision|pattern|workaround|gotcha
  title TEXT NOT NULL, content TEXT NOT NULL,
  tags TEXT, file_refs TEXT,
  status TEXT DEFAULT 'temp',-- temp|promoted
  created_at INTEGER
);
CREATE VIRTUAL TABLE knowledge_fts USING fts5(title, content, tags, content='');

-- Embeddings opcionales (semantic). vector como BLOB float32 LE.
CREATE TABLE embeddings (
  symbol_id INTEGER REFERENCES symbols(id) ON DELETE CASCADE,
  model TEXT, dim INTEGER, vector BLOB
);

-- Operativos
CREATE TABLE build_history (id INTEGER PRIMARY KEY, task TEXT, status TEXT, log TEXT, ts INTEGER);
```

FTS5 está disponible directamente en `modernc.org/sqlite` (sin compilar nada extra).

---

## 5. Semantic Code Exploration

Objetivo: que el agente ubique los archivos relevantes **sin grep ciego**.

1. **Indexado** (`index build`): recorre `.kt`/`.kts`, calcula hash (reindexado incremental),
   infiere `module`/`layer`/`package`, extrae símbolos y repuebla FTS.
2. **Extracción de símbolos**:
   - v1 heurística (regex): `@Composable fun`, `: ViewModel`/`@HiltViewModel`, `@Module`,
     `@Dao`, `interface *Repository` / `class *RepositoryImpl`, `composable("...")` en NavHost.
   - v2 precisa: **tree-sitter-kotlin**. En Go se puede vía `github.com/smacker/go-tree-sitter`
     o un sidecar invocado por exec. Migrar en fase posterior.
3. **Consultas**:
   - `search <kw>`: FTS5 → `path:line` + snippet.
   - `symbol <name|kind>`: filtra `symbols`.
   - `feature <nombre>`: agrupa todas las capas de una feature (el acelerador para crear features).
4. **Semántica opcional** (`semantic`): embeddings vía Ollama (`/api/embeddings`,
   `nomic-embed-text`), similitud coseno (brute-force en Go o `vtab`). Si Ollama no está, degrada a FTS.

---

## 6. Memoria del proyecto (brain)

`brain save|search|review|promote` sobre `knowledge_entries`.
- `save`: por defecto entra como `temp` y **requiere confirmación** del usuario (flag `--yes` para CI).
- `review`: lista `temp`; `promote`: pasa a `promoted`.
- Export/import a Markdown versionable para compartir conocimiento en el repo.

---

## 7. Operaciones Android

`internal/android` envuelve `./gradlew`, `adb`, `emulator`:
- `gradle <task>`: ejecuta y **parsea** la salida (primer error de compilación estructurado).
- `test [--unit|--instrumented]`: corre y resume fallos.
- `emulator [list|start|stop]`.
Registra resultados en `build_history`.

---

## 8. Loop del agente (desarrollo guiado)

`internal/agent` implementa el ciclo y su system prompt (`prompt.go`):

```
1. EXPLORAR  → index.feature / brain.search para reunir contexto
2. PLANEAR   → propone archivos/cambios; NO escribe aún
3. CONFIRMAR → aprobación del usuario (configurable: ask por defecto)
4. IMPLEMENTAR → genera/edita siguiendo las convenciones del proyecto
5. VERIFICAR → android.test (+ lint)
6. APRENDER  → propone brain.save de decisiones/gotchas
```

- **Tool registry** (`tools.go`): el LLM puede invocar `read_file`, `index_search`,
  `index_feature`, `brain_search`, `gradle`, `test`, `write_file` (gated por aprobación).
- **`llm`**: abstracción de proveedor. Default offline: Ollama (p. ej. `qwen2.5-coder`).
  Opcional: Anthropic / OpenAI-compatible. **No reimplementar un LLM**; solo orquestar.

---

## 9. Sistema de skills (extensible sin recompilar)

Una skill es una carpeta con `SKILL.md` (frontmatter YAML) y, opcionalmente, plantillas/scripts.

```
android-feature/
├── SKILL.md          # name, description, triggers, (opcional) command/templates
└── templates/        # plantillas de Screen/ViewModel/etc. (opcional)
```

`internal/skills` descubre skills de tres fuentes (precedencia: proyecto > global > integradas):
1. Integradas en el binario (`embed.FS`, carpeta `skills/`).
2. Globales: `~/.androideai/skills/`.
3. De proyecto: `./.androideai/skills/`.

Frontmatter mínimo:
```yaml
---
name: android-feature
description: Crea una feature MVVM/Compose ubicando archivos en el módulo/paquete correctos.
triggers: ["crear feature", "nueva pantalla", "add screen"]
---
```

El agente lista las skills disponibles y carga el contenido **bajo demanda**. Añadir una skill
nueva no requiere tocar Go ni recompilar: se coloca la carpeta y `skills list` la detecta.

---

## 10. Integraciones

- **Ollama**: inferencia y embeddings locales (offline). Configurable en `config.yml`.
- **MCP**: cliente hacia servidores externos (`internal/mcpclient`, go-sdk oficial) y, opcional,
  `androideai mcp serve` para exponer las tools de androideai-core a otros agentes.
- **Android SDK / Gradle / adb**: detección de rutas vía `ANDROID_HOME`/`local.properties`.

---

## 11. CLI (resumen)

```bash
androideai init                       # crea config + DB del proyecto
androideai index build                # indexa el repo
androideai search "token refresh"     # búsqueda FTS
androideai feature login              # capas de la feature "login"
androideai brain save --type decision --title "DI con Hilt" --content "..."
androideai brain search "DI"
androideai gradle assembleDebug
androideai test --unit
androideai emulator start Pixel_7_API_34
androideai agent "añade pantalla de recuperar contraseña"   # loop guiado
androideai skills list
androideai mcp serve                  # opcional
```

---

## 12. Distribución

- `go build` → binarios por plataforma; release con GoReleaser.
- Instaladores: Homebrew tap, script `curl | sh`, `go install`.
- Sin dependencias de runtime (CGO_ENABLED=0).

---

## 13. Fases

- **F1 Cimientos:** módulo Go, Cobra, `store` (schema+migraciones), `config`, `init`.
- **F2 Índice:** `walker`, `kotlin` (heurística), `index build/search/symbol`.
- **F3 Features:** `feature` (mapeo de capas) + skill `android-feature`.
- **F4 Memoria:** `brain` completo + aprobación; export/import Markdown.
- **F5 Android ops:** `gradle/test/emulator` con parseo + `build_history`.
- **F6 Agente:** `llm` (Ollama), tool registry, loop guiado, `prompt`.
- **F7 Skills dinámicas:** loader con embed.FS + dirs global/proyecto; `skills` cmds.
- **F8 Semántica:** embeddings Ollama + vector store (vtab/brute-force).
- **F9 MCP:** cliente + (opcional) modo servidor.
- **F10 Precisión:** tree-sitter-kotlin en el indexador.

(El desglose accionable está en `task.md`.)

---

## 14. Riesgos / decisiones abiertas

- **CGO:** no usar drivers que requieran CGO; quedarse en `modernc.org/sqlite`.
- **Parser Kotlin:** heurística rápida pero imperfecta → migrar a tree-sitter (F10).
- **Vector store:** `vtab` evita C pero es más trabajo; brute-force cosine es simple para repos medianos.
- **LLM provider:** definir interfaz estable en `internal/llm` para no acoplarse a Ollama.
- **Modelo local:** elegir default (p. ej. `qwen2.5-coder`) y documentar requisitos de RAM.
