---
name: android-feature
description: Crea una feature MVVM/Compose ubicando archivos en el módulo/paquete correctos.
triggers: ["crear feature", "nueva pantalla", "add screen", "nuevo módulo"]
---

# Android Feature Skill

Esta skill ayuda a crear una nueva feature completa en un proyecto Android siguiendo la arquitectura MVVM con Jetpack Compose.

## Flujo de trabajo

1. **Analizar estructura existente**: Usa `androideai feature <nombre>` para ver features similares
2. **Planificar capas**: Identifica qué archivos necesitas crear
3. **Crear archivos**: Genera los archivos en las ubicaciones correctas
4. **Registrar**: Actualiza rutas de navegación y módulos DI

## Capas típicas de una feature

```
feature/
├── ui/
│   ├── <Feature>Screen.kt          # Pantalla principal (Composable)
│   └── <Feature>ViewModel.kt       # ViewModel con estado
├── domain/
│   └── usecase/
│       └── <Action>UseCase.kt      # Casos de uso
├── data/
│   └── repository/
│       └── <Feature>Repository.kt  # Repositorio (interfaz)
│       └── <Feature>RepositoryImpl.kt  # Implementación
└── di/
    └── <Feature>Module.kt          # Módulo Hilt
```

## Convenciones

- **Nombres**: PascalCase para clases, camelCase para funciones
- **Paquete**: `<app>.<module>.<layer>.<feature>`
- **Anotaciones**: `@Composable`, `@HiltViewModel`, `@Inject`, `@Module`, `@InstallIn`
- **Navegación**: Usa `composable("route")` en NavHost

## Ejemplo de creación

```kotlin
// 1. Screen
@Composable
fun LoginScreen(
    viewModel: LoginViewModel = hiltViewModel()
) {
    val uiState by viewModel.uiState.collectAsState()
    // ...
}

// 2. ViewModel
@HiltViewModel
class LoginViewModel @Inject constructor(
    private val loginUseCase: LoginUseCase
) : ViewModel() {
    // ...
}

// 3. UseCase
class LoginUseCase @Inject constructor(
    private val repository: UserRepository
) {
    suspend operator fun invoke(username: String, password: String): Result<User> {
        // ...
    }
}

// 4. Repository
interface UserRepository {
    suspend fun login(username: String, password: String): Result<User>
}

// 5. Module
@Module
@InstallIn(SingletonComponent::class)
class RepositoryModule {
    @Provides
    fun provideUserRepository(impl: UserRepositoryImpl): UserRepository = impl
}
```

## Verificación

Después de crear la feature, ejecuta:
```bash
androideai feature <nombre>  # Verifica que todas las capas estén detectadas
androideai gradle assembleDebug  # Verifica compilación
```
