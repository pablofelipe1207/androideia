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
- **NUNCA sobrescribas código existente**: cuando agregues una feature, crea archivos
  nuevos. No modifiques ni borres código en archivos existentes (MainActivity,
  AndroidManifest, navegación, etc.) a menos que la tarea lo pida explícitamente.
  Preserva siempre el contenido original y agrega junto a él.

## Regla crítica — No destruir código existente

- **NO** sobrescribas archivos existentes. Siempre crea archivos nuevos para código nuevo.
- Si una feature requiere cambios en un archivo existente (ej. agregar una ruta al NavHost),
  **lee el archivo primero** y **agrega** el nuevo código sin eliminar el existente.
- Si MainActivity ya tiene `setContent { ... }` con contenido, **no lo reemplaces**.
  Agrega imports y código nuevo sin tocando el bloque existente.
- Prefiere crear un nuevo archivo (ej. `NuevoFeatureScreen.kt`) a modificar uno existente
  que ya funciona.
- Excepción: solo cuando el usuario diga explícitamente "reemplaza este archivo" o
  "sobrescribe este archivo". En ese caso, confirma con `confirm_plan` primero.

## Regla crítica — Consistencia de diseño

- **Features existentes**: **NUNCA** cambies el diseño visual, UI/UX, temas, colores,
  tipografía o estructura de navegación de features que ya existen. Su diseño es
  inmutable a menos que el usuario pida explícitamente un rediseño.
- **Nuevas features/pantallas**: **DEBES** seguir el diseño de las pantallas existentes.
  Antes de crear una nueva Screen, usa `androideai feature <existente>` o
  `semantic_locate type=composable` para inspeccionar pantallas actuales y copia:
  - Estructura de Composables (Column, LazyColumn, Scaffold, etc.)
  - Uso de `MaterialTheme`, `colors`, `typography`, `shapes`
  - Espaciado, padding, márgenes (usa `Spacing`, `Dimens` del tema)
  - Estilo de botones, inputs, tarjetas, barras de navegación
  - Patrones de estado (Loading, Error, Empty, Success)
  - Animaciones y transiciones si las hay
- Si no hay pantallas previas, usa la plantilla canónica de `android_scaffold`
  con `action=template role=composable` y valida con `validate_kotlin`.

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
