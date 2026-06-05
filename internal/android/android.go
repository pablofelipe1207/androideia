package android

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Android struct {
	db            *sql.DB
	sdkPath       string
	emulatorPath  string
	adbPath       string
}

func NewAndroid(db *sql.DB) *Android {
	a := &Android{db: db}
	a.detectSDK()
	return a
}

func (a *Android) detectSDK() {
	// Check ANDROID_HOME or ANDROID_SDK_ROOT
	androidHome := os.Getenv("ANDROID_HOME")
	if androidHome == "" {
		androidHome = os.Getenv("ANDROID_SDK_ROOT")
	}

	// Try common locations
	possiblePaths := []string{
		androidHome,
		filepath.Join(os.Getenv("HOME"), "Android", "Sdk"),
		"/usr/lib/android-sdk",
		"/opt/android-sdk",
	}

	for _, path := range possiblePaths {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			a.sdkPath = path
			a.emulatorPath = filepath.Join(path, "emulator", "emulator")
			a.adbPath = filepath.Join(path, "platform-tools", "adb")
			return
		}
	}

	// Fallback to PATH
	a.emulatorPath = "emulator"
	a.adbPath = "adb"
}

type BuildResult struct {
	Task     string
	Status   string
	Log      string
	Duration time.Duration
	Error    string
}

func (a *Android) Gradle(task string) (*BuildResult, error) {
	start := time.Now()

	// Check if gradlew exists
	gradlew := "./gradlew"
	if _, err := os.Stat(gradlew); os.IsNotExist(err) {
		// Try gradlew.bat on Windows
		if _, err := os.Stat(gradlew + ".bat"); os.IsNotExist(err) {
			return nil, fmt.Errorf("gradlew not found in current directory")
		}
		gradlew = "./gradlew.bat"
	}

	// Execute gradle task
	cmd := exec.Command(gradlew, task)
	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := &BuildResult{
		Task:     task,
		Status:   "success",
		Log:      string(output),
		Duration: duration,
	}

	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()

		// Parse first compilation error
		if compilationError := parseCompilationError(string(output)); compilationError != "" {
			result.Error = compilationError
		}
	}

	// Store in build history
	if err := a.storeBuildHistory(result); err != nil {
		return nil, fmt.Errorf("error storing build history: %w", err)
	}

	return result, nil
}

func (a *Android) Test(unit bool) (*BuildResult, error) {
	task := "test"
	if unit {
		task = "testDebugUnitTest"
	} else {
		task = "connectedDebugAndroidTest"
	}

	return a.Gradle(task)
}

func (a *Android) Emulator(action string, args ...string) (string, error) {
	// Check if emulator exists
	if _, err := os.Stat(a.emulatorPath); os.IsNotExist(err) {
		// Try to find in PATH
		if path, err := exec.LookPath("emulator"); err == nil {
			a.emulatorPath = path
		} else {
			return "", fmt.Errorf("emulator not found. Please set ANDROID_HOME or add emulator to PATH")
		}
	}

	switch action {
	case "list":
		return a.listEmulators()
	case "start":
		if len(args) == 0 {
			return "", fmt.Errorf("emulator name required for start")
		}
		return a.startEmulator(args[0])
	case "stop":
		return a.stopEmulator()
	case "status":
		return a.getEmulatorStatus()
	case "install":
		if len(args) == 0 {
			return "", fmt.Errorf("APK path required for install")
		}
		return a.installApp(args[0])
	case "launch":
		if len(args) == 0 {
			return "", fmt.Errorf("package name required for launch")
		}
		return a.launchApp(args[0])
	default:
		return "", fmt.Errorf("unknown emulator action: %s", action)
	}
}

func (a *Android) listEmulators() (string, error) {
	cmd := exec.Command(a.emulatorPath, "-list-avds")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error listing emulators: %w", err)
	}

	return string(output), nil
}

func (a *Android) startEmulator(name string) (string, error) {
	cmd := exec.Command(a.emulatorPath, "-avd", name, "-no-window", "-no-audio")
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("error starting emulator: %w", err)
	}

	return fmt.Sprintf("Emulator %s started (headless mode)", name), nil
}

func (a *Android) stopEmulator() (string, error) {
	cmd := exec.Command(a.adbPath, "emu", "kill")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error stopping emulator: %w", err)
	}

	return string(output), nil
}

func (a *Android) getEmulatorStatus() (string, error) {
	cmd := exec.Command(a.adbPath, "devices", "-l")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error getting emulator status: %w", err)
	}

	return string(output), nil
}

func (a *Android) installApp(apkPath string) (string, error) {
	cmd := exec.Command(a.adbPath, "install", "-r", apkPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error installing app: %w", err)
	}

	return string(output), nil
}

func (a *Android) launchApp(packageName string) (string, error) {
	// Get the main activity
	cmd := exec.Command(a.adbPath, "shell", "cmd", "package", "resolve-activity", "--brief", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error resolving activity: %w", err)
	}

	// Parse the activity from output
	lines := strings.Split(string(output), "\n")
	var activity string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "/") && !strings.HasPrefix(line, "priority") {
			activity = line
			break
		}
	}

	if activity == "" {
		return "", fmt.Errorf("could not find main activity for %s", packageName)
	}

	// Launch the activity
	cmd = exec.Command(a.adbPath, "shell", "am", "start", "-n", activity)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error launching app: %w", err)
	}

	return fmt.Sprintf("App %s launched successfully", packageName), nil
}

func parseCompilationError(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Look for compilation errors
		if strings.Contains(line, "error:") || strings.Contains(line, "Error:") {
			return strings.TrimSpace(line)
		}
		if strings.Contains(line, "FAILURE: Build failed") {
			// Get the next few lines for context
			for i, l := range lines {
				if strings.Contains(l, "FAILURE: Build failed") {
					end := i + 3
					if end > len(lines) {
						end = len(lines)
					}
					return strings.Join(lines[i:end], "\n")
				}
			}
		}
	}
	return ""
}

func (a *Android) storeBuildHistory(result *BuildResult) error {
	// Use UnixNano for more precise timestamps
	_, err := a.db.Exec(
		`INSERT INTO build_history (task, status, log, ts) VALUES (?, ?, ?, ?)`,
		result.Task, result.Status, result.Log, time.Now().UnixNano(),
	)
	if err != nil {
		return err
	}
	return nil
}

func (a *Android) GetBuildHistory(limit int) ([]BuildResult, error) {
	rows, err := a.db.Query(
		`SELECT task, status, log, ts FROM build_history ORDER BY ts DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []BuildResult
	for rows.Next() {
		var result BuildResult
		var ts int64
		if err := rows.Scan(&result.Task, &result.Status, &result.Log, &ts); err != nil {
			return nil, err
		}
		result.Duration = time.Duration(0)
		results = append(results, result)
	}

	return results, nil
}

func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		// Check for build.gradle or build.gradle.kts
		if _, err := os.Stat(filepath.Join(dir, "build.gradle")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, "build.gradle.kts")); err == nil {
			return dir, nil
		}

		// Check for settings.gradle
		if _, err := os.Stat(filepath.Join(dir, "settings.gradle")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, "settings.gradle.kts")); err == nil {
			return dir, nil
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("Android project root not found")
}
