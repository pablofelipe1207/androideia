package skills

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pablofelipe1207/androideia/internal/brain"
	"gopkg.in/yaml.v3"
)

// SkillInfo represents a skill for the CLI (backward compatible with cmd/skills.go).
type SkillInfo struct {
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description" yaml:"description"`
	Source      string   `json:"source" yaml:"source"`    // "builtin", "global", "project", "opencode", "android"
	Path        string   `json:"path" yaml:"path"`        // filesystem path
	Content     string   `json:"content" yaml:"content"`  // the actual skill text
	Triggers    []string `json:"triggers" yaml:"triggers"` // trigger keywords
}

// Skill is a reusable knowledge module (enhanced version for agent activation).
type Skill struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Content     string   `yaml:"content"`
	Always      bool     `yaml:"always"`
	Priority    int      `yaml:"priority"`
	MaxTokens   int      `yaml:"max_tokens"`
	Triggers    Triggers `yaml:"triggers"`
}

// Triggers define when a skill should be activated.
type Triggers struct {
	Files   []string `yaml:"files"`
	Content []string `yaml:"content"`
	Tools   []string `yaml:"tools"`
	Types   []string `yaml:"types"`
}

// SkillConfig is the YAML configuration for skills.
type SkillConfig struct {
	Skills []Skill `yaml:"skills"`
}

// SkillLoader manages skill discovery, loading, and import (backward compatible).
type SkillLoader struct {
	projectDir  string
	globalDir   string
	embeddedDir string
	skills      []SkillInfo
}

// NewSkillLoader creates a new skill loader for the given project.
func NewSkillLoader(projectDir string) *SkillLoader {
	homeDir, _ := os.UserHomeDir()
	globalDir := filepath.Join(homeDir, ".androideai", "skills")
	projectSkillsDir := filepath.Join(projectDir, ".androideai", "skills")

	return &SkillLoader{
		projectDir:  projectSkillsDir,
		globalDir:   globalDir,
		embeddedDir: "embedded",
	}
}

// LoadAll loads skills from all sources (project, global, embedded).
func (l *SkillLoader) LoadAll() error {
	l.skills = nil

	// Load embedded (built-in) skills
	for _, s := range builtinSkills() {
		l.skills = append(l.skills, SkillInfo{
			Name:        s.Name,
			Description: s.Description,
			Source:      "builtin",
			Path:        "(built-in)",
			Content:     s.Content,
			Triggers:    triggerToStrings(s.Triggers),
		})
	}

	// Load global skills
	l.loadFromDir(l.globalDir, "global")

	// Load project skills
	l.loadFromDir(l.projectDir, "project")

	return nil
}

// ListSkills returns all loaded skills.
func (l *SkillLoader) ListSkills() []SkillInfo {
	return l.skills
}

// GetSkill returns a skill by name.
func (l *SkillLoader) GetSkill(name string) (*SkillInfo, error) {
	for _, s := range l.skills {
		if strings.EqualFold(s.Name, name) {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("skill %q not found", name)
}

// GetSkillPaths returns the skill directories in order of precedence.
func (l *SkillLoader) GetSkillPaths() [3]string {
	return [3]string{l.projectDir, l.globalDir, l.embeddedDir}
}

// AddSkill copies a skill from a source path to the project skills directory.
func (l *SkillLoader) AddSkill(sourcePath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("source path not found: %w", err)
	}

	if err := os.MkdirAll(l.projectDir, 0755); err != nil {
		return fmt.Errorf("error creating skills directory: %w", err)
	}

	if info.IsDir() {
		// Copy entire directory
	 destDir := filepath.Join(l.projectDir, filepath.Base(sourcePath))
		return copyDir(sourcePath, destDir)
	}

	// Copy single file
	destPath := filepath.Join(l.projectDir, filepath.Base(sourcePath))
	return copyFile(sourcePath, destPath)
}

// FindSkillsByTrigger finds skills that match a trigger keyword.
func (l *SkillLoader) FindSkillsByTrigger(trigger string) []SkillInfo {
	var matches []SkillInfo
	triggerLower := strings.ToLower(trigger)
	for _, s := range l.skills {
		for _, t := range s.Triggers {
			if strings.EqualFold(t, triggerLower) {
				matches = append(matches, s)
				break
			}
		}
	}
	return matches
}

// ImportFromOpencode imports skills from opencode's skill directory.
func (l *SkillLoader) ImportFromOpencode() (int, error) {
	homeDir, _ := os.UserHomeDir()
	opencodeDir := filepath.Join(homeDir, ".config", "opencode", "skills")
	return l.importFromDir(opencodeDir, "opencode")
}

// ImportFromAndroidSkills imports official Android skills.
func (l *SkillLoader) ImportFromAndroidSkills() (int, error) {
	// For now, return 0 - the actual Android skills import would
	// download from github.com/android/skills
	return 0, fmt.Errorf("Android skills import not yet implemented")
}

// ImportAndroidSkillByName imports a specific Android skill.
func (l *SkillLoader) ImportAndroidSkillByName(name string) error {
	return fmt.Errorf("Android skill import not yet implemented")
}

func (l *SkillLoader) loadFromDir(dir, source string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			// Look for SKILL.md in subdirectory
			skillFile := filepath.Join(dir, e.Name(), "SKILL.md")
			if content, err := os.ReadFile(skillFile); err == nil {
				l.skills = append(l.skills, SkillInfo{
					Name:        e.Name(),
					Description: extractDescription(string(content)),
					Source:      source,
					Path:        skillFile,
					Content:     string(content),
				})
			}
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".yml" || ext == ".yaml" {
			l.loadYAMLFile(filepath.Join(dir, e.Name()), source)
		} else if e.Name() == "SKILL.md" {
			if content, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil {
				l.skills = append(l.skills, SkillInfo{
					Name:        strings.TrimSuffix(e.Name(), "SKILL.md"),
					Description: extractDescription(string(content)),
					Source:      source,
					Path:        filepath.Join(dir, e.Name()),
					Content:     string(content),
				})
			}
		}
	}
}

func (l *SkillLoader) loadYAMLFile(path, source string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var cfg SkillConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return
	}
	for _, s := range cfg.Skills {
		l.skills = append(l.skills, SkillInfo{
			Name:        s.Name,
			Description: s.Description,
			Source:      source,
			Path:        path,
			Content:     s.Content,
			Triggers:    triggerToStrings(s.Triggers),
		})
	}
}

func (l *SkillLoader) importFromDir(sourceDir, source string) (int, error) {
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return 0, nil
	}

	if err := os.MkdirAll(l.projectDir, 0755); err != nil {
		return 0, fmt.Errorf("error creating skills directory: %w", err)
	}

	count := 0
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, "SKILL.md") || strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml") {
			skillDir := filepath.Base(filepath.Dir(path))
			destDir := filepath.Join(l.projectDir, skillDir)
			if err := os.MkdirAll(destDir, 0755); err != nil {
				return err
			}
			destPath := filepath.Join(destDir, filepath.Base(path))
			if err := copyFile(path, destPath); err != nil {
				return err
			}
			count++
		}
		return nil
	})

	return count, err
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}
		return copyFile(path, dstPath)
	})
}

func extractDescription(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			if len(line) > 100 {
				return line[:100] + "..."
			}
			return line
		}
	}
	return ""
}

func triggerToStrings(t Triggers) []string {
	var result []string
	result = append(result, t.Files...)
	result = append(result, t.Content...)
	result = append(result, t.Tools...)
	result = append(result, t.Types...)
	return result
}

// --- Agent integration (new API) ---

// ActivationContext provides information to decide which skills to activate.
type ActivationContext struct {
	Task       string
	ToolsUsed  []string
	FileTypes  []string
	FilePaths  []string
	MaxTokens  int
	HasCompose bool
	HasHilt    bool
	HasRoom    bool
	HasNav     bool
}

// BuildContextFromTask creates an ActivationContext from a task string.
func BuildContextFromTask(task string) ActivationContext {
	ctx := ActivationContext{Task: task}
	lower := strings.ToLower(task)

	if strings.Contains(lower, "compose") || strings.Contains(lower, "screen") || strings.Contains(lower, "ui") {
		ctx.HasCompose = true
	}
	if strings.Contains(lower, "hilt") || strings.Contains(lower, "dagger") || strings.Contains(lower, "inject") || strings.Contains(lower, "di ") {
		ctx.HasHilt = true
	}
	if strings.Contains(lower, "room") || strings.Contains(lower, "database") || strings.Contains(lower, "dao") || strings.Contains(lower, "entity") {
		ctx.HasRoom = true
	}
	if strings.Contains(lower, "navigation") || strings.Contains(lower, "nav") || strings.Contains(lower, "route") || strings.Contains(lower, "deeplink") {
		ctx.HasNav = true
	}

	return ctx
}

// Registry manages skill activation for the agent.
type Registry struct {
	skills  []Skill
	builtin []Skill
	db      *sql.DB
}

// NewRegistry creates a new skill registry.
func NewRegistry(db *sql.DB) *Registry {
	return &Registry{
		db:      db,
		builtin: builtinSkills(),
	}
}

// LoadFromFile loads skills from a YAML file.
func (r *Registry) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading skills file: %w", err)
	}
	var cfg SkillConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing skills file: %w", err)
	}
	r.skills = append(r.skills, cfg.Skills...)
	return nil
}

// LoadFromDir loads all .yml/.yaml files from a directory.
func (r *Registry) LoadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading skills dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".yml" || ext == ".yaml" {
			if err := r.LoadFromFile(filepath.Join(dir, e.Name())); err != nil {
				return fmt.Errorf("loading %s: %w", e.Name(), err)
			}
		}
	}
	return nil
}

// Activate returns the skills that should be active given the context.
func (r *Registry) Activate(ctx ActivationContext) []Skill {
	var result []Skill
	usedTokens := 0

	for _, s := range r.allSkills() {
		if s.Always {
			result = append(result, s)
			usedTokens += s.MaxTokens
		}
	}

	for _, s := range r.allSkills() {
		if s.Always {
			continue
		}
		if r.matchesTriggers(s, ctx) {
			if ctx.MaxTokens > 0 && usedTokens+s.MaxTokens > ctx.MaxTokens {
				continue
			}
			result = append(result, s)
			usedTokens += s.MaxTokens
		}
	}

	return result
}

// RenderActiveSkills formats active skills for injection into the system prompt.
func RenderActiveSkills(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Active Skills\n\n")
	for _, s := range skills {
		sb.WriteString(fmt.Sprintf("### %s\n%s\n\n", s.Name, s.Content))
	}
	return sb.String()
}

func (r *Registry) allSkills() []Skill {
	all := make([]Skill, 0, len(r.builtin)+len(r.skills))
	all = append(all, r.builtin...)
	all = append(all, r.skills...)
	return all
}

func (r *Registry) matchesTriggers(s Skill, ctx ActivationContext) bool {
	t := s.Triggers

	if len(t.Files) > 0 && len(ctx.FilePaths) > 0 {
		for _, pattern := range t.Files {
			for _, path := range ctx.FilePaths {
				if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
					return true
				}
			}
		}
	}

	if len(t.Content) > 0 {
		lower := strings.ToLower(ctx.Task)
		for _, pattern := range t.Content {
			if strings.Contains(lower, strings.ToLower(pattern)) {
				return true
			}
		}
	}

	if len(t.Tools) > 0 && len(ctx.ToolsUsed) > 0 {
		toolSet := make(map[string]bool)
		for _, tu := range ctx.ToolsUsed {
			toolSet[tu] = true
		}
		for _, tool := range t.Tools {
			if toolSet[tool] {
				return true
			}
		}
	}

	if len(t.Types) > 0 && len(ctx.FileTypes) > 0 {
		typeSet := make(map[string]bool)
		for _, ft := range ctx.FileTypes {
			typeSet[ft] = true
		}
		for _, typ := range t.Types {
			if typeSet[typ] {
				return true
			}
		}
	}

	if ctx.HasCompose && hasTriggerWord(s, "compose") {
		return true
	}
	if ctx.HasHilt && hasTriggerWord(s, "hilt") {
		return true
	}
	if ctx.HasRoom && hasTriggerWord(s, "room") {
		return true
	}
	if ctx.HasNav && hasTriggerWord(s, "navigation") {
		return true
	}

	return false
}

func hasTriggerWord(s Skill, word string) bool {
	lower := strings.ToLower(s.Name + " " + s.Description)
	return strings.Contains(lower, word)
}

// SearchBrainForSkillContent searches the brain for content relevant to a skill topic.
func SearchBrainForSkillContent(db *sql.DB, topic string) string {
	if db == nil {
		return ""
	}
	b := brain.NewBrain(db)
	entries, err := b.Search(topic)
	if err != nil || len(entries) == 0 {
		return ""
	}
	var lines []string
	for _, e := range entries {
		if e.Status == "promoted" {
			lines = append(lines, fmt.Sprintf("- %s: %s", e.Title, truncateStr(e.Content, 150)))
		}
		if len(lines) >= 3 {
			break
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// parseSkillContent parses a SKILL.md file with YAML frontmatter.
func parseSkillContent(content string) (*SkillInfo, error) {
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("skill content must start with YAML frontmatter (---)")
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid frontmatter format")
	}

	frontmatter := strings.TrimSpace(parts[1])
	body := strings.TrimSpace(parts[2])

	var meta struct {
		Name        string   `yaml:"name"`
		Description string   `yaml:"description"`
		Triggers    []string `yaml:"triggers"`
	}

	if err := yaml.Unmarshal([]byte(frontmatter), &meta); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	return &SkillInfo{
		Name:        meta.Name,
		Description: meta.Description,
		Content:     body,
		Triggers:    meta.Triggers,
	}, nil
}

// ExportSkillsJSON exports all loaded skills as JSON.
func (l *SkillLoader) ExportSkillsJSON() (string, error) {
	if len(l.skills) == 0 {
		if err := l.LoadAll(); err != nil {
			return "", err
		}
	}
	data, err := yaml.Marshal(l.skills)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// builtinSkills returns the built-in skills for Android development.
func builtinSkills() []Skill {
	return []Skill{
		{
			Name:        "android_architecture",
			Description: "Core Android architecture patterns (MVVM, MVI, Clean Architecture)",
			Content: `## Android Architecture Patterns

### MVVM (Model-View-ViewModel)
- ViewModel holds UI state (UiState data class)
- UI observes StateFlow<UiState>
- Events go up via function calls, not LiveData
- No Android framework references in ViewModel (use cases for that)

### MVI (Model-View-Intent)
- Single source of truth: UiState
- User actions -> UiEvent -> Reducer -> UiState
- Side effects via UiEffect (Channel/SharedFlow, one-shot)
- Strict unidirectional data flow

### Clean Architecture Layers
- UI Layer: Activity/Fragment/Composable + ViewModel
- Domain Layer: UseCases (optional, for complex logic)
- Data Layer: Repository + DataSource (local/remote)

### Dependency Injection (Hilt)
- @HiltViewModel for ViewModels
- @Inject constructor for dependencies
- @Module + @InstallIn for bindings
- @AndroidEntryPoint for Activity/Fragment`,
			Always:    true,
			Priority:  100,
			MaxTokens: 200,
		},
		{
			Name:        "compose_ui",
			Description: "Jetpack Compose patterns, State, Effects, Material3",
			Content: `## Jetpack Compose Patterns

### State Management
- remember { mutableStateOf() } for local state
- rememberSaveable for state surviving config changes
- viewModel() / hiltViewModel() for shared state
- State hoisting: state goes down, events go up

### Effects
- LaunchedEffect(key) for side effects tied to composition
- DisposableEffect for cleanup
- rememberCoroutineScope for event-driven effects
- produceState for async data loading

### Recomposition
- Only re-read state that actually changed
- Use derivedStateOf for computed values
- Use Stable/Immutable annotations for stability
- LazyColumn/LazyGrid for lists

### Material3
- MaterialTheme for colors, typography, shapes
- Use theme tokens, not hardcoded colors
- Scaffold + TopAppBar + BottomBar pattern`,
			Always:    false,
			Priority:  90,
			MaxTokens: 150,
			Triggers: Triggers{
				Content: []string{"compose", "screen", "ui", "composable", "material"},
				Types:   []string{"composable", "activity"},
			},
		},
		{
			Name:        "hilt_di",
			Description: "Hilt/Dagger dependency injection patterns",
			Content: `## Hilt Dependency Injection

### ViewModels
@HiltViewModel
class MyViewModel @Inject constructor(
    private val useCase: MyUseCase
) : ViewModel()

### Use Cases
class MyUseCase @Inject constructor(
    private val repo: MyRepository
)

### Repositories
class MyRepositoryImpl @Inject constructor(
    private val dao: MyDao,
    private val api: MyApi
) : MyRepository

### Modules
@Module
@InstallIn(SingletonComponent::class)
object MyModule {
    @Provides
    @Singleton
    fun provideApi(): MyApi = Retrofit.Builder().build()
}

### Key Rules
- Never inject Context into ViewModel
- Use @ApplicationContext for Application-scoped deps
- Interface -> @Binds, concrete -> @Provides
- @Singleton = app scope, @ViewModelScoped = VM scope`,
			Always:    false,
			Priority:  85,
			MaxTokens: 150,
			Triggers: Triggers{
				Content: []string{"hilt", "dagger", "inject", "di ", "module"},
				Types:   []string{"di_module", "viewmodel", "usecase", "repository"},
			},
		},
		{
			Name:        "data_persistence",
			Description: "Room database, DAO, Entity, TypeConverter patterns",
			Content: `## Room Database Patterns

### Entity
@Entity(tableName = "items")
data class ItemEntity(
    @PrimaryKey val id: Long,
    val name: String,
    val createdAt: Long
)

### DAO
@Dao
interface ItemDao {
    @Query("SELECT * FROM items")
    fun getAll(): Flow<List<ItemEntity>>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(item: ItemEntity)

    @Delete
    suspend fun delete(item: ItemEntity)
}

### Database
@Database(entities = [ItemEntity::class], version = 1)
abstract class AppDatabase : RoomDatabase() {
    abstract fun itemDao(): ItemDao
}

### Repository Pattern
class ItemRepositoryImpl @Inject constructor(
    private val dao: ItemDao
) : ItemRepository {
    override fun getAll(): Flow<List<Item>> = dao.getAll().map { entities ->
        entities.map { it.toDomain() }
    }
}`,
			Always:    false,
			Priority:  70,
			MaxTokens: 120,
			Triggers: Triggers{
				Content: []string{"room", "database", "dao", "entity", "persistence"},
				Types:   []string{"dao", "entity", "repository"},
			},
		},
		{
			Name:        "navigation",
			Description: "Navigation Compose patterns, routes, NavHost",
			Content: `## Navigation Compose

### Route Definition
@Serializable
sealed class Screen(val route: String) {
    @Serializable data object Home : Screen("home")
    @Serializable data object Settings : Screen("settings")
    @Serializable data class Detail(val id: Long) : Screen("detail/{id}")
}

### NavHost Setup
NavHost(navController, startDestination = Screen.Home) {
    composable<Screen.Home> { HomeScreen(onNavigate = { navController.navigate(Screen.Settings) }) }
    composable<Screen.Settings> { SettingsScreen() }
    composable<Screen.Detail> { backStackEntry ->
        val detail = backStackEntry.toRoute<Screen.Detail>()
        DetailScreen(id = detail.id)
    }
}

### Navigation Arguments
- Use @Serializable data classes for type-safe args
- navController.navigate(Screen.Detail(id = 123))
- Deep links: navDeepLink<Screen.Detail>(basePath = "app://detail/{id}")`,
			Always:    false,
			Priority:  65,
			MaxTokens: 80,
			Triggers: Triggers{
				Content: []string{"navigation", "nav", "route", "deeplink", "navhost"},
				Types:   []string{"nav_route", "activity", "composable"},
			},
		},
		{
			Name:        "testing",
			Description: "JUnit, Espresso, Compose testing patterns",
			Content:    "## Testing Patterns\n\n### Unit Tests (JUnit + ViewModel)\n@Test\nfun whenActionTriggersStateChange() = runTest {\n    val useCase = mockk<MyUseCase>()\n    val viewModel = MyViewModel(useCase)\n    viewModel.onAction(SomeAction)\n    assertEquals(ExpectedState, viewModel.uiState.value)\n}\n\n### Compose UI Tests\n@Test\nfun myScreenTest() {\n    composeTestRule.setContent { MyScreen(viewModel = testViewModel) }\n    composeTestRule.onNodeWithText(\"Title\").assertIsDisplayed()\n    composeTestRule.onNodeWithTag(\"submit\").performClick()\n}\n\n### Key Patterns\n- Use Turbine for Flow testing\n- Use MockK for mocking\n- Use runTest for coroutine tests\n- Use composeTestRule for UI tests",
			Always:    false,
			Priority:  50,
			MaxTokens: 100,
			Triggers: Triggers{
				Content: []string{"test", "junit", "espresso", "mockk"},
				Types:   []string{"test"},
			},
		},
	}
}
