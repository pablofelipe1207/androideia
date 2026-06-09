package agent

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pablofelipe1207/androideia/internal/project"
	"github.com/pablofelipe1207/androideia/internal/skills"
)

// SystemPromptBase is the minimal system prompt (always included).
const SystemPromptBase = `You are androideai-core, an expert Android development assistant. You help developers build Android applications by exploring code, planning changes, and implementing features.

## Development Workflow
Follow this cycle for every task:

1. **EXPLORAR (Explore)**: Use index_search and index_feature to understand the codebase structure. Find relevant files, patterns, and existing implementations.
   - **ANTES de crear una feature nueva**: Ejecuta 'feature list' o usa 'semantic_locate type=screen' / 'semantic_locate type=viewmodel' para ver qué features YA EXISTEN.
   - Si la feature pedida ya existe (ej. "login"), NO la crees de nuevo. En su lugar, extiende o modifica la existente.
   - Si no existe, usa las features existentes como referencia de estructura y diseño.

2. **PLANEAR (Plan)**: Propose a detailed plan with specific files to create/modify. Explain your reasoning. DO NOT write any files yet.

3. **CONFIRMAR (Confirm)**: Wait for user approval before making any changes. Ask clarifying questions if needed.

4. **IMPLEMENTAR (Implement)**: Execute the plan step by step. Use write_file to create/modify files. Follow existing code conventions.

5. **COMPILAR Y VERIFICAR (Compile & Verify)**: This step is MANDATORY before finishing. After implementing:
   a. Run the Gradle compile task relevant to the project.
   b. **If compilation fails**, read the error output carefully, fix the files, and recompile.
   c. Only after clean compilation, run relevant tests with the 'test' tool.
   d. If tests fail, fix and recompile until all pass.
   e. **Do NOT call confirm_plan or declare the task done until compilation is green.**

6. **APRENDER (Learn)**: If you discover important patterns or decisions, suggest saving them to the knowledge base using brain_search.

## CRITICAL — Package and naming conventions
NEVER invent a package name for new files. The package directory of every new
Kotlin/Java file MUST be derived from the project's real applicationId
declared in AndroidManifest.xml or app/build.gradle.kts.

Before writing ANY new file you MUST:
1. Read AndroidManifest.xml or build.gradle.kts to obtain the real applicationId/namespace.
2. Inspect activities in the manifest. Use the SAME package prefix they use.
3. Inspect gradle/libs.versions.toml. REUSE existing version aliases and library declarations.
4. Never write a package line that does not match the directory path.

## Rules
- NEVER write files without user approval (mode: ask by default)
- Always explore before making changes
- Follow existing code conventions in the project
- Write clean, readable, and well-structured code
- Handle errors gracefully
- Explain your reasoning and decisions

## Tool Usage
You have access to tools. To use one, you MUST invoke it through the tool
calling mechanism (the 'tool_calls' field of your response), NOT by writing
JSON in plain text.

## Confirmation Rule
NEVER ask the user to confirm something in plain text. You MUST call the
confirm_plan tool whenever you want approval. Use ask_user for free-text questions.

Remember: Your goal is to help developers build better Android applications.`

// Module: always included for file creation tasks
const promptModuleFileCreation = `## File Creation Rules
- NEVER overwrite an existing file unless the user explicitly asks you to replace it.
- When adding a new feature, NEVER modify or delete code in unrelated files.
- If an existing file already has content, PRESERVE it. Add new code without deleting working code.
- When in doubt: CREATE a new file instead of modifying an existing one.

## Design consistency
- **Existing features**: IMMUTABLE unless user explicitly requests redesign.
- **New features**: MUST follow the design system of existing screens. Use semantic_locate to inspect existing patterns.`

// Module: semantic workflow
const promptModuleSemantic = `### Semantic-locate workflow
Before creating a new .kt/.kts file:
1. Call semantic_locate with the relevant type to see what already exists.
2. If the user mentioned a specific name, also call semantic_locate with that name.
3. Use the returned "conventions" + "architecture" fields to mirror existing style.
4. If no results, tell the user the semantic index needs rebuilding.`

// Module: scaffold workflow
const promptModuleScaffold = `### Mandatory scaffold + validate workflow
Every time you create or modify an Android component file:
1. **CHECK** — Call android_scaffold with action="check"
2. **TEMPLATE** — Call android_scaffold with action="template"
3. **WRITE** — Replace every TODO block with concrete implementation
4. **VALIDATE** — Call validate_kotlin with the file path and role
5. **CONFIRM** — Only when validate_kotlin reports OK, call confirm_plan`

// Module: tool list (condensed)
const promptModuleTools = `### Available Tools
- read_file, write_file: File I/O
- index_search, index_feature: Code index
- brain_search: Knowledge base
- semantic_search, semantic_locate: Semantic code exploration
- android_scaffold: Templates + existing file check
- validate_kotlin: Contract validation
- feature_graph, feature_deps, feature_suggest: Feature relationship graph
- gradle, test, emulator: Build & run
- confirm_plan, ask_user: User interaction`

// BuildDynamicSystemPrompt constructs the system prompt by loading only
// the modules relevant to the current task context.
func BuildDynamicSystemPrompt(ctx skills.ActivationContext, activeSkills []skills.Skill) string {
	var sb strings.Builder
	sb.WriteString(SystemPromptBase)

	// Add file creation module if task involves creating/modifying files
	if ctx.Task != "" {
		lower := strings.ToLower(ctx.Task)
		needsFileModule := strings.Contains(lower, "create") ||
			strings.Contains(lower, "add") ||
			strings.Contains(lower, "implement") ||
			strings.Contains(lower, "write") ||
			strings.Contains(lower, "build") ||
			strings.Contains(lower, "feature") ||
			strings.Contains(lower, "screen") ||
			strings.Contains(lower, "fix") ||
			strings.Contains(lower, "refactor")
		if needsFileModule {
			sb.WriteString("\n\n")
			sb.WriteString(promptModuleFileCreation)
		}
	}

	// Add semantic module if task involves code exploration
	if ctx.Task != "" {
		lower := strings.ToLower(ctx.Task)
		needsSemantic := strings.Contains(lower, "find") ||
			strings.Contains(lower, "search") ||
			strings.Contains(lower, "where") ||
			strings.Contains(lower, "locate") ||
			strings.Contains(lower, "exist")
		if needsSemantic {
			sb.WriteString("\n\n")
			sb.WriteString(promptModuleSemantic)
		}
	}

	// Add scaffold module if task involves creating Android components
	if ctx.HasCompose || ctx.HasHilt || ctx.HasRoom || ctx.HasNav {
		sb.WriteString("\n\n")
		sb.WriteString(promptModuleScaffold)
	}

	// Always add condensed tools list
	sb.WriteString("\n\n")
	sb.WriteString(promptModuleTools)

	// Append active skill content
	if len(activeSkills) > 0 {
		sb.WriteString("\n\n")
		sb.WriteString(skills.RenderActiveSkills(activeSkills))
	}

	return sb.String()
}

// BuildLegacySystemPrompt returns the full original system prompt for
// backward compatibility (used by resume of old sessions).
func BuildLegacySystemPrompt() string {
	return SystemPromptBase + "\n\n" + promptModuleFileCreation + "\n\n" + promptModuleSemantic + "\n\n" + promptModuleScaffold + "\n\n" + promptModuleTools
}

// SystemPrompt is kept for backward compatibility.
const SystemPrompt = SystemPromptBase + "\n\n" + promptModuleFileCreation + "\n\n" + promptModuleSemantic + "\n\n" + promptModuleScaffold + "\n\n" + promptModuleTools

func BuildUserPrompt(task string) string {
	return task
}

func BuildContextPrompt(context map[string]interface{}) string {
	var prompt string

	if features, ok := context["features"].([]string); ok && len(features) > 0 {
		prompt += "Relevant features found:\n"
		for _, f := range features {
			prompt += "- " + f + "\n"
		}
		prompt += "\n"
	}

	if files, ok := context["files"].([]string); ok && len(files) > 0 {
		prompt += "Relevant files:\n"
		for _, f := range files {
			prompt += "- " + f + "\n"
		}
		prompt += "\n"
	}

	if knowledge, ok := context["knowledge"].([]string); ok && len(knowledge) > 0 {
		prompt += "Project knowledge:\n"
		for _, k := range knowledge {
			prompt += "- " + k + "\n"
		}
		prompt += "\n"
	}

	return prompt
}

// BuildProjectContextBlock genera el bloque "## Project context" que el
// CLI inyecta al principio de la conversación cuando logra descubrir el
// AndroidManifest y/o libs.versions.toml.
func BuildProjectContextBlock(md *project.Metadata) string {
	if md == nil {
		return ""
	}
	var b strings.Builder
	hasAny := false
	if md.ApplicationID != "" {
		hasAny = true
		fmt.Fprintf(&b, "## Project context\n\n")
		fmt.Fprintf(&b, "- Application ID / namespace: `%s`\n", md.ApplicationID)
	}
	if md.ManifestPath != "" {
		if !hasAny {
			fmt.Fprintf(&b, "## Project context\n\n")
		}
		hasAny = true
		fmt.Fprintf(&b, "- AndroidManifest: `%s`\n", md.ManifestPath)
	}
	if len(md.ManifestActivities) > 0 {
		if !hasAny {
			fmt.Fprintf(&b, "## Project context\n\n")
			hasAny = true
		}
		fmt.Fprintf(&b, "- Manifest activities:\n")
		for _, a := range md.ManifestActivities {
			fmt.Fprintf(&b, "  - `%s`\n", a)
		}
	}
	if md.LibsVersionsPath != "" {
		if !hasAny {
			fmt.Fprintf(&b, "## Project context\n\n")
			hasAny = true
		}
		fmt.Fprintf(&b, "- gradle/libs.versions.toml: `%s`\n", md.LibsVersionsPath)
	}
	if len(md.LibsVersions) > 0 {
		fmt.Fprintf(&b, "- Library versions declared (reuse these aliases before adding a new one):\n")
		keys := make([]string, 0, len(md.LibsVersions))
		for k := range md.LibsVersions {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "  - `%s` = \"%s\"\n", k, md.LibsVersions[k])
		}
	}
	if len(md.LibsLibraries) > 0 {
		fmt.Fprintf(&b, "- Libraries already declared (do NOT redeclare these with a new alias):\n")
		keys := make([]string, 0, len(md.LibsLibraries))
		for k := range md.LibsLibraries {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "  - `%s` = `%s`\n", k, md.LibsLibraries[k])
		}
	}
	if !hasAny {
		return ""
	}
	return b.String()
}
