package android

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mobiai/androideai-core/internal/store"
)

func TestNewAndroid(t *testing.T) {
	// Create a temporary database
	tmpDir, err := os.MkdirTemp("", "android-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Error creating store: %v", err)
	}
	defer s.Close()

	// Create android instance
	a := NewAndroid(s.DB())
	if a == nil {
		t.Fatal("Android instance is nil")
	}
}

func TestStoreBuildHistory(t *testing.T) {
	// Create a temporary database
	tmpDir, err := os.MkdirTemp("", "android-history-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Error creating store: %v", err)
	}
	defer s.Close()

	// Create android instance
	a := NewAndroid(s.DB())

	// Create build result
	result := &BuildResult{
		Task:     "assembleDebug",
		Status:   "success",
		Log:      "BUILD SUCCESSFUL",
		Duration: 10 * time.Second,
	}

	// Store build history
	if err := a.storeBuildHistory(result); err != nil {
		t.Fatalf("Error storing build history: %v", err)
	}

	// Verify build was stored
	var count int
	err = s.DB().QueryRow("SELECT COUNT(*) FROM build_history").Scan(&count)
	if err != nil {
		t.Fatalf("Error counting builds: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 build, got %d", count)
	}
}

func TestGetBuildHistory(t *testing.T) {
	// Create a temporary database
	tmpDir, err := os.MkdirTemp("", "android-gethistory-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Error creating store: %v", err)
	}
	defer s.Close()

	// Create android instance
	a := NewAndroid(s.DB())

	// Store multiple builds with small delays to ensure different timestamps
	builds := []*BuildResult{
		{Task: "assembleDebug", Status: "success", Log: "BUILD SUCCESSFUL", Duration: 10 * time.Second},
		{Task: "testDebugUnitTest", Status: "failed", Log: "BUILD FAILED", Duration: 5 * time.Second},
		{Task: "assembleRelease", Status: "success", Log: "BUILD SUCCESSFUL", Duration: 15 * time.Second},
	}

	for i, build := range builds {
		// Add small delay to ensure different timestamps
		if i > 0 {
			time.Sleep(10 * time.Millisecond)
		}
		if err := a.storeBuildHistory(build); err != nil {
			t.Fatalf("Error storing build history: %v", err)
		}
	}

	// Get build history
	history, err := a.GetBuildHistory(10)
	if err != nil {
		t.Fatalf("Error getting build history: %v", err)
	}

	if len(history) != 3 {
		t.Errorf("Expected 3 builds, got %d", len(history))
	}

	// Verify order (most recent first)
	if history[0].Task != "assembleRelease" {
		t.Errorf("Expected most recent build to be 'assembleRelease', got '%s'", history[0].Task)
	}
}

func TestParseCompilationError(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "compilation error",
			output:   "error: unresolved reference: Foo",
			expected: "error: unresolved reference: Foo",
		},
		{
			name:     "build failure",
			output:   "FAILURE: Build failed with an exception.\n* What went wrong:\nExecution failed",
			expected: "FAILURE: Build failed with an exception.\n* What went wrong:\nExecution failed",
		},
		{
			name:     "no error",
			output:   "BUILD SUCCESSFUL",
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := parseCompilationError(test.output)
			if result != test.expected {
				t.Errorf("Expected '%s', got '%s'", test.expected, result)
			}
		})
	}
}

func TestFindProjectRoot(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "android-project-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "app", "src", "main")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Error creating subdirectory: %v", err)
	}

	// Create build.gradle in root
	buildGradle := filepath.Join(tmpDir, "build.gradle")
	if err := os.WriteFile(buildGradle, []byte("// build.gradle"), 0644); err != nil {
		t.Fatalf("Error creating build.gradle: %v", err)
	}

	// Change to subdirectory
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(subDir)

	// Find project root
	root, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("Error finding project root: %v", err)
	}

	if root != tmpDir {
		t.Errorf("Expected project root '%s', got '%s'", tmpDir, root)
	}
}
