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
# Initialize a project
androideai init

# Index your code
androideai index build

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

```bash
# Check semantic search status
androideai semantic status

# Index symbols for semantic search
androideai semantic index

# Search by meaning
androideai semantic search "user authentication patterns"
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
