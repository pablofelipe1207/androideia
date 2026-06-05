package index

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWalker(t *testing.T) {
	// Use testdata directory - need to go up two levels from internal/index/
	walker := NewWalker("../../testdata")
	if err := walker.LoadGitignore(); err != nil {
		t.Fatalf("Error loading .gitignore: %v", err)
	}

	files, err := walker.Walk()
	if err != nil {
		t.Fatalf("Error walking files: %v", err)
	}

	if len(files) == 0 {
		t.Error("No files found in testdata")
	}

	// Check that we found Kotlin files
	foundKt := false
	for _, file := range files {
		ext := filepath.Ext(file.Path)
		if ext == ".kt" {
			foundKt = true
		}
	}

	if !foundKt {
		t.Error("No .kt files found")
	}
}

func TestKotlinExtractor(t *testing.T) {
	extractor := NewKotlinExtractor()

	// Test file content
	content := `package com.example.testapp.ui.login

import androidx.compose.runtime.Composable

@Composable
fun LoginScreen() {
    // Login screen implementation
}

class LoginViewModel : ViewModel() {
    fun login(username: String, password: String) {
        // Login logic
    }
}
`

	symbols := extractor.ExtractSymbols("test.kt", content)

	// Should find LoginScreen (composable) and LoginViewModel (viewmodel)
	if len(symbols) < 2 {
		t.Errorf("Expected at least 2 symbols, got %d", len(symbols))
	}

	// Check for specific symbols
	foundComposable := false
	foundViewModel := false
	for _, sym := range symbols {
		if sym.Kind == "composable" && sym.Name == "LoginScreen" {
			foundComposable = true
		}
		if sym.Kind == "viewmodel" && sym.Name == "LoginViewModel" {
			foundViewModel = true
		}
	}

	if !foundComposable {
		t.Error("LoginScreen composable not found")
	}
	if !foundViewModel {
		t.Error("LoginViewModel not found")
	}
}

func TestInferPackage(t *testing.T) {
	extractor := NewKotlinExtractor()

	content := `package com.example.testapp.ui.login

import androidx.compose.runtime.Composable

@Composable
fun LoginScreen() {
    // Login screen implementation
}
`

	pkg := extractor.InferPackage(content)
	if pkg != "com.example.testapp.ui.login" {
		t.Errorf("Expected package 'com.example.testapp.ui.login', got '%s'", pkg)
	}
}

func TestInferModule(t *testing.T) {
	extractor := NewKotlinExtractor()

	// Test different paths
	tests := []struct {
		path     string
		expected string
	}{
		{"app/src/main/java/com/example/LoginScreen.kt", "app"},
		{"feature/src/main/java/LoginScreen.kt", "feature"},
		{"src/main/java/LoginScreen.kt", "app"}, // default
	}

	for _, test := range tests {
		module := extractor.InferModule(test.path)
		if module != test.expected {
			t.Errorf("For path %s, expected module '%s', got '%s'", test.path, test.expected, module)
		}
	}
}

func TestInferLayer(t *testing.T) {
	extractor := NewKotlinExtractor()

	// Test different files
	tests := []struct {
		path     string
		content  string
		expected string
	}{
		{"app/src/main/java/ui/LoginScreen.kt", "@Composable fun LoginScreen()", "ui"},
		{"app/src/main/java/viewmodel/LoginViewModel.kt", "class LoginViewModel", "domain"},
		{"app/src/main/java/usecase/LoginUseCase.kt", "class LoginUseCase", "domain"},
		{"app/src/main/java/repository/UserRepository.kt", "interface UserRepository", "data"},
		{"app/src/main/java/di/RepositoryModule.kt", "@Module class RepositoryModule", "di"},
		{"app/src/main/java/nav/AppNavigation.kt", "NavHost", "nav"},
	}

	for _, test := range tests {
		layer := extractor.InferLayer(test.path, test.content)
		if layer != test.expected {
			t.Errorf("For path %s, expected layer '%s', got '%s'", test.path, test.expected, layer)
		}
	}
}

func TestExtractFeature(t *testing.T) {
	extractor := NewKotlinExtractor()

	// Test with screen symbol
	symbols := []Symbol{
		{Name: "LoginScreen", Kind: "screen", Signature: "fun LoginScreen()", Line: 1},
	}

	feature := extractor.ExtractFeature(symbols)
	if feature != "login" {
		t.Errorf("Expected feature 'login', got '%s'", feature)
	}

	// Test with composable symbol
	symbols = []Symbol{
		{Name: "LoginButton", Kind: "composable", Signature: "fun LoginButton()", Line: 1},
	}

	feature = extractor.ExtractFeature(symbols)
	if feature != "loginbutton" {
		t.Errorf("Expected feature 'loginbutton', got '%s'", feature)
	}
}

func TestCalculateHash(t *testing.T) {
	walker := NewWalker(".")

	// Create a temporary file
	tmpDir, err := os.MkdirTemp("", "hash-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Error writing test file: %v", err)
	}

	hash1, err := walker.CalculateHash(testFile)
	if err != nil {
		t.Fatalf("Error calculating hash: %v", err)
	}

	// Calculate again - should be the same
	hash2, err := walker.CalculateHash(testFile)
	if err != nil {
		t.Fatalf("Error calculating hash second time: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("Hashes should be equal: %s != %s", hash1, hash2)
	}

	// Modify file and check hash changes
	if err := os.WriteFile(testFile, []byte("different content"), 0644); err != nil {
		t.Fatalf("Error modifying test file: %v", err)
	}

	hash3, err := walker.CalculateHash(testFile)
	if err != nil {
		t.Fatalf("Error calculating hash after modification: %v", err)
	}

	if hash1 == hash3 {
		t.Error("Hash should change after file modification")
	}
}
