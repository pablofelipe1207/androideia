package index

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mobiai/androideai-core/internal/store"
)

func TestNewFeature(t *testing.T) {
	feature := NewFeature("login")
	if feature.Name != "login" {
		t.Errorf("Expected feature name 'login', got '%s'", feature.Name)
	}
	if len(feature.Layers) != 0 {
		t.Errorf("Expected 0 layers, got %d", len(feature.Layers))
	}
}

func TestAddLayer(t *testing.T) {
	feature := NewFeature("login")

	// Add screen layer
	screen := &FeatureLayer{
		Name: "LoginScreen",
		Kind: "screen",
		Path: "ui/login/LoginScreen.kt",
		Line: 10,
	}
	feature.AddLayer(screen)

	// Add viewmodel layer
	viewmodel := &FeatureLayer{
		Name: "LoginViewModel",
		Kind: "viewmodel",
		Path: "ui/login/LoginViewModel.kt",
		Line: 5,
	}
	feature.AddLayer(viewmodel)

	// Add another screen
	screen2 := &FeatureLayer{
		Name: "LoginButton",
		Kind: "composable",
		Path: "ui/login/LoginButton.kt",
		Line: 1,
	}
	feature.AddLayer(screen2)

	// Check layers
	if len(feature.Layers) != 3 {
		t.Errorf("Expected 3 layers, got %d", len(feature.Layers))
	}

	if len(feature.Layers["screen"]) != 1 {
		t.Errorf("Expected 1 screen layer, got %d", len(feature.Layers["screen"]))
	}

	if len(feature.Layers["viewmodel"]) != 1 {
		t.Errorf("Expected 1 viewmodel layer, got %d", len(feature.Layers["viewmodel"]))
	}

	if len(feature.Layers["composable"]) != 1 {
		t.Errorf("Expected 1 composable layer, got %d", len(feature.Layers["composable"]))
	}
}

func TestGetLayerOrder(t *testing.T) {
	feature := NewFeature("login")

	// Add layers in random order
	layers := []string{"module", "repository", "viewmodel", "screen", "usecase"}
	for _, kind := range layers {
		feature.AddLayer(&FeatureLayer{
			Name: "test",
			Kind: kind,
		})
	}

	// Get order
	order := feature.GetLayerOrder()

	// Verify order (screen should be first, module last)
	expectedOrder := []string{"screen", "viewmodel", "usecase", "repository", "module"}
	if len(order) != len(expectedOrder) {
		t.Errorf("Expected %d layers, got %d", len(expectedOrder), len(order))
	}

	for i, kind := range expectedOrder {
		if order[i] != kind {
			t.Errorf("Expected order[%d] = '%s', got '%s'", i, kind, order[i])
		}
	}
}

func TestFormat(t *testing.T) {
	feature := NewFeature("login")

	// Add layers
	feature.AddLayer(&FeatureLayer{
		Name:      "LoginScreen",
		Kind:      "screen",
		Path:      "ui/login/LoginScreen.kt",
		Line:      10,
		Module:    "app",
		Package:   "com.example.login",
		Signature: "@Composable fun LoginScreen()",
	})

	feature.AddLayer(&FeatureLayer{
		Name:      "LoginViewModel",
		Kind:      "viewmodel",
		Path:      "ui/login/LoginViewModel.kt",
		Line:      5,
		Module:    "app",
		Package:   "com.example.login",
		Signature: "class LoginViewModel : ViewModel()",
	})

	// Format
	output := feature.Format()

	// Check output contains expected content
	if len(output) == 0 {
		t.Error("Format output is empty")
	}

	// Check that it contains the feature name
	if !containsString(output, "Feature: login") {
		t.Error("Format output does not contain feature name")
	}

	// Check that it contains layer information
	if !containsString(output, "LoginScreen") {
		t.Error("Format output does not contain LoginScreen")
	}

	if !containsString(output, "LoginViewModel") {
		t.Error("Format output does not contain LoginViewModel")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGetFeatureByName(t *testing.T) {
	// Create a temporary database
	tmpDir, err := os.MkdirTemp("", "feature-test")
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

	// Insert test data
	_, err = s.DB().Exec(`INSERT INTO files (path, package, module, layer, hash, updated_at) 
		VALUES ('ui/login/LoginScreen.kt', 'com.example.login', 'app', 'ui', 'hash1', strftime('%s', 'now'))`)
	if err != nil {
		t.Fatalf("Error inserting file: %v", err)
	}

	_, err = s.DB().Exec(`INSERT INTO symbols (file_id, name, kind, signature, line, feature) 
		VALUES (1, 'LoginScreen', 'screen', '@Composable fun LoginScreen()', 10, 'login')`)
	if err != nil {
		t.Fatalf("Error inserting symbol: %v", err)
	}

	// Get feature
	feature, err := GetFeatureByName(s.DB(), "login")
	if err != nil {
		t.Fatalf("Error getting feature: %v", err)
	}

	if feature.Name != "login" {
		t.Errorf("Expected feature name 'login', got '%s'", feature.Name)
	}

	if len(feature.Layers) != 1 {
		t.Errorf("Expected 1 layer, got %d", len(feature.Layers))
	}

	if _, exists := feature.Layers["screen"]; !exists {
		t.Error("Expected screen layer")
	}
}

func TestGetFeatureByNameNotFound(t *testing.T) {
	// Create a temporary database
	tmpDir, err := os.MkdirTemp("", "feature-notfound-test")
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

	// Try to get non-existent feature
	_, err = GetFeatureByName(s.DB(), "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent feature")
	}
}
