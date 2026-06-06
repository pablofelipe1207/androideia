package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pablofelipe1207/androideia/internal/android"
	"github.com/pablofelipe1207/androideia/internal/store"
	"github.com/spf13/cobra"
)

var androidCmd = &cobra.Command{
	Use:   "android",
	Short: "Operaciones de Android (Gradle, adb, emulador)",
	Long:  `Ejecuta y gestiona operaciones de desarrollo Android.`,
}

var gradleCmd = &cobra.Command{
	Use:   "gradle [task]",
	Short: "Ejecuta tareas de Gradle",
	Long:  `Ejecuta una tarea de Gradle y muestra el resultado.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		task := args[0]
		fmt.Printf("Running Gradle task: %s\n\n", task)

		// Open store
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Create android instance
		a := android.NewAndroid(s.DB())

		// Execute gradle task
		result, err := a.Gradle(task)
		if err != nil {
			return fmt.Errorf("error running gradle: %w", err)
		}

		// Print result
		fmt.Printf("Task: %s\n", result.Task)
		fmt.Printf("Status: %s\n", result.Status)
		fmt.Printf("Duration: %s\n\n", result.Duration)

		if result.Error != "" {
			fmt.Printf("Error:\n%s\n\n", result.Error)
		}

		if result.Log != "" {
			fmt.Printf("Output:\n%s\n", result.Log)
		}

		return nil
	},
}

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Ejecuta tests de Android",
	Long:  `Ejecuta tests de Android (unitarios o instrumentados).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		unit, _ := cmd.Flags().GetBool("unit")
		instrumented, _ := cmd.Flags().GetBool("instrumented")

		if unit && instrumented {
			return fmt.Errorf("cannot run both unit and instrumented tests")
		}

		if !unit && !instrumented {
			unit = true // default to unit tests
		}

		testType := "unit"
		if instrumented {
			testType = "instrumented"
		}
		fmt.Printf("Running %s tests...\n\n", testType)

		// Open store
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Create android instance
		a := android.NewAndroid(s.DB())

		// Execute tests
		result, err := a.Test(unit)
		if err != nil {
			return fmt.Errorf("error running tests: %w", err)
		}

		// Print result
		fmt.Printf("Task: %s\n", result.Task)
		fmt.Printf("Status: %s\n", result.Status)
		fmt.Printf("Duration: %s\n\n", result.Duration)

		if result.Error != "" {
			fmt.Printf("Error:\n%s\n\n", result.Error)
		}

		if result.Log != "" {
			// Parse and summarize test results
			summary := parseTestResults(result.Log)
			fmt.Printf("Test Summary:\n%s\n", summary)
		}

		return nil
	},
}

var emulatorCmd = &cobra.Command{
	Use:   "emulator [list|start|stop|status|install|launch]",
	Short: "Gestiona el emulador de Android",
	Long:  `Lista, inicia, detiene o gestiona emuladores de Android.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		action := args[0]

		switch action {
		case "list":
			return listEmulators()
		case "start":
			if len(args) < 2 {
				return fmt.Errorf("emulator name required for start")
			}
			return startEmulator(args[1])
		case "stop":
			return stopEmulator()
		case "status":
			return getEmulatorStatus()
		case "install":
			if len(args) < 2 {
				return fmt.Errorf("APK path required for install")
			}
			return installApp(args[1])
		case "launch":
			if len(args) < 2 {
				return fmt.Errorf("package name required for launch")
			}
			return launchApp(args[1])
		default:
			return fmt.Errorf("unknown action: %s (use list, start, stop, status, install, or launch)", action)
		}
	},
}

var buildHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Muestra el historial de builds",
	Long:  `Muestra el historial reciente de builds de Gradle.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Open store
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Create android instance
		a := android.NewAndroid(s.DB())

		// Get build history
		history, err := a.GetBuildHistory(10)
		if err != nil {
			return fmt.Errorf("error getting build history: %w", err)
		}

		if len(history) == 0 {
			fmt.Println("No build history found")
			return nil
		}

		fmt.Printf("Recent builds:\n\n")
		for _, build := range history {
			fmt.Printf("Task: %s\n", build.Task)
			fmt.Printf("Status: %s\n", build.Status)
			fmt.Printf("Duration: %s\n", build.Duration)
			fmt.Println("---")
		}

		return nil
	},
}

func listEmulators() error {
	a := android.NewAndroid(nil)
	output, err := a.Emulator("list")
	if err != nil {
		return fmt.Errorf("error listing emulators: %w", err)
	}

	fmt.Printf("Available emulators:\n%s\n", output)
	return nil
}

func startEmulator(name string) error {
	a := android.NewAndroid(nil)
	output, err := a.Emulator("start", name)
	if err != nil {
		return fmt.Errorf("error starting emulator: %w", err)
	}

	fmt.Println(output)
	return nil
}

func stopEmulator() error {
	a := android.NewAndroid(nil)
	output, err := a.Emulator("stop")
	if err != nil {
		return fmt.Errorf("error stopping emulator: %w", err)
	}

	fmt.Println(output)
	return nil
}

func getEmulatorStatus() error {
	a := android.NewAndroid(nil)
	output, err := a.Emulator("status")
	if err != nil {
		return fmt.Errorf("error getting emulator status: %w", err)
	}

	fmt.Printf("Emulator status:\n%s\n", output)
	return nil
}

func installApp(apkPath string) error {
	a := android.NewAndroid(nil)
	output, err := a.Emulator("install", apkPath)
	if err != nil {
		return fmt.Errorf("error installing app: %w", err)
	}

	fmt.Println(output)
	return nil
}

func launchApp(packageName string) error {
	a := android.NewAndroid(nil)
	output, err := a.Emulator("launch", packageName)
	if err != nil {
		return fmt.Errorf("error launching app: %w", err)
	}

	fmt.Println(output)
	return nil
}

func parseTestResults(log string) string {
	// Simple parsing of test results
	lines := []string{"Tests completed."}
	// In a real implementation, you would parse the JUnit XML or Gradle output
	// to extract pass/fail counts
	return lines[0]
}

func init() {
	// Test command flags
	testCmd.Flags().BoolP("unit", "u", false, "Run unit tests")
	testCmd.Flags().BoolP("instrumented", "i", false, "Run instrumented tests")

	// Add commands
	androidCmd.AddCommand(gradleCmd)
	androidCmd.AddCommand(testCmd)
	androidCmd.AddCommand(emulatorCmd)
	androidCmd.AddCommand(buildHistoryCmd)
}
