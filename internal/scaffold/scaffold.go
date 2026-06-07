// Package scaffold provides canonical Kotlin templates and a static
// validator for the most common Android components (ViewModel, Composable
// screen, Activity, UseCase, Repository, DAO, Hilt module, data class,
// Room entity, navigation route). It is consumed by:
//
//   - the agent tools (android_scaffold / validate_kotlin) to bootstrap
//     and self-check files it is about to write;
//   - the CLI (androideai scaffold / androideai validate) for ad-hoc
//     usage;
//   - the `android-scaffold` skill, which describes the same workflow
//     in natural language.
//
// Templates are kept conservative: they are valid, idiomatic MVVM with
// Jetpack Compose + Hilt + Coroutines/Flow, and they document the
// required shape with comments. The agent is expected to fill in the
// `// TODO: <feature>` blocks; the validator then enforces that the
// non-TODO parts of the contract hold (annotations, nested types,
// public state holder, error handling, ...).
package scaffold

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Role is the canonical component kind. The set mirrors
// `semantic.AllowedTypes` plus a few extras (composable, activity,
// nav_route) that are useful for scaffolding even though some of them
// overlap with composable/activity in the semantic classifier.
type Role string

const (
	RoleViewModel  Role = "viewmodel"
	RoleComposable Role = "composable"
	RoleActivity   Role = "activity"
	RoleUseCase    Role = "usecase"
	RoleRepository Role = "repository"
	RoleDao        Role = "dao"
	RoleDiModule   Role = "di_module"
	RoleDataClass  Role = "data_class"
	RoleEntity     Role = "entity"
	RoleNavRoute   Role = "nav_route"
)

// AllRoles returns the ordered list of every supported role. Used by
// the CLI to render help and by the agent to validate the role string
// the LLM hands us.
func AllRoles() []Role {
	return []Role{
		RoleViewModel, RoleComposable, RoleActivity, RoleUseCase,
		RoleRepository, RoleDao, RoleDiModule, RoleDataClass,
		RoleEntity, RoleNavRoute,
	}
}

// IsValidRole returns true if r is one of the supported roles.
func IsValidRole(r Role) bool {
	for _, k := range AllRoles() {
		if k == r {
			return true
		}
	}
	return false
}

// Issue is a single validation finding. Severity is "error" (must fix)
// or "warning" (style). The validator returns at most a few of each so
// the agent has a clear, actionable list.
type Issue struct {
	Severity string `json:"severity"` // "error" | "warning"
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	Line     int    `json:"line,omitempty"`
}

// Rule is a single static check. We use a tiny DSL to keep the spec
// readable: a regex, an optional `MustMatch` flag, and a human
// description. Multi-line aware matching is handled by the validator
// (it scans with `(?m)`).
type Rule struct {
	ID          string
	Description string
	Pattern     *regexp.Regexp
	MustMatch   bool   // true = required, false = forbidden
	Example     string // optional hint shown in the template
	Help        string // optional remediation message
}

// Spec describes one role: template + validation rules + metadata.
type Spec struct {
	Role         Role
	DisplayName  string
	Description  string
	FileNameHint string // e.g. "LoginViewModel.kt"
	Template     string
	Rules        []Rule
}

// specs is the registry of every supported role. Templates are kept
// inline so they ship with the binary and the agent can request them
// without touching the filesystem.
var specs = map[Role]Spec{}

// registerSpec is a tiny helper used by the init() blocks below to
// keep each spec readable.
func registerSpec(s Spec) { specs[s.Role] = s }

// ---------------------------------------------------------------------------
// VIEWMODEL
// ---------------------------------------------------------------------------

func init() {
	registerSpec(Spec{
		Role:         RoleViewModel,
		DisplayName:  "ViewModel (MVVM con StateFlow + Hilt)",
		Description:  "ViewModel con UiState/UiEvent/UiEffect sellados, Hilt @Inject, expone StateFlow y consume un UseCase.",
		FileNameHint: "{Name}ViewModel.kt",
		Template: viewModelTemplate,
		Rules: []Rule{
			{
				ID:          "vm.hilt",
				Description: "Debe tener @HiltViewModel",
				Pattern:     regexp.MustCompile(`@HiltViewModel`),
				MustMatch:   true,
				Help:        "Add @HiltViewModel above the class declaration.",
			},
			{
				ID:          "vm.viewmodel",
				Description: "Debe extender androidx.lifecycle.ViewModel",
				Pattern:     regexp.MustCompile(`:\s*ViewModel\s*\(\s*\)`),
				MustMatch:   true,
				Help:        "Make the class extend ViewModel().",
			},
			{
				ID:          "vm.inject",
				Description: "Debe tener constructor con @Inject",
				Pattern:     regexp.MustCompile(`@Inject\s+constructor`),
				MustMatch:   true,
				Help:        "Add @Inject to the primary constructor.",
			},
			{
				ID:          "vm.uistate",
				Description: "Debe declarar un UiState (data class o sealed interface)",
				Pattern:     regexp.MustCompile(`(data\s+class|sealed\s+interface)\s+UiState`),
				MustMatch:   true,
				Help:        "Declare a nested UiState: data class UiState(...) or sealed interface UiState.",
			},
			{
				ID:          "vm.uievent",
				Description: "Debe declarar un UiEvent (sealed interface) para los intents del usuario",
				Pattern:     regexp.MustCompile(`sealed\s+interface\s+UiEvent`),
				MustMatch:   true,
				Help:        "Declare a nested sealed interface UiEvent for user intents.",
			},
			{
				ID:          "vm.uieffect",
				Description: "Debe declarar un UiEffect (sealed interface) para one-shot events",
				Pattern:     regexp.MustCompile(`sealed\s+interface\s+UiEffect`),
				MustMatch:   true,
				Help:        "Declare a nested sealed interface UiEffect for one-shot events (snackbar, nav, ...).",
			},
			{
				ID:          "vm.stateflow",
				Description: "Debe exponer un StateFlow<UiState> público",
				Pattern:     regexp.MustCompile(`val\s+state\s*:\s*StateFlow<UiState>`),
				MustMatch:   true,
				Help:        "Expose a `val state: StateFlow<UiState>` for the UI to collect.",
			},
			{
				ID:          "vm.effects",
				Description: "Debe exponer un Flow<UiEffect> para eventos one-shot",
				Pattern:     regexp.MustCompile(`val\s+effects\s*:\s*Flow<UiEffect>`),
				MustMatch:   true,
				Help:        "Expose `val effects: Flow<UiEffect>` backed by a Channel.",
			},
			{
				ID:          "vm.onevent",
				Description: "Debe tener un entrypoint `fun onEvent(event: UiEvent)`",
				Pattern:     regexp.MustCompile(`fun\s+onEvent\s*\(\s*event\s*:\s*UiEvent\s*\)`),
				MustMatch:   true,
				Help:        "Expose `fun onEvent(event: UiEvent)` so the screen can dispatch intents.",
			},
			{
				ID:          "vm.no_livedata",
				Description: "No debe usar LiveData (preferir StateFlow)",
				Pattern:     regexp.MustCompile(`LiveData|MutableLiveData`),
				MustMatch:   false,
				Help:        "Use StateFlow instead of LiveData unless the project already uses LiveData.",
			},
		},
	})
}

const viewModelTemplate = `package <package>

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * ViewModel for <feature>.
 *
 * Sigue el patrón MVVM con UDF:
 *   - UiState: estado inmutable, una sola fuente de verdad para la UI.
 *   - UiEvent: intents del usuario (clicks, text changes, submits).
 *   - UiEffect: efectos one-shot (snackbars, navegación, toasts).
 *
 * La UI llama a [onEvent] para enviar intents; este ViewModel actualiza
 * [state] (StateFlow) y emite [effects] (Channel/ReceiveAsFlow) cuando
 * corresponde.
 */
@HiltViewModel
class <Name>ViewModel @Inject constructor(
    private val <useCaseName>: <UseCaseName>UseCase,
) : ViewModel() {

    // -----------------------------------------------------------------
    // 1) Estado: data class inmutable, una sola fuente de verdad.
    // -----------------------------------------------------------------
    data class UiState(
        val isLoading: Boolean = false,
        // TODO: agrega los campos de estado específicos de <feature>
        val error: String? = null,
    )

    // -----------------------------------------------------------------
    // 2) Eventos: intents del usuario.
    // -----------------------------------------------------------------
    sealed interface UiEvent {
        data object Load : UiEvent
        // TODO: define los intents de <feature> (data object / data class)
    }

    // -----------------------------------------------------------------
    // 3) Efectos: one-shot (snackbar, navegación, ...).
    // -----------------------------------------------------------------
    sealed interface UiEffect {
        data class ShowError(val message: String) : UiEffect
        // TODO: define los efectos de <feature> (data class / data object)
    }

    private val _state = MutableStateFlow(UiState())
    val state: StateFlow<UiState> = _state.asStateFlow()

    private val _effects = Channel<UiEffect>(Channel.BUFFERED)
    val effects: Flow<UiEffect> = _effects.receiveAsFlow()

    /**
     * Entry point: la UI envía todos los intents del usuario vía este
     * método. Mantén el 'when' exhaustivo y delega a handlers privados.
     */
    fun onEvent(event: UiEvent) {
        when (event) {
            UiEvent.Load -> load()
            // TODO: maneja el resto de UiEvent
        }
    }

    private fun load() {
        viewModelScope.launch {
            _state.value = _state.value.copy(isLoading = true, error = null)
            <useCaseName>()
                .onSuccess { /* TODO: actualiza _state con el resultado */ }
                .onFailure { e ->
                    _state.value = _state.value.copy(isLoading = false, error = e.message)
                    _effects.send(UiEffect.ShowError(e.message ?: "Unknown error"))
                }
        }
    }
}
`

// ---------------------------------------------------------------------------
// COMPOSABLE SCREEN
// ---------------------------------------------------------------------------

func init() {
	registerSpec(Spec{
		Role:         RoleComposable,
		DisplayName:  "Composable Screen (UDF)",
		Description:  "Pantalla Compose que consume un ViewModel con UDF (state + onEvent).",
		FileNameHint: "{Name}Screen.kt",
		Template: composableTemplate,
		Rules: []Rule{
			{
				ID:          "cmp.annot",
				Description: "Debe tener al menos una @Composable",
				Pattern:     regexp.MustCompile(`@Composable`),
				MustMatch:   true,
				Help:        "Add @Composable above the screen function.",
			},
			{
				ID:          "cmp.name",
				Description: "El nombre debe terminar en `Screen` (convención)",
				Pattern:     regexp.MustCompile(`fun\s+\w+Screen\s*\(`),
				MustMatch:   true,
				Help:        "Name the composable function ending in `Screen`.",
			},
			{
				ID:          "cmp.hvm",
				Description: "Debe inyectar el ViewModel con hiltViewModel()",
				Pattern:     regexp.MustCompile(`hiltViewModel\s*\(\s*\)`),
				MustMatch:   true,
				Help:        "Default the viewModel parameter with `viewModel: XxxViewModel = hiltViewModel()`.",
			},
			{
				ID:          "cmp.state",
				Description: "Debe coleccionar el state con collectAsStateWithLifecycle (o collectAsState)",
				Pattern:     regexp.MustCompile(`collectAsState(?:WithLifecycle)?\s*\(\s*\)`),
				MustMatch:   true,
				Help:        "Use `viewModel.state.collectAsStateWithLifecycle()` to consume state.",
			},
			{
				ID:          "cmp.dispatch",
				Description: "Debe despachar eventos al ViewModel (llamar onEvent / onAction)",
				Pattern:     regexp.MustCompile(`\.onEvent\s*\(`),
				MustMatch:   true,
				Help:        "Wire user input to `viewModel.onEvent(XxxUiEvent.Whatever)`.",
			},
		},
	})
}

const composableTemplate = `package <package>

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle

/**
 * Pantalla de <feature>. Sigue UDF:
 *   - 'state' viene del ViewModel (StateFlow<UiState>).
 *   - 'onEvent' despacha intents al ViewModel.
 *
 * NO accedas a repositorios, use cases, DataStore, etc. directamente
 * desde un Composable: hazlo a través del ViewModel.
 */
@Composable
fun <Name>Screen(
    modifier: Modifier = Modifier,
    viewModel: <Name>ViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()

    Scaffold(modifier = modifier.fillMaxSize()) { padding ->
        Column(
            modifier = Modifier.padding(padding).fillMaxSize(),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center,
        ) {
            if (state.isLoading) {
                CircularProgressIndicator()
            }

            // TODO: pinta el contenido de <feature> a partir de 'state'

            state.error?.let { msg ->
                Text(text = msg)
            }

            // Despacha intents al ViewModel: la UI nunca llama
            // directamente al use case o al repositorio.
            Button(onClick = { viewModel.onEvent(<Name>ViewModel.UiEvent.Load) }) {
                Text("Load")
            }
        }
    }
}

@Preview(showBackground = true)
@Composable
private fun <Name>ScreenPreview() {
    // TODO: provee un UiState de muestra para la preview
    <Name>Screen()
}
`

// ---------------------------------------------------------------------------
// ACTIVITY
// ---------------------------------------------------------------------------

func init() {
	registerSpec(Spec{
		Role:         RoleActivity,
		DisplayName:  "Activity (Compose host)",
		Description:  "Activity Compose moderna: ComponentActivity + setContent { AppTheme { ... } }.",
		FileNameHint: "{Name}Activity.kt",
		Template: activityTemplate,
		Rules: []Rule{
			{
				ID:          "act.extends",
				Description: "Debe extender ComponentActivity (recomendado) o AppCompatActivity",
				Pattern:     regexp.MustCompile(`:\s*(ComponentActivity|AppCompatActivity)\s*\(\s*\)`),
				MustMatch:   true,
				Help:        "Extend ComponentActivity() (or AppCompatActivity() if the project uses AppCompat).",
			},
			{
				ID:          "act.setcontent",
				Description: "Debe llamar setContent { ... } en onCreate",
				Pattern:     regexp.MustCompile(`setContent\s*\{`),
				MustMatch:   true,
				Help:        "Call setContent { AppTheme { /* root composable */ } } inside onCreate.",
			},
			{
				ID:          "act.theme",
				Description: "Debe envolver el contenido con un theme Compose (AppTheme, MaterialTheme, ...)",
				Pattern:     regexp.MustCompile(`(App|Material)Theme\b`),
				MustMatch:   true,
				Help:        "Wrap your composable with AppTheme { ... } (or MaterialTheme { ... }).",
			},
			{
				ID:          "act.manifest",
				Description: "Debe estar anotada con @AndroidEntryPoint si usa Hilt",
				Pattern:     regexp.MustCompile(`@AndroidEntryPoint`),
				MustMatch:   true,
				Help:        "Add @AndroidEntryPoint above the class if any field is @Inject.",
			},
		},
	})
}

const activityTemplate = `package <package>

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Surface
import androidx.compose.ui.Modifier
import dagger.hilt.android.AndroidEntryPoint
import <appPackage>.ui.theme.AppTheme

/**
 * Activity host para <feature>. Toda la UI vive en composables; esta
 * clase sólo configura el theme y monta el grafo de navegación.
 */
@AndroidEntryPoint
class <Name>Activity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            AppTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    // TODO: monta el NavHost o el composable raíz de <feature>
                }
            }
        }
    }
}
`

// ---------------------------------------------------------------------------
// USE CASE
// ---------------------------------------------------------------------------

func init() {
	registerSpec(Spec{
		Role:         RoleUseCase,
		DisplayName:  "Use Case (operator fun invoke)",
		Description:  "Caso de uso con operador invoke y @Inject, devuelve Result<T>.",
		FileNameHint: "{Action}{Feature}UseCase.kt",
		Template: useCaseTemplate,
		Rules: []Rule{
			{
				ID:          "uc.invoke",
				Description: "Debe declarar `suspend operator fun invoke(...)`",
				Pattern:     regexp.MustCompile(`suspend\s+operator\s+fun\s+invoke\s*\(`),
				MustMatch:   true,
				Help:        "Add `suspend operator fun invoke(...)` as the public entry point.",
			},
			{
				ID:          "uc.inject",
				Description: "Debe ser @Inject-injectable (constructor con @Inject o anotado)",
				Pattern:     regexp.MustCompile(`@Inject\s+constructor|class\s+\w+UseCase\s*\(`),
				MustMatch:   true,
				Help:        "Use `@Inject constructor(...)` so Hilt can build it.",
			},
			{
				ID:          "uc.result",
				Description: "Recomendado: devolver Result<T> (o Flow<T>) en lugar de throw",
				Pattern:     regexp.MustCompile(`Result<|Flow<`),
				MustMatch:   true,
				Help:        "Return Result<T> or Flow<T>; never throw across the domain boundary.",
			},
		},
	})
}

const useCaseTemplate = `package <package>

import <repositoryPackage>
import javax.inject.Inject

/**
 * Caso de uso: <descripción corta>. Mantén esta clase con una sola
 * responsabilidad; si crece, divídela en varios UseCase.
 */
class <Name>UseCase @Inject constructor(
    private val repository: <RepositoryName>,
) {
    suspend operator fun invoke(
        // TODO: parámetros de entrada
    ): Result<<ReturnType>> = runCatching {
        // TODO: implementar la lógica de dominio
    }
}
`

// ---------------------------------------------------------------------------
// REPOSITORY
// ---------------------------------------------------------------------------

func init() {
	registerSpec(Spec{
		Role:         RoleRepository,
		DisplayName:  "Repository (interfaz + impl)",
		Description:  "Interfaz de repositorio e implementación, sin dependencias Android.",
		FileNameHint: "{Name}Repository.kt + {Name}RepositoryImpl.kt",
		Template: repositoryTemplate,
		Rules: []Rule{
			{
				ID:          "repo.iface",
				Description: "Debe declarar una `interface XxxRepository`",
				Pattern:     regexp.MustCompile(`interface\s+\w+Repository\b`),
				MustMatch:   true,
				Help:        "Define an `interface XxxRepository` with the public contract.",
			},
			{
				ID:          "repo.impl",
				Description: "Debe existir una `class XxxRepositoryImpl : XxxRepository`",
				Pattern:     regexp.MustCompile(`class\s+\w+RepositoryImpl[\s\S]*?:\s*\w+Repository\b`),
				MustMatch:   true,
				Help:        "Provide a `class XxxRepositoryImpl : XxxRepository` with the actual implementation.",
			},
			{
				ID:          "repo.no_android",
				Description: "No debe importar android.* en el repositorio (lógica de dominio)",
				Pattern:     regexp.MustCompile(`(?m)^import\s+android\.`),
				MustMatch:   false,
				Help:        "Repositories should not depend on android.* — keep them framework-free for testability.",
			},
			{
				ID:          "repo.suspend_or_flow",
				Description: "Las funciones deben ser `suspend` o devolver `Flow<...>`",
				Pattern:     regexp.MustCompile(`(suspend\s+fun|Flow<|suspend\s+fun\s+\w+\s*:\s*\w+)`),
				MustMatch:   true,
				Help:        "Repository methods should be `suspend` or return `Flow<...>`.",
			},
		},
	})
}

const repositoryTemplate = `package <package>

import kotlinx.coroutines.flow.Flow

/**
 * Contrato del repositorio de <feature>. Define QUÉ se puede hacer,
 * no CÓMO. La implementación vive en [XxxRepositoryImpl].
 */
interface <Name>Repository {
    // TODO: define aquí las operaciones de dominio
    // suspend fun doX(...): Result<T>
    // fun observeX(): Flow<List<X>>
}

class <Name>RepositoryImpl(
    // TODO: inyecta el/los data source (DAO, service, ...) aquí.
    // private val remote: <Name>Service,
    // private val local: <Name>Dao,
) : <Name>Repository {
    // TODO: implementa los métodos de la interfaz
}
`


// ---------------------------------------------------------------------------
// DAO (Room)
// ---------------------------------------------------------------------------

func init() {
	registerSpec(Spec{
		Role:         RoleDao,
		DisplayName:  "Room DAO",
		Description:  "Data Access Object de Room, con @Query, @Insert, @Update, etc.",
		FileNameHint: "{Name}Dao.kt",
		Template: daoTemplate,
		Rules: []Rule{
			{
				ID:          "dao.annot",
				Description: "Debe tener @Dao",
				Pattern:     regexp.MustCompile(`@Dao\b`),
				MustMatch:   true,
				Help:        "Add @Dao above the interface/class declaration.",
			},
			{
				ID:          "dao.iface",
				Description: "Preferentemente interface (Room lo permite)",
				Pattern:     regexp.MustCompile(`(interface|abstract\s+class)\s+\w+Dao\b`),
				MustMatch:   true,
				Help:        "Declare as an interface (or abstract class) so Room can implement it.",
			},
			{
				ID:          "dao.suspend_or_flow",
				Description: "Las funciones deben ser `suspend` o devolver `Flow<...>`",
				Pattern:     regexp.MustCompile(`(suspend\s+fun|Flow<)`),
				MustMatch:   true,
				Help:        "DAO methods should be `suspend` or return `Flow<...>` to stay main-safe.",
			},
		},
	})
}

const daoTemplate = `package <package>

import androidx.room.Dao
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query
import androidx.room.Update
import kotlinx.coroutines.flow.Flow

/**
 * Data Access Object para <entity>. Room genera la implementación.
 */
@Dao
interface <Name>Dao {

    @Query("SELECT * FROM <table>")
    fun observeAll(): Flow<List<<EntityName>>>

    @Query("SELECT * FROM <table> WHERE id = :id LIMIT 1")
    suspend fun findById(id: String): <EntityName>?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(entity: <EntityName>)

    @Update
    suspend fun update(entity: <EntityName>)

    @Query("DELETE FROM <table> WHERE id = :id")
    suspend fun deleteById(id: String)
}
`

// ---------------------------------------------------------------------------
// DI MODULE
// ---------------------------------------------------------------------------

func init() {
	registerSpec(Spec{
		Role:         RoleDiModule,
		DisplayName:  "Hilt @Module",
		Description:  "Módulo Hilt con @Provides/@Binds, @InstallIn en el componente correcto.",
		FileNameHint: "{Name}Module.kt",
		Template: diModuleTemplate,
		Rules: []Rule{
			{
				ID:          "di.module",
				Description: "Debe tener @Module",
				Pattern:     regexp.MustCompile(`@Module\b`),
				MustMatch:   true,
				Help:        "Add @Module above the class/object declaration.",
			},
			{
				ID:          "di.installin",
				Description: "Debe tener @InstallIn(<Component>::class)",
				Pattern:     regexp.MustCompile(`@InstallIn\s*\(\s*\w+Component\s*::\s*class\s*\)`),
				MustMatch:   true,
				Help:        "Annotate with @InstallIn(SingletonComponent::class) (or ViewModelComponent, ...).",
			},
			{
				ID:          "di.provides_or_binds",
				Description: "Debe declarar al menos una función @Provides o @Binds",
				Pattern:     regexp.MustCompile(`@(Provides|Binds)\b`),
				MustMatch:   true,
				Help:        "Provide at least one dependency via @Provides or @Binds.",
			},
		},
	})
}

const diModuleTemplate = `package <package>

import dagger.Binds
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

/**
 * Módulo Hilt para <feature>. Mantén aquí las @Provides/@Binds.
 */
@Module
@InstallIn(SingletonComponent::class)
abstract class <Name>Module {

    // Opción A: @Binds cuando hay una única Impl
    @Binds
    @Singleton
    abstract fun bind<Name>Repository(
        impl: <Name>RepositoryImpl,
    ): <Name>Repository

    // Opción B: @Provides para casos donde la construcción requiere lógica
    // @Provides
    // @Singleton
    // fun provide<Name>Service(...): <Name>Service = ...
}
`

// ---------------------------------------------------------------------------
// DATA CLASS
// ---------------------------------------------------------------------------

func init() {
	registerSpec(Spec{
		Role:         RoleDataClass,
		DisplayName:  "Data class",
		Description:  "Data class inmutable, todas las propiedades `val`.",
		FileNameHint: "{Name}.kt",
		Template: dataClassTemplate,
		Rules: []Rule{
			{
				ID:          "dc.keyword",
				Description: "Debe ser declarado como `data class`",
				Pattern:     regexp.MustCompile(`data\s+class\s+\w+`),
				MustMatch:   true,
				Help:        "Declare the type as `data class <Name>(...)`.",
			},
			{
				ID:          "dc.val",
				Description: "Los parámetros del constructor primario deben ser `val` (inmutables)",
				Pattern:     regexp.MustCompile(`data\s+class\s+\w+\s*\(([^)]*)\)`),
				MustMatch:   true,
				Help:        "Use `val` for every primary-constructor parameter; remove any `var`.",
			},
		},
	})
}

const dataClassTemplate = `package <package>

/**
 * <descripción corta del modelo de datos>.
 *
 * Mantén esta clase plana: si crece a > 6-8 campos, plantéate
 * separarla en subtipos cohesivos.
 */
data class <Name>(
    val id: String,
    // TODO: agrega el resto de campos
)
`

// ---------------------------------------------------------------------------
// ENTITY (Room)
// ---------------------------------------------------------------------------

func init() {
	registerSpec(Spec{
		Role:         RoleEntity,
		DisplayName:  "Room @Entity",
		Description:  "Entidad Room con @Entity, @PrimaryKey, ...; sin lógica de UI.",
		FileNameHint: "{Name}Entity.kt",
		Template: entityTemplate,
		Rules: []Rule{
			{
				ID:          "ent.annot",
				Description: "Debe tener @Entity",
				Pattern:     regexp.MustCompile(`@Entity\b`),
				MustMatch:   true,
				Help:        "Add @Entity above the class declaration.",
			},
			{
				ID:          "ent.pk",
				Description: "Debe declarar un @PrimaryKey",
				Pattern:     regexp.MustCompile(`@PrimaryKey\b`),
				MustMatch:   true,
				Help:        "Mark at least one field as @PrimaryKey.",
			},
		},
	})
}

const entityTemplate = `package <package>

import androidx.room.Entity
import androidx.room.PrimaryKey

/**
 * Entidad Room para <feature>. NO incluyas lógica de UI ni de dominio
 * aquí: la entidad representa la fila de la tabla, no más.
 */
@Entity(tableName = "<table>")
data class <Name>Entity(
    @PrimaryKey val id: String,
    // TODO: agrega el resto de columnas
)
`

// ---------------------------------------------------------------------------
// NAV ROUTE
// ---------------------------------------------------------------------------

func init() {
	registerSpec(Spec{
		Role:         RoleNavRoute,
		DisplayName:  "Nav Route (sealed)",
		Description:  "Definición sellada de rutas para Navigation Compose.",
		FileNameHint: "{Name}Routes.kt",
		Template: navRouteTemplate,
		Rules: []Rule{
			{
				ID:          "nav.sealed",
				Description: "Debe ser una `sealed class/interface` o `object` con sufijo Routes",
				Pattern:     regexp.MustCompile(`(sealed\s+(class|interface)|object)\s+\w+(Routes|Nav)\b`),
				MustMatch:   true,
				Help:        "Declare routes as `object XxxRoutes` (or `sealed class XxxRoutes`).",
			},
			{
				ID:          "nav.route",
				Description: "Debe incluir el prefijo de ruta y al menos una ruta concreta",
				Pattern:     regexp.MustCompile(`const\s+val\s+\w+\s*:\s*String\s*=\s*"`),
				MustMatch:   true,
				Help:        "Expose route constants as `const val Home: String = \"home\"`.",
			},
		},
	})
}

const navRouteTemplate = `package <package>

/**
 * Rutas de navegación de <feature>. Centraliza TODAS las rutas aquí
 * para evitar strings sueltos en el NavHost.
 *
 * Convención:
 *   - Prefijo de feature: "<feature>/"
 *   - Rutas parametrizadas: "<feature>/{id}"
 */
object <Name>Routes {
    const val GRAPH = "<feature>"
    const val HOME = "<feature>/home"
    const val DETAIL = "<feature>/detail/{id}"
    const val ARG_ID = "id"
    fun detail(id: String): String = "<feature>/detail/$id"
}
`

// ---------------------------------------------------------------------------
// PUBLIC API
// ---------------------------------------------------------------------------

// SpecFor returns the spec for a role, or an error if the role is
// unknown. It is the single entry point used by both the CLI and the
// agent tool.
func SpecFor(role Role) (Spec, error) {
	s, ok := specs[role]
	if !ok {
		return Spec{}, fmt.Errorf("unknown role %q (supported: %s)", role, joinRoles(AllRoles()))
	}
	return s, nil
}

// RenderTemplate takes a template and applies the substitutions:
//
//	<package>        → pkg
//	<appPackage>     → appPkg
//	<repositoryPackage> → repoPkg
//	<Name>           → name
//	<feature>        → feature (lower-cased)
//	<UseCaseName>    → useCaseName
//	<useCaseName>    → useCaseCamel
//	<RepositoryName> → repoName
//	<EntityName>     → entityName
//	<table>          → tableName
//	<ReturnType>     → returnType
//
// Unknown placeholders are left untouched so the agent can see them.
func RenderTemplate(tpl string, vars TemplateVars) string {
	r := strings.NewReplacer(
		"<package>", vars.Package,
		"<appPackage>", vars.AppPackage,
		"<repositoryPackage>", vars.RepositoryPackage,
		"<Name>", vars.Name,
		"<feature>", vars.Feature,
		"<UseCaseName>", vars.UseCaseName,
		"<useCaseName>", vars.UseCaseCamel,
		"<RepositoryName>", vars.RepositoryName,
		"<EntityName>", vars.EntityName,
		"<table>", vars.Table,
		"<ReturnType>", vars.ReturnType,
	)
	return r.Replace(tpl)
}

// TemplateVars are the substitution values for RenderTemplate.
type TemplateVars struct {
	Package            string
	AppPackage         string
	RepositoryPackage  string
	Name               string
	Feature            string
	UseCaseName        string
	UseCaseCamel       string
	RepositoryName     string
	EntityName         string
	Table              string
	ReturnType         string
}

// Validate runs every rule of the role's spec against content and
// returns the list of issues. The list is sorted (errors first, then
// warnings, then by line) so the output is stable for snapshots.
func Validate(content string, role Role) []Issue {
	spec, err := SpecFor(role)
	if err != nil {
		return []Issue{{Severity: "error", Rule: "role", Message: err.Error()}}
	}

	var issues []Issue
	for _, r := range spec.Rules {
		matched := r.Pattern.MatchString(content)
		if r.MustMatch && !matched {
			issues = append(issues, Issue{
				Severity: "error",
				Rule:     r.ID,
				Message:  fmt.Sprintf("%s — %s", r.Description, r.Help),
				Line:     firstLineMatching(content, r.Pattern, true),
			})
		}
		if !r.MustMatch && matched {
			issues = append(issues, Issue{
				Severity: "warning",
				Rule:     r.ID,
				Message:  r.Help,
				Line:     firstLineMatching(content, r.Pattern, false),
			})
		}
	}

	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Severity != issues[j].Severity {
			return issues[i].Severity == "error"
		}
		return issues[i].Line < issues[j].Line
	})
	return issues
}

// firstLineMatching returns the 1-based line of the first line of
// content that matches re. If `mustMatch` is true we look for any
// line; if false we look for the offending line. -1 when no match.
func firstLineMatching(content string, re *regexp.Regexp, _ bool) int {
	lines := strings.Split(content, "\n")
	for i, ln := range lines {
		if re.MatchString(ln) {
			return i + 1
		}
	}
	return -1
}

// Summary returns a one-line "OK (n warnings)" or "FAIL (n errors, m
// warnings)" string for the given issues. Used by the CLI for the
// exit-status line.
func Summary(issues []Issue) string {
	var errors, warnings int
	for _, i := range issues {
		switch i.Severity {
		case "error":
			errors++
		case "warning":
			warnings++
		}
	}
	switch {
	case errors == 0 && warnings == 0:
		return "OK"
	case errors == 0:
		return fmt.Sprintf("OK with %d warning(s)", warnings)
	default:
		return fmt.Sprintf("FAIL: %d error(s), %d warning(s)", errors, warnings)
	}
}

func joinRoles(roles []Role) string {
	parts := make([]string, len(roles))
	for i, r := range roles {
		parts[i] = string(r)
	}
	return strings.Join(parts, ", ")
}
