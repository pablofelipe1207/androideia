package scaffold

import (
	"strings"
	"testing"
)

func TestAllRolesValid(t *testing.T) {
	for _, r := range AllRoles() {
		if !IsValidRole(r) {
			t.Errorf("AllRoles returned %q but IsValidRole says false", r)
		}
		spec, err := SpecFor(r)
		if err != nil {
			t.Errorf("SpecFor(%q): %v", r, err)
			continue
		}
		if spec.Role != r {
			t.Errorf("role mismatch: %q vs %q", spec.Role, r)
		}
		if spec.Template == "" {
			t.Errorf("role %q has empty template", r)
		}
		if len(spec.Rules) == 0 {
			t.Errorf("role %q has no rules", r)
		}
	}
}

func TestSpecForUnknown(t *testing.T) {
	_, err := SpecFor("nope")
	if err == nil {
		t.Fatal("expected error for unknown role")
	}
	if !strings.Contains(err.Error(), "unknown role") {
		t.Errorf("expected 'unknown role' in error, got %q", err.Error())
	}
}

func TestRenderTemplate(t *testing.T) {
	tpl := "package <package> class <Name>ViewModel(val repo: <RepositoryName>)"
	got := RenderTemplate(tpl, TemplateVars{
		Package:        "com.x.auth",
		Name:           "Login",
		RepositoryName: "AuthRepository",
	})
	want := "package com.x.auth class LoginViewModel(val repo: AuthRepository)"
	if got != want {
		t.Errorf("RenderTemplate\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderTemplate_UnknownPlaceholder(t *testing.T) {
	// Unknown placeholders must be left untouched so the agent sees them.
	tpl := "package <package> use <Unknown>"
	got := RenderTemplate(tpl, TemplateVars{Package: "x"})
	if !strings.Contains(got, "<Unknown>") {
		t.Errorf("expected <Unknown> to remain, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Validation tests per role
// ---------------------------------------------------------------------------

func TestValidate_ViewModel_GoldenPath(t *testing.T) {
	src := `package com.x.auth
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import javax.inject.Inject

@HiltViewModel
class LoginViewModel @Inject constructor(
    private val useCase: LoginUseCase,
) : ViewModel() {

    data class UiState(val isLoading: Boolean = false, val error: String? = null)
    sealed interface UiEvent { data object Load : UiEvent }
    sealed interface UiEffect { data class ShowError(val m: String) : UiEffect }

    private val _state = MutableStateFlow(UiState())
    val state: StateFlow<UiState> = _state.asStateFlow()

    private val _effects = Channel<UiEffect>(Channel.BUFFERED)
    val effects: Flow<UiEffect> = _effects.receiveAsFlow()

    fun onEvent(event: UiEvent) { /* ... */ }
}
`
	issues := Validate(src, RoleViewModel)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d: %+v", len(issues), issues)
	}
}

func TestValidate_ViewModel_Empty(t *testing.T) {
	issues := Validate("", RoleViewModel)
	if len(issues) == 0 {
		t.Fatal("expected issues for an empty file")
	}
	rulesSeen := map[string]bool{}
	for _, i := range issues {
		rulesSeen[i.Rule] = true
	}
	mustHave := []string{"vm.hilt", "vm.viewmodel", "vm.inject", "vm.uistate", "vm.uievent", "vm.uieffect", "vm.stateflow", "vm.effects", "vm.onevent"}
	for _, r := range mustHave {
		if !rulesSeen[r] {
			t.Errorf("expected rule %q to fire on empty VM, did not. issues: %+v", r, issues)
		}
	}
}

func TestValidate_ViewModel_RejectsLiveData(t *testing.T) {
	src := `package x
import androidx.lifecycle.LiveData
import androidx.lifecycle.MutableLiveData
@HiltViewModel class A @Inject constructor() : ViewModel() {
    data class UiState(val a: Int = 0)
    sealed interface UiEvent
    sealed interface UiEffect
    val state: StateFlow<UiState> = MutableStateFlow(UiState()).asStateFlow()
    val effects: Flow<UiEffect> = Channel<UiEffect>(Channel.BUFFERED).receiveAsFlow()
    fun onEvent(event: UiEvent) {}
}`
	issues := Validate(src, RoleViewModel)
	found := false
	for _, i := range issues {
		if i.Rule == "vm.no_livedata" {
			found = true
			if i.Severity != "warning" {
				t.Errorf("expected warning severity for LiveData, got %s", i.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected vm.no_livedata warning, got issues: %+v", issues)
	}
}

func TestValidate_Composable_GoldenPath(t *testing.T) {
	src := `package x.ui
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle

@Composable
fun LoginScreen(
    viewModel: LoginViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    Button(onClick = { viewModel.onEvent(LoginViewModel.UiEvent.Load) }) { Text("Go") }
}`
	issues := Validate(src, RoleComposable)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %+v", issues)
	}
}

func TestValidate_Composable_NoHiltVM(t *testing.T) {
	src := `@Composable
fun LoginScreen() { /* ... */ }`
	issues := Validate(src, RoleComposable)
	found := false
	for _, i := range issues {
		if i.Rule == "cmp.hvm" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected cmp.hvm rule to fire, issues: %+v", issues)
	}
}

func TestValidate_Activity_GoldenPath(t *testing.T) {
	src := `package x
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import dagger.hilt.android.AndroidEntryPoint

@AndroidEntryPoint
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent { AppTheme { Surface { /* ... */ } } }
    }
}`
	issues := Validate(src, RoleActivity)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %+v", issues)
	}
}

func TestValidate_UseCase_GoldenPath(t *testing.T) {
	src := `package x.domain
import javax.inject.Inject
class LoginUseCase @Inject constructor(
    private val repo: UserRepository,
) {
    suspend operator fun invoke(u: String, p: String): Result<User> = runCatching { repo.login(u, p) }
}`
	issues := Validate(src, RoleUseCase)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %+v", issues)
	}
}

func TestValidate_Repository_GoldenPath(t *testing.T) {
	src := `package x.data
import kotlinx.coroutines.flow.Flow
interface UserRepository {
    suspend fun login(u: String, p: String): Result<User>
    fun observeUser(id: String): Flow<User?>
}
class UserRepositoryImpl(
    private val remote: UserService,
) : UserRepository {
    override suspend fun login(u: String, p: String): Result<User> = runCatching { remote.login(u, p) }
    override fun observeUser(id: String): Flow<User?> = remote.observeUser(id)
}`
	issues := Validate(src, RoleRepository)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %+v", issues)
	}
}

func TestValidate_Repository_RejectsAndroidImport(t *testing.T) {
	src := `package x.data
import android.content.Context
import kotlinx.coroutines.flow.Flow
interface UserRepository { fun x(): Flow<Int> }
class UserRepositoryImpl(private val ctx: Context) : UserRepository { override fun x() = TODO() }`
	issues := Validate(src, RoleRepository)
	found := false
	for _, i := range issues {
		if i.Rule == "repo.no_android" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected repo.no_android warning, got: %+v", issues)
	}
}

func TestValidate_Dao_GoldenPath(t *testing.T) {
	src := `package x.data.local
import androidx.room.Dao
import androidx.room.Query
import kotlinx.coroutines.flow.Flow

@Dao
interface UserDao {
    @Query("SELECT * FROM users") fun observeAll(): Flow<List<UserEntity>>
    @Query("SELECT * FROM users WHERE id = :id") suspend fun findById(id: String): UserEntity?
}`
	issues := Validate(src, RoleDao)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %+v", issues)
	}
}

func TestValidate_DiModule_GoldenPath(t *testing.T) {
	src := `package x.di
import dagger.Binds
import dagger.Module
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
abstract class AuthModule {
    @Binds @Singleton
    abstract fun bindRepo(impl: AuthRepositoryImpl): AuthRepository
}`
	issues := Validate(src, RoleDiModule)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %+v", issues)
	}
}

func TestValidate_DataClass_GoldenPath(t *testing.T) {
	src := `package x.model
data class User(val id: String, val name: String, val email: String)`
	issues := Validate(src, RoleDataClass)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %+v", issues)
	}
}

func TestValidate_Entity_GoldenPath(t *testing.T) {
	src := `package x.data.local
import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "users")
data class UserEntity(
    @PrimaryKey val id: String,
    val name: String,
)`
	issues := Validate(src, RoleEntity)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %+v", issues)
	}
}

func TestValidate_NavRoute_GoldenPath(t *testing.T) {
	src := `package x.nav
object AuthRoutes {
    const val GRAPH = "auth"
    const val HOME: String = "auth/home"
    const val DETAIL: String = "auth/detail/{id}"
    const val ARG_ID = "id"
    fun detail(id: String): String = "auth/detail/$id"
}`
	issues := Validate(src, RoleNavRoute)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %+v", issues)
	}
}

func TestValidate_UnknownRole(t *testing.T) {
	issues := Validate("anything", "not-a-role")
	if len(issues) != 1 || issues[0].Rule != "role" {
		t.Errorf("expected single role error, got %+v", issues)
	}
}

func TestSummary(t *testing.T) {
	cases := []struct {
		issues []Issue
		want   string
	}{
		{nil, "OK"},
		{[]Issue{{Severity: "warning", Rule: "x", Message: "y"}}, "OK with 1 warning(s)"},
		{[]Issue{{Severity: "error", Rule: "x", Message: "y"}}, "FAIL: 1 error(s), 0 warning(s)"},
		{[]Issue{{Severity: "error", Rule: "a", Message: "x"}, {Severity: "warning", Rule: "b", Message: "y"}}, "FAIL: 1 error(s), 1 warning(s)"},
	}
	for _, c := range cases {
		got := Summary(c.issues)
		if got != c.want {
			t.Errorf("Summary(%v) = %q, want %q", c.issues, got, c.want)
		}
	}
}

func TestValidate_IssuesSortedErrorsFirst(t *testing.T) {
	src := ""
	issues := Validate(src, RoleViewModel)
	if len(issues) < 2 {
		t.Fatal("expected many issues on empty VM")
	}
	for i := 1; i < len(issues); i++ {
		if issues[i-1].Severity == "warning" && issues[i].Severity == "error" {
			t.Errorf("issues not sorted errors-first: %+v", issues)
		}
	}
}

func TestViewModelTemplate_Renderable(t *testing.T) {
	spec, _ := SpecFor(RoleViewModel)
	rendered := RenderTemplate(spec.Template, TemplateVars{
		Package:      "com.x.auth",
		Name:         "Login",
		Feature:      "login",
		UseCaseName:  "Login",
		UseCaseCamel: "login",
	})
	// Rendered template must itself validate clean: the agent
	// fills the TODOs but keeps the contract.
	issues := Validate(rendered, RoleViewModel)
	if len(issues) != 0 {
		t.Errorf("rendered VM template has %d issues: %+v", len(issues), issues)
	}
}

func TestComposableTemplate_Renderable(t *testing.T) {
	spec, _ := SpecFor(RoleComposable)
	rendered := RenderTemplate(spec.Template, TemplateVars{
		Package: "com.x.auth.ui",
		Name:    "Login",
	})
	issues := Validate(rendered, RoleComposable)
	if len(issues) != 0 {
		t.Errorf("rendered Composable template has %d issues: %+v", len(issues), issues)
	}
}

func TestUseCaseTemplate_Renderable(t *testing.T) {
	spec, _ := SpecFor(RoleUseCase)
	rendered := RenderTemplate(spec.Template, TemplateVars{
		Package:         "com.x.auth.domain",
		Name:            "Login",
		RepositoryName:  "UserRepository",
		RepositoryPackage: "com.x.auth.data.repository",
		ReturnType:      "User",
	})
	issues := Validate(rendered, RoleUseCase)
	if len(issues) != 0 {
		t.Errorf("rendered UseCase template has %d issues: %+v", len(issues), issues)
	}
}

func TestRepositoryTemplate_Renderable(t *testing.T) {
	spec, _ := SpecFor(RoleRepository)
	rendered := RenderTemplate(spec.Template, TemplateVars{
		Package: "com.x.auth.data.repository",
		Name:    "User",
	})
	issues := Validate(rendered, RoleRepository)
	if len(issues) != 0 {
		t.Errorf("rendered Repository template has %d issues: %+v", len(issues), issues)
	}
}
