---
name: android-scaffold
description: Genera archivos Android (ViewModel, Composable, Activity, UseCase, Repository, DAO, Hilt module, data class, Room entity, nav route) usando plantillas canónicas y los valida contra el contrato del rol.
triggers: ["crear viewmodel", "nueva pantalla", "crear usecase", "nuevo repository", "add dao", "add screen", "viewmodel template", "compose screen template", "android scaffold", "validar kotlin", "validate kotlin"]
---

# Android Scaffold Skill

Esta skill te da **plantillas canónicas** y un **validador** para que
cualquier archivo Android que escribas (ViewModel, Composable Screen,
Activity, UseCase, Repository, DAO, módulo Hilt, data class, entidad
Room, nav route) salga completo, no a medias.

## Flujo obligatorio antes de escribir un .kt

1. **Buscar referencias en el índice semántico** con `android_scaffold`
   (o `semantic_locate` para archivos ya existentes del mismo rol).
2. Si hay archivos existentes: lee uno representativo con `read_file`
   y **copia sus convenciones** (DI, async, naming, etc.).
3. Si no hay o vas a crear uno nuevo: usa `android_scaffold` con la
   acción `template` para obtener la plantilla rellena.
4. Rellena los `// TODO: <feature>` con la lógica concreta.
5. **Valida** el archivo resultante con `validate_kotlin` antes de
   pedir `confirm_plan` al usuario. Si hay errores, itera.

## Roles soportados

| Rol           | Plantilla                     | Validaciones clave                                              |
|---------------|-------------------------------|------------------------------------------------------------------|
| `viewmodel`   | MVVM con UiState/UiEvent/UiEffect, Hilt, StateFlow | @HiltViewModel, ViewModel(), @Inject, UiState, UiEvent, UiEffect, `val state: StateFlow<UiState>`, `val effects: Flow<UiEffect>`, `fun onEvent(event: UiEvent)`, no LiveData |
| `composable`  | Screen Compose con UDF y preview | @Composable, nombre `*Screen`, `hiltViewModel()`, `collectAsStateWithLifecycle()`, `.onEvent(` |
| `activity`    | ComponentActivity + setContent + AppTheme | extiende ComponentActivity/AppCompatActivity, `setContent {`, envuelto en `AppTheme`/`MaterialTheme`, `@AndroidEntryPoint` |
| `usecase`     | Operator invoke + Result | `suspend operator fun invoke(`, @Inject, devuelve `Result<` o `Flow<` |
| `repository`  | Interface + Impl sin android.* | interface + class Impl, **no** `import android.*`, funciones `suspend` o `Flow<` |
| `dao`         | @Dao interface Room | @Dao, interface, funciones `suspend`/`Flow<` |
| `di_module`   | Hilt @Module con @Provides/@Binds | @Module, @InstallIn(XxxComponent::class), @Provides o @Binds |
| `data_class`  | data class inmutable | `data class`, todos los parámetros `val` |
| `entity`      | @Entity con @PrimaryKey | @Entity, @PrimaryKey |
| `nav_route`   | object Routes con const val | object/sealed class con sufijo `Routes`/`Nav`, al menos un `const val` ruta |

## Ejemplo completo (crear un LoginViewModel)

```text
1) Buscar en semantic
   android_scaffold role=viewmodel action=check name=Login

   → Si hay un XxxViewModel existente, te lo devuelvo con su
     conventions/summary para que lo mires con read_file.
   → Si no hay, paso 2.

2) Cargar plantilla
   android_scaffold role=viewmodel action=template name=Login
                    feature=login use_case=Login

   → Te devuelvo el código completo con TODO marcados.

3) Escribir
   Rellenas los TODO: campos de UiState, intents de UiEvent,
   efectos de UiEffect, cuerpo de load(), etc.
   write_file path=... content=<código>

4) Validar
   validate_kotlin path=... role=viewmodel

   → Si hay errores, los corrijo y revalido.
   → Si pasa, llamo a confirm_plan.
```

## Reglas duras

- **NUNCA** escribir un ViewModel sin `UiState` + `UiEvent` + `UiEffect`.
- **NUNCA** escribir un Composable Screen sin `hiltViewModel()` y
  `collectAsStateWithLifecycle()`.
- **NUNCA** escribir un Repository que importe `android.*`.
- **NUNCA** entregar un archivo con `// TODO: <feature>` sin resolver
  antes de pedir `confirm_plan` al usuario (los TODO en la plantilla
  son el trabajo que TIENES que hacer).
- **SIEMPRE** correr `validate_kotlin` después de `write_file` y
  antes de `confirm_plan`.
- **NUNCA SOBRESCRIBAS código existente**. Cuando crees archivos nuevos,
  no toques los existentes a menos que la tarea lo exija explícitamente.
  Preserva siempre el contenido original y agrega junto a él.
- **CONSISTENCIA DE DISEÑO — OBLIGATORIA**:
  - Features existentes: **NO** cambies su diseño visual, temas, colores,
    tipografía, espaciado, componentes ni navegación.
  - Nuevas pantallas: **DEBES** inspeccionar pantallas existentes con
    `semantic_locate type=composable` o `android_scaffold role=composable action=check`
    y replicar su estilo (MaterialTheme, Spacing, Dimens, patrones de estado,
    botones, inputs, cards, navegación, animaciones).
