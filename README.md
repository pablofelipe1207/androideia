# androideai-core

Offline-first AI agent for Android development with semantic code exploration, official Android skills, and automatic knowledge storage.

## Installation

### Quick Install (Linux/macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/pablofelipe1207/androideia/main/install.sh | bash
```

### Using Make

```bash
git clone https://github.com/pablofelipe1207/androideia.git
cd androideia
make install
```

### Using Go

```bash
go install github.com/pablofelipe1207/androideia@latest
```

### Manual Download

Download the latest binary from [Releases](https://github.com/pablofelipe1207/androideia/releases) and add it to your PATH.

### Build from Source

```bash
# Build without tree-sitter (pure Go, no CGO required)
make build

# Build with tree-sitter (requires CGO and C compiler)
make build-full
```

## Quick Start

```bash
# Initialize a project (also runs index build, semantic index, and
# seeds the brain with detected conventions — see "What init does").
androideai init

# Import official Android skills
androideai skills import-android

# Start the agent
androideai agent "Create a login feature with MVVM architecture"

# Later: list past sessions and resume one of them
androideai memory list
androideai agent --resume 1 "now add unit tests for the ViewModel"
```

## Features

### Core Features
- **Code Indexing**: Fast symbol search with tree-sitter parser
- **Feature Navigation**: Find all layers of a feature (Screen, ViewModel, UseCase, etc.)
- **Knowledge Base**: Save and search project decisions and patterns
- **Agent Loop**: AI-guided development with approval gate and a dedicated `confirm_plan` tool
- **Agent Memory**: Every run is persisted; resume with `androideai agent --resume <id>` and inspect with `androideai memory list/show`
- **Skills System**: Extensible without recompilation
- **Android Operations**: Gradle, tests, emulator management

### Advanced Features
- **Semantic Code Exploration**: Find code by meaning, not just keywords
- **Official Android Skills**: Import and use skills from https://github.com/android/skills
- **Automatic Knowledge Storage**: Agent stores knowledge when tasks complete successfully
- **Context-Aware Agent**: Searches knowledge base before executing tasks

## Agent Commands

```bash
# Start the agent with a task
androideai agent "Your task here"

# Examples:
androideai agent "Create a User data class with id, name, email"
androideai agent "Make my app adaptive for tablets"
androideai agent "Start the emulator and install my app"

# Override the model for this run (does not change config files)
androideai agent "Refactor the auth flow" --model qwen2.5-coder:7b
# or: -m llama3.2:3b

# Resume a previous conversation (see "Agent Memory" below)
androideai agent --resume 7 "now add the forgot-password screen"
```

### Confirmations

The agent never writes files or executes destructive actions without your
approval. The LLM is instructed to call a dedicated `confirm_plan` tool
instead of asking in plain text; if the model ever forgets, the loop
detects confirmation phrases ("please confirm", "should I proceed",
"¿confirmas?", "¿procedo?", …) and re-prompts you before continuing.

When prompted you can answer:

| Input | Effect |
|-------|--------|
| `y` / `yes` / `s` / `si` | Approve and continue |
| `n` / `no` / empty | Reject; session stays open and is marked `interrupted` |
| `e` | Edit; you are asked for a new plan |
| any other text | Approve with the text forwarded as feedback |

### Project context (package name & dependency versions)

When you launch the agent inside an Android project, the CLI discovers
the real `applicationId` / `namespace` and the contents of
`gradle/libs.versions.toml` automatically and injects them as a
`## Project context` block at the start of the conversation. The agent
is required to:

- Use the real applicationId/namespace for every new Kotlin/Java file
  (so the `package` line and the file path always agree, and the IDE
  doesn't end up with files under a made-up package like
  `com.example.yourapp`).
- Match the package prefix of existing activities (e.g. if
  `MainActivity` is `com.example.myapplication.MainActivity`, new files
  go under `com.example.myapplication.<feature>...`).
- Reuse versions and libraries already declared in
  `gradle/libs.versions.toml` via `version.ref` / library alias instead
  of adding duplicate entries.

If the project has no `app/src/main/AndroidManifest.xml` (e.g. library
modules only) or no `gradle/libs.versions.toml`, the agent will ask
you via `ask_user` for the applicationId before writing any file.

## Agent Memory

Every agent run is persisted to `.androideai/core.db` as a `conversation`
with its full message history (system, user, assistant, tool calls and
tool results). You can inspect past sessions, resume them or delete them.

```bash
# List past conversations (most recent first)
androideai memory list
# or with a custom limit
androideai memory list -n 50

# Show every message of a conversation
androideai memory show 7

# Resume a conversation: the original task is restored, and your
# new message is appended as a user turn. The LLM keeps the full context.
androideai agent --resume 7 "now add the forgot-password screen"

# Mark a session as completed by hand
androideai memory complete 7

# Reopen a completed session
androideai memory reopen 7

# Delete a single conversation
androideai memory delete 7

# Delete every conversation (asks for confirmation unless -y)
androideai memory purge -y
```

Sessions transition between three states:

| Status | Meaning |
|--------|---------|
| `active` | The loop is currently running or was interrupted (Ctrl-C, denied plan, error, or the user simply hit Enter at the end-of-task prompt). Resumable. |
| `completed` | The agent finished a task successfully **and** the user confirmed. Listed for reference; you can still resume it. |
| `interrupted` | The session was cut short. Always resumable with `--resume`. |

When a run finishes, the agent prints the conversation ID and a hint
such as:

```
[Memory] Conversación guardada con ID 7.
  Ver mensajes:    androideai memory show 7
  Continuar:       androideai agent --resume 7 "<nuevo mensaje>"
  Cerrar/eliminar: androideai memory delete 7

Si los archivos no aparecen en Android Studio, haz click
derecho en la raíz del proyecto → 'Synchronize' (o usa
'File → Sync Project with Gradle Files').
```

### Who decides when a task is done?

The agent never marks a session as `completed` on its own. When the
loop ends without errors, the agent asks:

```
¿La tarea quedó completada a tu satisfacción?
[y=marcar como completada / N o Enter=mantener activa para continuar luego]
```

Only answering `y`/`yes` flips the session to `completed`. Anything
else (including a blank Enter) leaves it as `active` so you can
`--resume` it later. If the user denies a plan, cancels, or an error
occurs, the session is marked `interrupted` automatically.

### Tool call fallback

The LLM is instructed to invoke tools through the native tool calling
mechanism (the `tool_calls` field of the response). As a safety net, the
loop also scans the assistant message for JSON objects of the form
`{"name": "...", "arguments": {...}}` and executes them as if they had
been declared natively. This means a model that "prints" its tool calls
will still write the files and create the resources it describes.

### Android Studio / IntelliJ file refresh

Android Studio does not always pick up new files immediately. After the
agent finishes a run that created files, if they do not appear in the
Project view:

- Right-click the project root → **Synchronize**, or
- **File → Sync Project with Gradle Files** (or **Invalidate Caches…**
  if Synchronize does not work).

This is an IDE limitation, not an agent bug — the files exist on disk
and will be picked up by Gradle on the next build regardless.

## Skills Management

```bash
# List all available skills
androideai skills list

# Import official Android skills
androideai skills import-android

# Import a specific Android skill
androideai skills import-android-skill adaptive

# Import skills from opencode
androideai skills import-opencode

# Show skill content
androideai skills show adaptive

# Search skills by trigger
androideai skills search "compose"
```

### Official Android Skills

| Skill | Description |
|-------|-------------|
| `adaptive` | UI adaptativa para diferentes dispositivos |
| `agp-9-upgrade` | Migración a Android Gradle Plugin 9 |
| `android-cli` | Orquestación de tareas de desarrollo Android |
| `appfunctions` | Análisis de apps para AppFunctions |
| `camera1-to-camerax` | Migración de Camera1 a CameraX |
| `edge-to-edge` | Migración a edge-to-edge |
| `jetpack-compose-m3` | Wear OS Compose Material3 |
| `migrate-xml-views-to-jetpack-compose` | Migración XML a Compose |
| `navigation-3` | Migración a Jetpack Navigation 3 |
| `r8-analyzer` | Análisis de R8 keep rules |
| `testing-setup` | Estrategia de testing para Android |
| `verified-email` | Implementación de email verificado |

## Android Operations

```bash
# List emulators
androideai android emulator list

# Start emulator
androideai android emulator start Pixel_9_Pro

# Check emulator status
androideai android emulator status

# Run Gradle task
androideai android gradle assembleDebug

# Run tests
androideai android test --unit
```

## Semantic Code Exploration

The semantic module keeps **two complementary indexes** for every Android
project:

1. **Symbol embeddings** (the original behavior) — every Kotlin symbol
   gets a vector so you can search code by meaning.
2. **LLM file classification** — for every `.kt`/`.kts` file the LLM
   produces a structured record with:
   - `type` (`viewmodel`, `activity`, `composable`, `usecase`,
     `repository`, `dao`, `di_module`, `nav_route`, `data_class`,
     `entity`, `service`, `application`, `test`, `build`, `other`)
   - `tags` (1-6 kebab-case keywords describing the feature/role)
   - `architecture` (the project pattern the file hints at: `MVVM`,
     `MVI`, `Clean`, `MVP`, `unknown`, ...)
   - `conventions` (how files of that role are written in THIS project:
     DI style, state holder, async pattern, etc.)
   - `summary` (one-sentence purpose)

The agent queries this index with the `semantic_locate` tool *before*
creating any new file, so it never reinvents a pattern that already
exists, and never collides with a class/module that the project
already has.

```bash
# Check the index status (classified files, architecture, top types)
androideai semantic status

# Run both passes: LLM classification + symbol embeddings
androideai semantic index

# Search code by meaning (embeddings)
androideai semantic search "user authentication patterns"

# Locate existing files — this is what the agent uses internally:
androideai semantic locate viewmodel          # every ViewModel in the project
androideai semantic locate usecase            # every UseCase
androideai semantic locate repository         # every Repository / DAO
androideai semantic locate di_module          # Hilt/Dagger modules
androideai semantic locate LoginViewModel     # a specific file
androideai semantic locate tag:auth           # any file tagged "auth"
androideai semantic locate "state flow"       # free-text on conventions/summary

# Show all matches (up to 200) and the project architecture summary
androideai semantic locate viewmodel --all
```

The `locate` command prints for every match: path, package, layer,
module, type, tags, detected architecture, summary and the
"conventions" snippet so you can see at a glance how a given role is
written in your project (e.g. "Hilt constructor injection, exposes
StateFlow<UiState>, no Android dependencies").

## What `androideai init` does

`androideai init` is the one-shot bootstrap for a new project. In a
single command it does, in order:

1. Creates `.androideai/`, `.gitignore`, `config.yml` and the SQLite
   database with the full schema.
2. Runs `androideai index build` (parses every `.kt`/`.kts` and
   populates `files`, `symbols`, the FTS index).
3. Runs `androideai semantic index` — but only if Ollama is reachable.
   It calls the LLM per file to classify it (ViewModel, Activity,
   UseCase, Repository, ...) and stores tags, architecture and
   conventions. If Ollama is down, this step is skipped with a warning
   and the rest of init still succeeds.
4. Aggregates the detected conventions by `type` and seeds the brain
   with one `convention` entry per role (e.g. *ViewModel convention*,
   *UseCase convention*, *Repository convention*). After this, the
   agent can do `brain_search "viewmodel convention"` before creating a
   new file and follow the project's actual style.

Idempotent: re-running `init` is safe. The brain uses
`SaveIfNotExists` keyed on the entry title, so existing convention
entries are not duplicated; the message is "Las convenciones ya
estaban en el brain; nada nuevo que añadir."

Flags:

```bash
androideai init                   # full bootstrap (default)
androideai init --no-index        # skip index build
androideai init --no-semantic     # skip LLM classification + embeddings
androideai init --no-brain-seed   # skip seeding the brain
```

Typical output (Ollama available):

```
Inicializando androideai-core...
  ✓ Created .androideai/config.yml
  ✓ Created .androideai/core.db with schema

→ Construyendo índice de código...
  Found 7 Kotlin files
  ✓ Indexed 7 files

→ Corriendo índice semántico (LLM classify + embeddings)...
  Clasificados: 7   Fallidos: 0
  Arquitectura detectada: MVVM
  Embeddings nuevos: 19

→ Sembrando brain con convenciones detectadas...
  ✓ Composable convention (2 archivo(s) de muestra)
  ✓ Di_module convention (1 archivo(s) de muestra)
  ✓ Repository convention (1 archivo(s) de muestra)
  ✓ Usecase convention (1 archivo(s) de muestra)
  ✓ Viewmodel convention (1 archivo(s) de muestra)
  5 nueva(s), 0 ya existente(s).
  El agente ahora puede hacer brain_search "<rol> convention" antes de crear archivos.

Initialization complete!
```

## Knowledge Base

```bash
# Save knowledge
androideai brain save --type "architecture" --title "MVVM Pattern" \
  --content "We use MVVM with ViewModel, Repository, UseCase" \
  --tags "architecture,mvvm" --promote

# Search knowledge
androideai brain search "ViewModel"

# List all knowledge
androideai brain list

# Export knowledge
androideai brain export knowledge.md
```

## Android Scaffold & Validate

To make sure the agent never writes a "half" ViewModel (just a function
with no UiState / UiEvent / UiEffect), a "pelado" Composable (no
`hiltViewModel()`), or a Repository that imports `android.*`, the
project ships with **canonical templates** and a **static validator**
for every common Android component.

The agent uses these through two tools:

- `android_scaffold` — first checks the semantic index for existing
  files of the role, then returns the canonical template filled with
  the name/feature/package you pass. Roles: `viewmodel`, `composable`,
  `activity`, `usecase`, `repository`, `dao`, `di_module`,
  `data_class`, `entity`, `nav_route`.
- `validate_kotlin` — runs the role's rules (regex/keyword checks) on
  a file you just wrote and returns a list of errors / warnings. The
  agent must call this AFTER `write_file` and BEFORE `confirm_plan`.

The same templates and rules are exposed on the CLI for ad-hoc usage:

```bash
# List supported roles
androideai scaffold list

# Print a ViewModel template to stdout
androideai scaffold viewmodel Login --feature login

# Write the template straight to a file
androideai scaffold viewmodel Login --feature login \
    --package com.example.app.feature.login \
    --output app/src/main/java/com/example/app/feature/login/LoginViewModel.kt

# All other roles
androideai scaffold composable Login   --output LoginScreen.kt
androideai scaffold usecase    Login   --output LoginUseCase.kt
androideai scaffold repository User    --output UserRepository.kt
androideai scaffold dao        User    --output UserDao.kt --table users
androideai scaffold di_module  Auth    --output AuthModule.kt
androideai scaffold data_class User    --output User.kt
androideai scaffold entity     User    --output UserEntity.kt --table users
androideai scaffold nav_route  Auth    --output AuthRoutes.kt
androideai scaffold activity   Main    --output MainActivity.kt

# Validate a file (exit code 0 = OK, 1 = errors)
androideai validate app/src/main/java/.../LoginViewModel.kt viewmodel
```

### What the validator checks (per role)

| Role           | Hard requirements                                                                                |
|----------------|---------------------------------------------------------------------------------------------------|
| `viewmodel`    | `@HiltViewModel`, extends `ViewModel()`, `@Inject constructor`, nested `UiState`, `UiEvent`, `UiEffect`, public `val state: StateFlow<UiState>`, `val effects: Flow<UiEffect>`, `fun onEvent(event: UiEvent)`, **no** `LiveData` |
| `composable`   | `@Composable`, name ends in `Screen`, uses `hiltViewModel()`, `collectAsStateWithLifecycle()`, calls `viewModel.onEvent(` |
| `activity`     | extends `ComponentActivity`/`AppCompatActivity`, `setContent {`, wrapped in `AppTheme`/`MaterialTheme`, `@AndroidEntryPoint` |
| `usecase`      | `suspend operator fun invoke(`, `@Inject constructor`, returns `Result<…>` or `Flow<…>` |
| `repository`   | `interface …Repository` + `class …RepositoryImpl : …Repository`, **no** `import android.*`, methods are `suspend` or return `Flow<…>` |
| `dao`          | `@Dao`, `interface`, methods `suspend` or `Flow<…>` |
| `di_module`    | `@Module`, `@InstallIn(XxxComponent::class)`, at least one `@Provides` / `@Binds` |
| `data_class`   | `data class`, primary-constructor params are `val` |
| `entity`       | `@Entity`, `@PrimaryKey` |
| `nav_route`    | `object …Routes` or `sealed class …Routes`, at least one `const val` route string |

The agent's mandatory workflow per new file is:

1. `android_scaffold action=check role=viewmodel name=Login` → reads an
   existing project ViewModel with `read_file` if one is found.
2. `android_scaffold action=template role=viewmodel name=Login feature=login` → fills the template's `// TODO:` blocks.
3. `write_file path=…` → writes the file.
4. `validate_kotlin path=… role=viewmodel` → iterates until OK.
5. Only then does the agent call `confirm_plan`.

## Configuration

Two configuration files are merged on every command (project wins over global):

| Scope | Path | Created by |
|-------|------|------------|
| Global | `~/.androideai/config.yml` | `install.sh` (during install) |
| Project | `./.androideai/config.yml` | `androideai init` |

```yaml
model: qwen3-coder-64k-32k:latest
ollama_url: http://localhost:11434
provider: ollama   # ollama | anthropic | openai
approval: ask      # ask | auto | never
timeout: 300       # seconds per LLM call; raise this for slow models / long contexts
```

### Managing config with the CLI

```bash
# Show the effective config (project + global merged)
androideai config show

# Read a single key
androideai config get model

# Set a value in the project config
androideai config set model qwen2.5-coder:7b
androideai config set ollama_url http://remote:11434
androideai config set provider ollama
androideai config set approval auto
androideai config set timeout 900   # 15 minutes per LLM call

# Set a value in the global config (applies to every project)
androideai config set model llama3.2:3b --global
# or: -g
```

You can also edit the YAML files directly — `loadFromFile` reads partial files,
so you only need to set the keys you want to override.

### Override the model for a single run

The `agent` command accepts `--model` (or `-m`) to override the configured
model without touching any file:

```bash
androideai agent "refactor the login flow" --model qwen2.5-coder:7b
androideai agent "explain this code" -m llama3.2:3b
```

This also works with `semantic index` / `semantic search` via the same
`ResolveOllamaModel` flow.

### Override the timeout for a single run

Local 7B+ models on CPU can take several minutes to produce a long plan
or to summarize a large conversation. The `agent` command accepts
`--timeout` (in seconds) to override the value from config:

```bash
androideai agent "explain src/agent/loop.go" --timeout 600   # 10 min
```

If a request does exceed the timeout, the session is saved as
`interrupted` and the next run prints a hint with the resume command —
nothing is lost.

While a request is in flight, the agent prints a `... still thinking
(turn N, elapsed: 1m30s) ...` line every 30 seconds so you can tell
that the LLM is still working, not stalled.

### Auto-selection when Ollama has a single model

Whenever the provider is `ollama`, the agent and semantic commands query
`GET /api/tags` and apply these rules:

| Ollama state | Behavior |
|--------------|----------|
| Exactly 1 model installed | Use it (and notify the user, e.g. `Ollama has a single model installed; using X (config had Y)`) |
| Configured model is in the list | Use the configured model |
| Multiple models and configured one is missing | Return an error listing the available models |
| Ollama is unreachable | Fall back to the configured model; `IsAvailable` will fail later with a clear message |
| Ollama has no models | Return an error suggesting `ollama pull <modelo>` |

## Models

```bash
# List the models installed in Ollama, mark the configured one with *,
# and show which one would be auto-selected
androideai models list
```

Example output:

```
Modelos disponibles en Ollama (http://localhost:11434):
* qwen2.5-coder:7b

Modelo configurado disponible: qwen2.5-coder:7b
```

When only one model is installed the configured model is overridden
automatically (see the table above).

## Requirements

- Go 1.22+ (for building from source)
- Ollama running locally (for AI agent)
- Android SDK (for emulator operations) - automatically detected or installed
- No runtime dependencies (single binary)

### Android SDK Setup

The installer automatically detects Android SDK in common locations:
- `~/Android/Sdk`
- `/usr/lib/android-sdk`
- `/opt/android-sdk`
- `$ANDROID_HOME`

If not found, you can install it:
```bash
# Install Android SDK tools
./install.sh sdk

# Or install everything
./install.sh install
```

## License

MIT
