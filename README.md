# androideai-core v1.0.0

Offline-first AI agent for Android development with semantic code exploration, official Android skills, and automatic knowledge storage.

**No external dependencies required** — uses OpenCode Zen (Mimo V2.5 Free) for both agent chat and semantic code classification. Ollama is optional for local embeddings.

## Installation

### Quick Install (Linux/macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/pablofelipe1207/androideia/main/install.sh | bash
```

This installs androideai-core with Mimo V2.5 Free (no API key required, no Ollama needed).

### Install options

| Flag | Description |
|------|-------------|
| `--with-ollama` | Also install Ollama and pull `nomic-embed-text` + `qwen2.5:1.5b` for local embeddings and file classification |
| `install` | Default action: install the binary to `~/.local/bin` |
| `uninstall` | Remove the binary and config files |
| `from-source` | Build from source (requires Go 1.22+) |
| `sdk` | Install Android SDK components (platform-tools, build-tools) |

### With Ollama (optional, for local embeddings)

```bash
curl -fsSL https://raw.githubusercontent.com/pablofelipe1207/androideia/main/install.sh | bash -s -- --with-ollama
```

This also installs Ollama and pulls a lightweight model for local embeddings and file classification. The `models.yml` is configured to use Ollama for the semantic index:

```yaml
agent:
  provider: opencode_zen
  model: mimo-v2.5-free

semantic:
  provider: ollama
  base_url: http://localhost:11434
  chat_model: qwen2.5:1.5b
  embedding_model: nomic-embed-text
```

**When to use `--with-ollama`:**
- You want faster file classification (local LLM vs API calls)
- You need vector embeddings for semantic search (FTS is used without Ollama)
- You want everything to work fully offline
- You have a machine with enough RAM for the models (~2GB for both)

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
# Check the installed version
androideai version

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
- **Interview Mode**: Practice Android technical interviews with 40+ curated questions, scoring, and history tracking
- **Task Queue Manager**: Manage and process tasks by priority with LLM-powered execution
- **Feature Graph**: Map relationships between files, detect missing layers, and suggest what to create
- **Dynamic System Prompt**: Prompt is built dynamically based on task context, reducing token usage
- **Modular Skills**: Skills activate automatically based on triggers (Compose, Hilt, Room, Navigation)
- **Session Summaries**: Old sessions are compressed to summaries, saving tokens on resume
- **Semantic-Brain Sync**: Conventions from semantic index are automatically synced to brain after each session

## Version

androideai-core v1.0.0

Check the installed version:

```bash
androideai version
# Output:
# androideai-core v1.0.0
#   Go: go1.26.4 | OS: darwin | Arch: arm64
```

The version is embedded at build time via `-ldflags`. When building from source with `make build`, the version is automatically set from `Makefile` variables.

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

## Interview Mode

Practice Android technical interviews with AI-generated questions. The system uses a question bank of 40+ curated questions and can generate additional questions using the configured LLM.

```bash
# Start a general interview (10 questions)
androideai interview

# Filter by category
androideai interview --category compose
androideai interview --category architecture
androideai interview --category di
androideai interview --category async
androideai interview --category storage
androideai interview --category navigation
androideai interview --category testing

# Filter by difficulty
androideai interview --difficulty easy
androideai interview --difficulty medium
androideai interview --difficulty hard

# Combine filters
androideai interview --category compose --difficulty hard --count 5

# View history
androideai interview history

# List available categories and difficulties
androideai interview categories
androideai interview difficulties
```

### Interview Features

- **Immediate feedback**: After each answer, you get an explanation of the correct answer
- **Score tracking**: Final score with grade (A+, A, B, C, D, F) and per-category breakdown
- **Weak areas identification**: The system identifies your weak areas for focused study
- **LLM-generated questions**: Optionally generates additional questions using the configured LLM
- **History persistence**: All interviews are saved to the database for tracking progress

### Categories

| Category | Topics |
|----------|--------|
| `compose` | Jetpack Compose, State, Effects, Recomposition |
| `architecture` | MVVM, MVI, Clean Architecture, Repository pattern |
| `di` | Hilt, Dagger, Dependency Injection |
| `async` | Coroutines, Flow, StateFlow, SharedFlow, Dispatchers |
| `storage` | Room, DataStore, SharedPreferences |
| `navigation` | Navigation Compose, Routes, Arguments |
| `testing` | JUnit, Espresso, Mockk, Turbine |

## Task Queue

Manage a queue of tasks for the agent to process. Tasks are processed by priority (urgent > high > medium > low).

```bash
# Add tasks
androideai task add "Implement login screen" --priority high --type feature
androideai task add "Fix crash on home" -p urgent -t bugfix -d "NullPointerException in HomeViewModel"
androideai task add "Add unit tests" -t test -p medium

# List tasks
androideai task list                    # All tasks
androideai task list --status pending   # Filter by status
androideai task list --status queued    # Only queued tasks

# View task details
androideai task show 1

# Update tasks
androideai task update 1 --status queued
androideai task update 1 --priority urgent
androideai task update 1 --title "New title" --description "New description"

# Delete/Cancel tasks
androideai task delete 1
androideai task cancel 1

# Queue management
androideai task queue                   # View queued tasks
androideai task stats                   # Queue statistics
androideai task clear --type completed  # Clear completed tasks
androideai task clear --type all        # Clear completed + cancelled

# Process tasks
androideai task process                 # Process next task in queue
androideai task process --all           # Process all queued tasks
androideai task process --model qwen2.5-coder:7b  # Override model

# Process tasks from markdown file
androideai task run --file tasks.md           # Process tasks from file
androideai task run --file tasks.md --git     # With git workflow (branch + PR)
```

### Task Priorities

| Priority | Value | Processing Order |
|----------|-------|------------------|
| `urgent` | 3 | Processed first |
| `high` | 2 | Second |
| `medium` | 1 | Third |
| `low` | 0 | Processed last |

### Task Status

| Status | Description |
|--------|-------------|
| `pending` | Task created, not yet queued |
| `queued` | Ready to be processed |
| `processing` | Currently being processed by the agent |
| `completed` | Task finished successfully |
| `failed` | Task failed during processing |
| `cancelled` | Task was cancelled |

### Task Types

| Type | Description |
|------|-------------|
| `feature` | New feature implementation |
| `bugfix` | Bug fix |
| `refactor` | Code refactoring |
| `test` | Test creation |
| `review` | Code review |

### Processing with LLM

When you run `task process`, the system:

1. Gets the next task from the queue (highest priority first)
2. Marks it as `processing`
3. Uses the configured LLM to execute the task
4. Creates a conversation in memory for tracking
5. Stores the result and marks as `completed` or `failed`

### Process Tasks from Markdown

You can process tasks directly from a markdown file with checkboxes. The agent executes each task sequentially, marks them as completed in the file, and optionally creates git branches and PRs.

#### Markdown File Format

```markdown
# My Development Tasks

## Phase 1 - Backend
- [ ] Implement User data model
- [ ] Create User repository
- [ ] Implement GetUser use case
- [ ] Add unit tests for UserRepository

## Phase 2 - Frontend  
- [ ] Create user profile screen
- [ ] Implement profile ViewModel
- [ ] Add navigation from Home to Profile
```

#### Basic Usage

```bash
# Process all pending tasks from file
androideai task run --file tasks.md

# With specific model
androideai task run --file tasks.md --model qwen3-coder-64k-32k:latest

# Stop on first compilation error
androideai task run --file tasks.md --stop-on-error
```

#### Git Workflow

Enable automatic git workflow with the `--git` flag. For each task, the system will:

1. Create a new branch (`task/<task-name>`)
2. Execute the task with the agent
3. Validate compilation (`go build` + `go vet`)
4. Commit changes with message `feat: <task-name>`
5. Push branch and create a Pull Request
6. Return to original branch for the next task

```bash
# Enable git workflow
androideai task run --file tasks.md --git

# Custom branch prefix
androideai task run --file tasks.md --git --branch-prefix feature/
```

#### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--file`, `-f` | Markdown file with tasks (required) | - |
| `--git` | Enable git workflow (branch + PR per task) | `false` |
| `--branch-prefix` | Prefix for git branches | `task/` |
| `--validate-build` | Run `go build` after each task | `true` |
| `--stop-on-error` | Stop if compilation fails | `false` |
| `--model`, `-m` | Override LLM model | Config default |
| `--timeout` | Timeout per LLM call in seconds | `120` |
| `--max-turns` | Max agent turns per task | `25` |

#### How It Works

1. **Parse**: Reads the markdown file and extracts all `- [ ]` tasks
2. **Filter**: Skips already completed tasks (`- [x]`)
3. **Execute**: For each pending task:
   - Creates a git branch (if `--git` enabled)
   - Runs the agent with auto-approval
   - Validates compilation (if `--validate-build` enabled)
   - Marks task as `[x]` in the file
   - Creates PR and commits (if `--git` enabled)
4. **Report**: Shows summary of completed/failed tasks

#### Example Output

```
============================================================
  PROCESSING TASK FILE
  File: tasks.md
  Total: 9 | Pending: 9 | Completed: 0
============================================================

------------------------------------------------------------
  TASK 1/9: Implement User data model
------------------------------------------------------------
  [Git] Branch created: task/implement-user-data-model
  [Build] Verifying compilation...
  [Build] OK
  [Git] Commit created
  [Git] Push realizado
  [Git] PR creado: https://github.com/.../pull/123
  [File] Task marked as completed

============================================================
  SUMMARY
============================================================
  ✓ Implement User data model
    PR: https://github.com/.../pull/123
  ✓ Create User repository
    PR: https://github.com/.../pull/124
  ...

  Total: 9 | Successful: 7 | Failed: 2
============================================================
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

## Code Search

There are several commands with `search` in the name. They look
similar but do different things; pick the one that matches the
question you're actually asking.

| Question you're asking | Command | How it works |
|------------------------|---------|--------------|
| "Does a symbol named X exist?" | `androideai search <keyword>` | FTS5 over `symbols` (full-text, supports `OR`, `*`, `"phrase"`) |
| "Show me symbols whose name or kind contains X" | `androideai symbol <name\|kind>` | `LIKE %query%` on `symbols.name` / `symbols.kind` |
| "Where is the code that does X even if it isn't named that?" | `androideai semantic search <query>` | Embeddings + cosine similarity over symbol vectors |
| "In which file is the ViewModel / Composable / Repository for X?" | `androideai semantic locate <query>` | Filter on `file_semantics` (LLM classification by role/tag) |
| "What does the project know / what conventions exist?" | `androideai brain search <query>` | FTS5 over `knowledge_entries` |

### `androideai search` (FTS5 over symbols)

`androideai index build` parses every `.kt` / `.kts` with tree-sitter
and stores each function, class, property, etc. in the `symbols`
table, mirrored into a SQLite FTS5 index. `search` queries that index
and prints the matches with a highlighted snippet.

```bash
$ androideai search login
Searching for: login
src/main/java/.../LoginViewModel.kt:7 <b>Login</b>ViewModel : ViewModel()...
src/main/java/.../LoginUseCase.kt:5   class <b>Login</b>UseCase(@Inject...
src/main/java/.../LoginScreen.kt:8    fun <b>Login</b>Screen(viewModel: <b>Login</b>ViewModel) {

# Boolean operators and phrase queries are supported (FTS5 syntax):
androideai search "login OR logout"
androideai search "login event"      # exact phrase
androideai search "login*"           # prefix match
```

This is a **lexical** search: it matches the words you type. It will
not find a function that does what you want but is named differently.
For that, use `semantic search`.

### Quick rule of thumb

- **Exact name or known word** → `androideai search` / `symbol`
- **Describe what the code does** → `androideai semantic search`
- **Find a file by its role (ViewModel, Repository, ...)** → `androideai semantic locate`
- **Find project knowledge / conventions** → `androideai brain search`

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

1. **LLM file classification** — for every `.kt`/`.kts` file the LLM
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
2. **Symbol embeddings** (optional, requires Ollama) — every Kotlin symbol
   gets a vector so you can search code by meaning.

The agent queries this index with the `semantic_locate` tool *before*
creating any new file, so it never reinvents a pattern that already
exists, and never collides with a class/module that the project
already has.

### Default Setup (No Ollama Required)

By default, androideai-core uses **Mimo V2.5 Free** via OpenCode Zen for
file classification. No local LLM or API key is needed:

```yaml
# ~/.androideai/models.yml
agent:
  provider: opencode_zen
  model: mimo-v2.5-free

semantic:
  provider: opencode_zen
  chat_model: mimo-v2.5-free
  embedding_model: ""  # No embeddings, uses FTS for search
```

### With Ollama (Optional)

If you installed with `--with-ollama`, the semantic flow uses local
Ollama for faster classification and embeddings:

```yaml
# ~/.androideai/models.yml
agent:
  provider: opencode_zen
  model: mimo-v2.5-free

semantic:
  provider: ollama
  base_url: http://localhost:11434
  chat_model: qwen2.5:1.5b
  embedding_model: nomic-embed-text
```

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

## Feature Graph

The feature graph builds on the semantic index to map **relationships
between files**: which files belong to the same feature, which depend
on which architecturally, and what's missing. The agent uses this to
navigate the codebase and decide what to create or modify.

### How it works

1. **Feature grouping**: files are grouped by feature name (derived
   from tags, file path, and file name — e.g., `LoginViewModel` +
   `LoginScreen` + `LoginUseCase` → feature "login").
2. **Architectural dependencies**: inferred from file types —
   `Activity` → `ViewModel` → `UseCase` → `Repository` → `DAO`.
3. **Package co-location**: files in the same small package are
   linked as related.

### Agent tools

| Tool | What it does |
|------|-------------|
| `feature_graph` | Shows all features and their files, or a single feature's full subgraph |
| `feature_deps` | Shows what a file depends on, what depends on it, and the impact chain |
| `feature_suggest` | Suggests missing layers and files to create/modify for a feature |

Example agent flow:

```
Agent: feature_graph feature="login"
→ shows LoginViewModel, LoginScreen, LoginUseCase, LoginRepository

Agent: feature_deps path="app/src/.../LoginViewModel.kt"
→ shows: depends on LoginUseCase, LoginUseCase depends on LoginRepository
→ shows: LoginScreen depends on this, impact = 3 files

Agent: feature_suggest feature="login"
→ suggests: missing DAO layer, missing DI module, missing tests
```

### CLI (also available from the command line)

```bash
# See all features
androideai semantic graph

# See a specific feature
androideai semantic graph --feature login

# Check dependencies for a file
androideai semantic deps --path app/src/.../LoginViewModel.kt

# Get suggestions for a feature
androideai semantic suggest --feature login
```

## Token Optimization

androideai-core optimizes token usage in several ways:

### Dynamic System Prompt
The system prompt is built dynamically based on the task:
- **Base prompt**: Identity + workflow + package rules (~50 lines)
- **File creation module**: Added when task involves creating/modifying files
- **Semantic module**: Added when task involves code exploration
- **Scaffold module**: Added when task involves Compose, Hilt, Room, or Navigation
- **Skills**: Only active skills are included

This reduces token usage by ~30-50% compared to a static prompt.

### Session Summaries
When a session completes, a 3-5 sentence summary is generated. When resuming
old sessions, only the summary is loaded instead of all messages. This reduces
token usage by ~60-80% for sessions with long history.

### Selective Brain Knowledge
Knowledge entries are searched by relevance to the task type, not just
keyword matching. Only 2-3 most relevant entries are injected, truncated
to 150 chars each.

## Modular Skills

Skills are reusable knowledge modules that activate based on context:

### Built-in Skills

| Skill | Activates When | Tokens |
|-------|---------------|--------|
| `android_architecture` | Always | ~200 |
| `compose_ui` | Task mentions compose, screen, UI | ~150 |
| `hilt_di` | Task mentions Hilt, DI, inject | ~150 |
| `data_persistence` | Task mentions Room, database, DAO | ~120 |
| `navigation` | Task mentions navigation, routes | ~80 |
| `testing` | Task mentions tests, JUnit | ~100 |

### Custom Skills

Create skills in `.androideai/skills/` as YAML files:

```yaml
# .androideai/skills/my-skill.yml
skills:
  - name: my-custom-skill
    description: My project-specific patterns
    content: |
      ## My Patterns
      - Use StateFlow for all state
      - Always use @HiltViewModel
      - Follow MVVM architecture
    triggers:
      content: ["my-pattern", "custom"]
      types: ["viewmodel"]
```

### Skill Management

```bash
# List all skills (built-in + project)
androideai skills list

# Show a specific skill
androideai skills show compose_ui

# Add a skill from a file
androideai skills add path/to/skill.yml

# Search skills by trigger
androideai skills search compose
```

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

Three files are involved; **the model configuration lives in
`models.yml`** (see next section). The legacy `config.yml` is kept
for `approval`, `timeout`, and as a fallback for older callers.

| Scope | Path | Created by | What it holds |
|-------|------|------------|---------------|
| Global models | `~/.androideai/models.yml` | `install.sh` | agent + semantic index provider/model |
| Project models | `./.androideai/models.yml` | `androideai init` | overrides global |
| Global config (legacy) | `~/.androideai/config.yml` | `install.sh` | approval, timeout, fallback model fields |
| Project config (legacy) | `./.androideai/config.yml` | `androideai init` | overrides global |

### `models.yml` — the one place to configure models

This is the file you should edit when you install androideai-core in
a new environment. It has two top-level sections: `agent` (the LLM
that the agent loop talks to) and `semantic` (the LLM used by
`androideai semantic index` and `androideai init`).

```yaml
# ~/.androideai/models.yml
agent:
  # Provider: ollama | anthropic | openai | opencode_zen
  provider: opencode_zen

  # Model (provider-specific)
  model: mimo-v2.5-free

  # Optional: override the base URL (default depends on provider)
  # base_url: https://opencode.ai/zen/v1

  # Optional: name of the env var that holds the API key.
  # Leave empty for the free tier of OpenCode Zen.
  # api_key_env: OPENCODE_ZEN_API_KEY

semantic:
  # Provider: ollama | opencode_zen | openai
  # Default: opencode_zen (uses Mimo, no Ollama required)
  provider: opencode_zen

  # Model used for file classification
  # Default: mimo-v2.5-free (via OpenCode Zen)
  chat_model: mimo-v2.5-free

  # Optional: Ollama base URL (only needed if provider is "ollama")
  # base_url: http://localhost:11434

  # Optional: Model for embeddings (only needed for vector search)
  # If empty, uses FTS (full-text search) instead of embeddings
  # embedding_model: nomic-embed-text
```

**Default setup**: Both agent and semantic use Mimo V2.5 Free via
OpenCode Zen. No API key required, no Ollama needed. The semantic
index uses FTS (full-text search) for code lookup, which is fast
and works offline.

**With Ollama**: If you want local embeddings and faster classification,
install with `--with-ollama` and set the semantic provider to `ollama`.

### Managing models from the CLI

```bash
# Show the effective configuration (project overrides global)
androideai models show

# List available models for the current provider
androideai models list

# Set a value (writes to the project models.yml AND syncs the
# legacy config.yml so the semantic flow sees the change)
androideai models set agent.provider opencode_zen
androideai models set agent.model minimax-m3-free
androideai models set semantic.chat_model qwen2.5-coder:7b
androideai models set semantic.embedding_model nomic-embed-text
androideai models set agent.api_key_env OPENCODE_ZEN_API_KEY

# Create a fresh models.yml with defaults (or migrate from the
# legacy config.yml if one exists)
androideai models init              # project file
androideai models init --global     # global file

# Show where models.yml is loaded from
androideai models path
```

### Legacy `config.yml` (still supported)

The legacy flat config keeps working. If you don't want to use
`models.yml`, you can keep using the old `androideai config set`
commands:

```yaml
# ~/.androideai/config.yml
model: mimo-v2.5-free
provider: opencode_zen
approval: ask      # ask | auto | never
timeout: 300       # seconds per LLM call; raise this for slow models / long contexts
```

### Providers

| Provider | What it is | Setup |
|----------|------------|-------|
| `opencode_zen` (default) | [OpenCode Zen](https://opencode.ai/zen), a hosted gateway with a free tier. | No API key required for free tier. Optionally set `OPENCODE_ZEN_API_KEY`. |
| `ollama` | Local LLM via Ollama. Free, private, works offline. | Install [Ollama](https://ollama.com) with `--with-ollama`, pull a model. |
| `anthropic` | Anthropic Claude API. | Set `ANTHROPIC_API_KEY` env var. |
| `openai` | OpenAI Chat Completions API. | Set `OPENAI_API_KEY` env var. |

#### OpenCode Zen (default, free tier, no API key required)

[OpenCode Zen](https://opencode.ai/zen) is a curated model gateway from the
OpenCode team. The free tier includes models like `mimo-v2.5-free`,
`minimax-m3-free`, `glm-4.7-free`, `kimi-k2.5-free`, `gpt-5-nano`,
`deepseek-v4-flash-free` etc. with no API key required.
See the [full model list](https://opencode.ai/zen/v1/models).

The API is OpenAI-compatible, so androideai-core talks to it with
the standard `POST /v1/chat/completions` shape.

**Default setup (no Ollama needed):** Both agent and semantic use
Mimo V2.5 Free via OpenCode Zen. No local LLM required.

```yaml
# ~/.androideai/models.yml
agent:
  provider: opencode_zen
  model: mimo-v2.5-free

semantic:
  provider: opencode_zen
  chat_model: mimo-v2.5-free
  embedding_model: ""  # Uses FTS for search
```

Optional environment variables:

- `OPENCODE_ZEN_API_KEY` — if you've logged in with `opencode auth login`,
  set this to your key. The free tier does not require it.
- `OPENCODE_ZEN_BASE_URL` — override the endpoint (defaults to
  `https://opencode.ai/zen/v1`). Useful for self-hosted gateways.

#### With Ollama (optional, for local embeddings)

If you installed with `--with-ollama`, you can use Ollama for faster
classification and local embeddings:

```yaml
# ~/.androideai/models.yml
agent:
  provider: opencode_zen
  model: mimo-v2.5-free

semantic:
  provider: ollama
  base_url: http://localhost:11434
  chat_model: qwen2.5:1.5b
  embedding_model: nomic-embed-text
```

This hybrid setup uses OpenCode Zen for the agent's chat and Ollama
for semantic indexing (embeddings + file classification).

### Managing config with the CLI

```bash
# Show the effective config (project + global merged)
androideai config show

# Read a single key
androideai config get model

# Set a value in the project config
androideai config set model qwen2.5-coder:7b
androideai config set ollama_url http://remote:11434
androideai config set ollama_model qwen2.5-coder:7b  # for the hybrid Zen+Ollama setup
androideai config set provider opencode_zen
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
- Android SDK (for emulator operations) - automatically detected or installed
- No runtime dependencies (single binary)
- Ollama (optional, only needed if you want local embeddings)

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
