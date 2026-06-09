package interview

import "math/rand"

type Category string

const (
	CategoryCompose      Category = "compose"
	CategoryArchitecture Category = "architecture"
	CategoryDI           Category = "di"
	CategoryAsync        Category = "async"
	CategoryStorage      Category = "storage"
	CategoryNavigation   Category = "navigation"
	CategoryTesting      Category = "testing"
)

type Difficulty string

const (
	DifficultyEasy   Difficulty = "easy"
	DifficultyMedium Difficulty = "medium"
	DifficultyHard   Difficulty = "hard"
)

type Question struct {
	ID         string
	Category   Category
	Difficulty Difficulty
	Question   string
	Options    []string
	Answer     int
	Explanation string
}

var questionBank = []Question{
	// COMPOSE - Easy
	{
		ID: "compose_01", Category: CategoryCompose, Difficulty: DifficultyEasy,
		Question:   "¿Qué anotación se usa para marcar una función como Composable en Jetpack Compose?",
		Options:   []string{"@Composable", "@posable", "@Compose", "@UIComponent"},
		Answer:    0,
		Explanation: "@Composable es la anotación que le indica al compilador que la función es un componente UI en Compose.",
	},
	{
		ID: "compose_02", Category: CategoryCompose, Difficulty: DifficultyEasy,
		Question:   "¿Qué función se usa para mantener y observar un estado en Compose?",
		Options:   []string{"remember", "mutableStateOf", "stateOf", "observableState"},
		Answer:    0,
		Explanation: "remember { mutableStateOf() } es la combinación correcta. remember preserva el estado entre recomposiciones.",
	},
	{
		ID: "compose_03", Category: CategoryCompose, Difficulty: DifficultyEasy,
		Question:   "¿Qué hace la función 'LaunchedEffect' en Compose?",
		Options:   []string{
			"Ejecuta un efecto secundario al entrar en composición",
			"Crea un estado persistente",
			"Optimiza las recomposiciones",
			"Maneja la navegación",
		},
		Answer:    0,
		Explanation: "LaunchedEffect ejecuta código en una coroutine que se cancela cuando el composable sale del tree.",
	},
	{
		ID: "compose_04", Category: CategoryCompose, Difficulty: DifficultyMedium,
		Question:   "¿Cuál es la diferencia entre 'mutableStateOf' y 'mutableStateListOf'?",
		Options:   []string{
			"mutableStateOf es para un solo valor, mutableStateListOf para listas observables",
			"No hay diferencia",
			"mutableStateListOf es más rápido",
			"mutableStateOf es para primitivos, mutableStateListOf para objetos",
		},
		Answer:    0,
		Explanation: "mutableStateOf crea un State<T> para un valor, mientras que mutableStateListOf crea una MutableList que notifica cambios.",
	},
	{
		ID: "compose_05", Category: CategoryCompose, Difficulty: DifficultyMedium,
		Question:   "¿Qué es 'derivedStateOf' y cuándo usarlo?",
		Options:   []string{
			"Crea un estado derivado que solo se recalcula cuando cambian sus inputs",
			"Es una alternativa a mutableStateOf",
			"Optimiza la memoria de los estados",
			"Maneja estados asíncronos",
		},
		Answer:    0,
		Explanation: "derivedStateOf evita recomposiciones innecesarias calculando un valor derivado solo cuando los estados que lee cambian.",
	},
	{
		ID: "compose_06", Category: CategoryCompose, Difficulty: DifficultyHard,
		Question:   "¿Qué problema resuelve 'Stable' y 'Immutable' en Compose?",
		Options:   []string{
			"Ayudan al compilador a evitar recomposiciones innecesarias",
			"Previenen memory leaks",
			"Optimizan el manejo de memoria",
			"Mejoran la seguridad del código",
		},
		Answer:    0,
		Explanation: "@Stable e @Immutable son acuerdos de estabilidad que permiten al composador skip recomposiciones cuando los parámetros no cambian.",
	},

	// ARCHITECTURE - Easy
	{
		ID: "arch_01", Category: CategoryArchitecture, Difficulty: DifficultyEasy,
		Question:   "¿Qué significa MVVM?",
		Options:   []string{
			"Model-View-ViewModel",
			"Module-View-ViewModel",
			"Manager-View-ViewModel",
			"Model-View-VirtualMachine",
		},
		Answer:    0,
		Explanation: "MVVM es Model-View-ViewModel, un patrón que separa la lógica de negocio de la UI.",
	},
	{
		ID: "arch_02", Category: CategoryArchitecture, Difficulty: DifficultyEasy,
		Question:   "¿Cuál es el rol del ViewModel en MVVM?",
		Options:   []string{
			"Mantener y proveer datos para la UI, sobreviviendo cambios de configuración",
			"Manejar la base de datos",
			"Hacer llamadas de red",
			"Navegar entre pantallas",
		},
		Answer:    0,
		Explanation: "El ViewModel actúa como puente entre la UI y el dominio, sobreviviendo a rotaciones de pantalla.",
	},
	{
		ID: "arch_03", Category: CategoryArchitecture, Difficulty: DifficultyMedium,
		Question:   "¿Qué es MVI y cómo difiere de MVVM?",
		Options:   []string{
			"MVI usa un flujo unidireccional de datos con Intents, MVVM usa binding directo",
			"No hay diferencia",
			"MVI es más rápido que MVVM",
			"MVI no requiere ViewModel",
		},
		Answer:    0,
		Explanation: "MVI (Model-View-Intent) enfatiza un ciclo unidireccional: Intent → Model → View, facilitando el testeo y debugging.",
	},
	{
		ID: "arch_04", Category: CategoryArchitecture, Difficulty: DifficultyMedium,
		Question:   "¿Qué es Clean Architecture en Android?",
		Options:   []string{
			"Una arquitectura que separa capas: domain, data, presentation",
			"Un patrón de diseño para UI",
			"Una librería de Google",
			"Una forma de estructurar archivos",
		},
		Answer:    0,
		Explanation: "Clean Architecture separa responsabilidades en capas independientes: domain (casos de uso), data (fuentes), presentation (UI).",
	},
	{
		ID: "arch_05", Category: CategoryArchitecture, Difficulty: DifficultyHard,
		Question:   "¿Qué es el patrón 'Repository' y por qué es importante?",
		Options:   []string{
			"Abstrae las fuentes de datos, permitiendo cambiar entre local/remoto sin afectar la UI",
			"Es un patrón de diseño creacional",
			"Maneja la navegación entre pantallas",
			"Optimiza el rendimiento de la app",
		},
		Answer:    0,
		Explanation: "Repository centraliza el acceso a datos, facilitando testing con mocks y cambiando fuentes sin modificar la lógica de negocio.",
	},

	// DI - Easy
	{
		ID: "di_01", Category: CategoryDI, Difficulty: DifficultyEasy,
		Question:   "¿Qué es la Inyección de Dependencias?",
		Options:   []string{
			"Un patrón donde los objetos reciben sus dependencias de fuentes externas",
			"Un tipo de testing",
			"Una forma de crear bases de datos",
			"Un patrón de diseño de UI",
		},
		Answer:    0,
		Explanation: "DI permite que las clases reciban sus dependencias en lugar de crearlas, facilitando testing y desacoplamiento.",
	},
	{
		ID: "di_02", Category: CategoryDI, Difficulty: DifficultyMedium,
		Question:   "¿Qué anotación se usa en Hilt para inyectar dependencias en un ViewModel?",
		Options:   []string{"@HiltViewModel", "@Inject", "@Module", "@Provides"},
		Answer:    0,
		Explanation: "@HiltViewModel marca un ViewModel para que Hilt pueda inyectar sus dependencias automáticamente.",
	},
	{
		ID: "di_03", Category: CategoryDI, Difficulty: DifficultyMedium,
		Question:   "¿Cuál es la diferencia entre @Provides y @Binds en Hilt?",
		Options:   []string{
			"@Provides crea instancias nuevas, @Binds une una implementación a una interfaz",
			"@Provides es para objetos, @Binds para funciones",
			"No hay diferencia",
			"@Binds es más rápido que @Provides",
		},
		Answer:    0,
		Explanation: "@Provides ejecuta código para crear instancias; @Binds solo declara que una implementación satisface una interfaz, sin código adicional.",
	},

	// ASYNC - Easy
	{
		ID: "async_01", Category: CategoryAsync, Difficulty: DifficultyEasy,
		Question:   "¿Qué son las Coroutines en Kotlin?",
		Options:   []string{
			"Hilos ligeros para programación asíncrona",
			"Una nueva forma de crear threads",
			"Un reemplazo de AsyncTask",
			"Un tipo de variable",
		},
		Answer:    0,
		Explanation: "Las coroutines son hilos gestionados por Kotlin que permiten código asíncrono limpio sin callbacks anidados.",
	},
	{
		ID: "async_02", Category: CategoryAsync, Difficulty: DifficultyEasy,
		Question:   "¿Qué hace la función 'suspend' en Kotlin?",
		Options:   []string{
			"Marca una función que puede pausar y reanudar su ejecución",
			"Detiene el hilo completamente",
			"Es igual que Thread.sleep()",
			"Crea un nuevo hilo",
		},
		Answer:    0,
		Explanation: "suspend permite que la función se pause sin bloquear el hilo, liberándolo para otras tareas mientras espera.",
	},
	{
		ID: "async_03", Category: CategoryAsync, Difficulty: DifficultyMedium,
		Question:   "¿Cuál es la diferencia entre StateFlow y SharedFlow?",
		Options:   []string{
			"StateFlow tiene valor inicial y emite el último valor; SharedFlow no tiene valor inicial",
			"StateFlow es más rápido",
			"SharedFlow es para UI, StateFlow para red",
			"No hay diferencia",
		},
		Answer:    0,
		Explanation: "StateFlow siempre tiene un valor (como LiveData), SharedFlow es un emisor de eventos sin estado inicial.",
	},
	{
		ID: "async_04", Category: CategoryAsync, Difficulty: DifficultyMedium,
		Question:   "¿Qué es 'Dispatchers.IO' y cuándo usarlo?",
		Options:   []string{
			"Dispatcher optimizado para operaciones de I/O (red, base de datos)",
			"Dispatcher para la UI principal",
			"Dispatcher para operaciones matemáticas",
			"Dispatcher para testing",
		},
		Answer:    0,
		Explanation: "Dispatchers.IO usa un pool de hilos optimizado para operaciones que bloquean como redes o bases de datos.",
	},
	{
		ID: "async_05", Category: CategoryAsync, Difficulty: DifficultyHard,
		Question:   "¿Qué problema resuelve 'flowOn' en Kotlin Flow?",
		Options:   []string{
			"Cambia el dispatcher donde se ejecuta upstream sin afectar downstream",
			"Cambia el orden de los elementos",
			"Filtra elementos del flow",
			"Convierte un Flow en StateFlow",
		},
		Answer:    0,
		Explanation: "flowOn cambia el contexto de ejecución aguas arriba, útil para hacer I/O sin bloquear el hilo de UI.",
	},

	// STORAGE - Easy
	{
		ID: "storage_01", Category: CategoryStorage, Difficulty: DifficultyEasy,
		Question:   "¿Qué es Room en Android?",
		Options:   []string{
			"Una biblioteca de persistencia sobre SQLite",
			"Un sistema de archivos",
			"Un gestor de caché",
			"Un reemplazo de SharedPreferences",
		},
		Answer:    0,
		Explanation: "Room provee una capa de abstracción sobre SQLite con verificación en tiempo de compilación y menos boilerplate.",
	},
	{
		ID: "storage_02", Category: CategoryStorage, Difficulty: DifficultyMedium,
		Question:   "¿Qué anotación se usa para definir una consulta SQL en Room?",
		Options:   []string{"@Query", "@Insert", "@Update", "@Database"},
		Answer:    0,
		Explanation: "@Query permite escribir consultas SQL personalizadas que se verifican en compilación.",
	},
	{
		ID: "storage_03", Category: CategoryStorage, Difficulty: DifficultyMedium,
		Question:   "¿Qué es DataStore y por qué reemplaza SharedPreferences?",
		Options:   []string{
			"API moderna con soporte para coroutines y type-safety",
			"Es más rápido que SharedPreferences",
			"No requiere permisos",
			"Es una base de datos",
		},
		Answer:    0,
		Explanation: "DataStore usa Kotlin Coroutines y Flow, es thread-safe, y soporta serialización de tipos complejos.",
	},

	// NAVIGATION - Easy
	{
		ID: "nav_01", Category: CategoryNavigation, Difficulty: DifficultyEasy,
		Question:   "¿Qué biblioteca se usa para navegar en apps Jetpack Compose?",
		Options:   []string{
			"Navigation Compose",
			"Activity Navigator",
			"Intent Router",
			"Route Manager",
		},
		Answer:    0,
		Explanation: "Navigation Compose es la biblioteca oficial de Google para manejar la navegación en apps Compose.",
	},
	{
		ID: "nav_02", Category: CategoryNavigation, Difficulty: DifficultyMedium,
		Question:   "¿Qué función se usa para definir rutas en Navigation Compose?",
		Options:   []string{"composable()", "navigate()", "route()", "screen()"},
		Answer:    0,
		Explanation: "composable() dentro de NavHost define una pantalla con su ruta y contenido.",
	},
	{
		ID: "nav_03", Category: CategoryNavigation, Difficulty: DifficultyMedium,
		Question:   "¿Cómo se pasan argumentos de navegación en Compose?",
		Options:   []string{
			"Usando rutas con argumentos tipo '{id}' y NavBackStackEntry",
			"Usando Intents",
			"Usando ViewModel compartido",
			"Usando Bundle directamente",
		},
		Answer:    0,
		Explanation: "Se definen rutas como 'user/{id}' y se accede con navBackStackEntry.arguments?.getString(\"id\").",
	},

	// TESTING - Easy
	{
		ID: "test_01", Category: CategoryTesting, Difficulty: DifficultyEasy,
		Question:   "¿Qué es JUnit en Android?",
		Options:   []string{
			"Un framework de testing unitario",
			"Una librería de testing UI",
			"Un工具 de build",
			"Un lenguaje de programación",
		},
		Answer:    0,
		Explanation: "JUnit es el framework estándar para tests unitarios en JVM, incluyendo Android.",
	},
	{
		ID: "test_02", Category: CategoryTesting, Difficulty: DifficultyMedium,
		Question:   "¿Qué es Mockk y cuándo se usa?",
		Options:   []string{
			"Librería para crear mocks en tests Kotlin",
			"Un testing framework",
			"Una librería de red",
			"Un generador de código",
		},
		Answer:    0,
		Explanation: "Mockk permite crear mocks y spies de clases Kotlin de forma nativa, ideal para tests unitarios.",
	},
	{
		ID: "test_03", Category: CategoryTesting, Difficulty: DifficultyMedium,
		Question:   "¿Qué es espresso en Android?",
		Options:   []string{
			"Framework de testing de UI para interacciones del usuario",
			"Una librería de networking",
			"Un sistema de build",
			"Un ORM de base de datos",
		},
		Answer:    0,
		Explanation: "Espresso permite automatizar tests de UI simulando interacciones reales del usuario.",
	},

	// MORE COMPOSE - Medium/Hard
	{
		ID: "compose_07", Category: CategoryCompose, Difficulty: DifficultyMedium,
		Question:   "¿Qué es 'SideEffect' en Compose?",
		Options:   []string{
			"Un efecto que se ejecuta después de cada recomposición exitosa",
			"Un error de composición",
			"Una optimización de rendimiento",
			"Un tipo de estado",
		},
		Answer:    0,
		Explanation: "SideEffect ejecuta código después de que la composición se ha aplicado al framework, ideal para analytics.",
	},
	{
		ID: "compose_08", Category: CategoryCompose, Difficulty: DifficultyMedium,
		Question:   "¿Qué es 'DisposableEffect' y por qué es importante?",
		Options:   []string{
			"Ejecuta cleanup cuando el key cambia o el composable sale del tree",
			"Es igual que LaunchedEffect",
			"Crea efectos visuales",
			"Maneja errores de composición",
		},
		Answer:    0,
		Explanation: "DisposableEffect es crucial para liberar recursos (listeners, observers) cuando ya no se necesitan.",
	},
	{
		ID: "compose_09", Category: CategoryCompose, Difficulty: DifficultyHard,
		Question:   "¿Qué es 'derivedStateOf' y por qué optimiza el rendimiento?",
		Options:   []string{
			"Crea un estado que solo se recalcula cuando cambian sus dependencias",
			"Es una caché de estados",
			"Reduce el uso de memoria",
			"Optimiza el garbage collector",
		},
		Answer:    0,
		Explanation: "derivedStateOf evita recomposiciones innecesarias calculando valores derivados solo cuando los inputs cambian.",
	},
	{
		ID: "compose_10", Category: CategoryCompose, Difficulty: DifficultyHard,
		Question:   "¿Qué problema resuelve 'key' en LazyColumn?",
		Options:   []string{
			"Identifica únicamente cada ítem para optimizar recomposiciones",
			"Ordena los elementos",
			"Filtra elementos",
			"Crea animaciones",
		},
		Answer:    0,
		Explanation: "key ayuda a Compose a identificar qué ítems cambiaron, movieron o se agregaron, optimizando las actualizaciones.",
	},

	// MORE ARCHITECTURE
	{
		ID: "arch_06", Category: CategoryArchitecture, Difficulty: DifficultyMedium,
		Question:   "¿Qué es un 'UseCase' en Clean Architecture?",
		Options:   []string{
			"Un objeto que encapsula una regla de negocio específica",
			"Un tipo de ViewModel",
			"Una fuente de datos",
			"Un componente de UI",
		},
		Answer:    0,
		Explanation: "UseCases encapsulan lógica de negocio reutilizable, manteniendo al ViewModel delgado y enfocado en la UI.",
	},
	{
		ID: "arch_07", Category: CategoryArchitecture, Difficulty: DifficultyHard,
		Question:   "¿Qué es 'Unidirectional Data Flow' (UDF) y por qué es importante?",
		Options:   []string{
			"Los datos fluyen en una dirección: Model → View → Intent → Model",
			"Los datos van de la UI a la base de datos directamente",
			"Es un patrón de diseño creacional",
			"Optimiza el rendimiento de la app",
		},
		Answer:    0,
		Explanation: "UDF hace el estado predecible y fácil de testear: la UI emite intents, el modelo actualiza el estado, la UI se redibuja.",
	},

	// MORE DI
	{
		ID: "di_04", Category: CategoryDI, Difficulty: DifficultyHard,
		Question:   "¿Qué es un 'Component' en Hilt y cuándo se usa?",
		Options:   []string{
			"Un contenedor de dependencias que gestiona el ciclo de vida de los módulos",
			"Un componente de UI",
			"Un tipo de ViewModel",
			"Una anotación de testing",
		},
		Answer:    0,
		Explanation: "Los Components de Hilt gestionan el ciclo de vida de las dependencias (Application, Activity, ViewModel, etc.).",
	},

	// MORE ASYNC
	{
		ID: "async_06", Category: CategoryAsync, Difficulty: DifficultyHard,
		Question:   "¿Qué es 'Channel' en Kotlin y cómo difiere de Flow?",
		Options:   []string{
			"Channel es para comunicación entre coroutines; Flow es para emitir múltiples valores",
			"Son lo mismo",
			"Channel es más rápido que Flow",
			"Flow es para un solo valor",
		},
		Answer:    0,
		Explanation: "Channel es un tubo de comunicación entre coroutines (como Queue), mientras que Flow emite una secuencia de valores.",
	},

	// MORE TESTING
	{
		ID: "test_04", Category: CategoryTesting, Difficulty: DifficultyHard,
		Question:   "¿Qué es 'Turbine' en testing de Kotlin Flow?",
		Options:   []string{
			"Librería para testear Flows de forma sencilla y legible",
			"Un tipo de test",
			"Una librería de mocking",
			"Un generador de datos de prueba",
		},
		Answer:    0,
		Explanation: "Turbine permite testear Flows como si fueran listas, facilitando la verificación de emisiones.",
	},

	// MORE STORAGE
	{
		ID: "storage_04", Category: CategoryStorage, Difficulty: DifficultyHard,
		Question:   "¿Qué es un 'TypeConverter' en Room?",
		Options:   []string{
			"Convierte tipos complejos a tipos que SQLite puede almacenar",
			"Convierte Strings a Integers",
			"Optimiza las consultas",
			"Crea índices automáticos",
		},
		Answer:    0,
		Explanation: "TypeConverter permite guardar objetos complejos (Date, List, custom) convirtiéndolos a tipos primitivos.",
	},
}

// GetQuestionsByCategory retorna preguntas filtradas por categoría.
func GetQuestionsByCategory(category Category) []Question {
	var result []Question
	for _, q := range questionBank {
		if q.Category == category {
			result = append(result, q)
		}
	}
	return result
}

// GetQuestionsByDifficulty retorna preguntas filtradas por dificultad.
func GetQuestionsByDifficulty(difficulty Difficulty) []Question {
	var result []Question
	for _, q := range questionBank {
		if q.Difficulty == difficulty {
			result = append(result, q)
		}
	}
	return result
}

// GetFilteredQuestions retorna preguntas filtradas por categoría y dificultad.
func GetFilteredQuestions(category Category, difficulty Difficulty) []Question {
	var result []Question
	for _, q := range questionBank {
		if (category == "" || q.Category == category) &&
			(difficulty == "" || q.Difficulty == difficulty) {
			result = append(result, q)
		}
	}
	return result
}

// GetRandomQuestions obtiene N preguntas aleatorias del banco filtrado.
func GetRandomQuestions(category Category, difficulty Difficulty, count int) []Question {
	filtered := GetFilteredQuestions(category, difficulty)
	if count <= 0 || count > len(filtered) {
		count = len(filtered)
	}

	// Fisher-Yates shuffle
	shuffled := make([]Question, len(filtered))
	copy(shuffled, filtered)
	for i := len(shuffled) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	return shuffled[:count]
}

// AllCategories devuelve todas las categorías disponibles.
func AllCategories() []Category {
	return []Category{
		CategoryCompose,
		CategoryArchitecture,
		CategoryDI,
		CategoryAsync,
		CategoryStorage,
		CategoryNavigation,
		CategoryTesting,
	}
}

// AllDifficulties devuelve todas las dificultades disponibles.
func AllDifficulties() []Difficulty {
	return []Difficulty{
		DifficultyEasy,
		DifficultyMedium,
		DifficultyHard,
	}
}
