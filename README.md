# androideai-core

Offline-first AI agent for Android development with semantic code exploration, official Android skills, and automatic knowledge storage.

## Installation

### Quick Install (Linux/macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/mobiai/androideai-core/main/install.sh | bash
```

### Using Make

```bash
git clone https://github.com/mobiai/androideai-core.git
cd androideai-core
make install
```

### Using Go

```bash
go install github.com/mobiai/androideai-core@latest
```

### Manual Download

Download the latest binary from [Releases](https://github.com/mobiai/androideai-core/releases) and add it to your PATH.

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
```

## Features

### Core Features
- **Code Indexing**: Fast symbol search with tree-sitter parser
- **Feature Navigation**: Find all layers of a feature (Screen, ViewModel, UseCase, etc.)
- **Knowledge Base**: Save and search project decisions and patterns
- **Agent Loop**: AI-guided development with approval gate
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
```

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

Configuration file: `.androideai/config.yml`

```yaml
model: qwen3-coder-64k-32k:latest
ollama_url: http://localhost:11434
provider: ollama
approval: ask  # or "auto" for automatic approval
```

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
