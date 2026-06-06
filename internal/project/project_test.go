package project

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

const classicManifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="com.example.myapplication">

    <application
        android:label="MyApp"
        android:theme="@style/Theme.MyApp">
        <activity
            android:name=".MainActivity"
            android:exported="true" />
        <activity
            android:name=".feature.detail.DetailActivity" />
    </application>
</manifest>
`

const modernManifestNoPackage = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">

    <application
        android:label="MyApp">
        <activity
            android:name=".MainActivity" />
    </application>
</manifest>
`

const modernBuildGradle = `plugins {
    alias(libs.plugins.android.application)
}
android {
    namespace = "com.example.myapplication"
}
`

const libsVersions = `# Project-wide Gradle settings.
[versions]
agp = "8.2.0"
kotlin = "1.9.22"
compose-bom = "2024.02.00"
hilt = "2.50"

[libraries]
androidx-core-ktx = { module = "androidx.core:core-ktx", version.ref = "agp" }
androidx-compose-bom = { module = "androidx.compose:compose-bom", version.ref = "compose-bom" }
hilt-android = { module = "com.google.dagger:hilt-android", version.ref = "hilt" }

[plugins]
android-application = { id = "com.android.application", version.ref = "agp" }
`

func TestDiscover_ClassicManifestWithPackage(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "app/src/main/AndroidManifest.xml"), classicManifest)

	md, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if md.ApplicationID != "com.example.myapplication" {
		t.Errorf("ApplicationID = %q, want com.example.myapplication", md.ApplicationID)
	}
	if len(md.ManifestActivities) != 2 {
		t.Errorf("expected 2 activities, got %d (%v)", len(md.ManifestActivities), md.ManifestActivities)
	}
}

func TestDiscover_Agp8NamespaceFromBuildGradle(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "app/src/main/AndroidManifest.xml"), modernManifestNoPackage)
	writeFile(t, filepath.Join(root, "app/build.gradle.kts"), modernBuildGradle)

	md, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if md.ApplicationID != "com.example.myapplication" {
		t.Errorf("ApplicationID = %q, want com.example.myapplication", md.ApplicationID)
	}
}

func TestDiscover_LibsVersionsToml(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "app/src/main/AndroidManifest.xml"), classicManifest)
	writeFile(t, filepath.Join(root, "gradle/libs.versions.toml"), libsVersions)

	md, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if md.LibsVersionsPath == "" {
		t.Fatal("LibsVersionsPath is empty")
	}
	wantVersions := map[string]string{
		"agp":         "8.2.0",
		"kotlin":      "1.9.22",
		"compose-bom": "2024.02.00",
		"hilt":        "2.50",
	}
	for k, v := range wantVersions {
		if got := md.LibsVersions[k]; got != v {
			t.Errorf("LibsVersions[%q] = %q, want %q", k, got, v)
		}
	}
	wantLibs := map[string]string{
		"androidx-core-ktx":  "androidx.core:core-ktx",
		"androidx-compose-bom": "androidx.compose:compose-bom",
		"hilt-android":       "com.google.dagger:hilt-android",
	}
	for k, v := range wantLibs {
		if got := md.LibsLibraries[k]; got != v {
			t.Errorf("LibsLibraries[%q] = %q, want %q", k, got, v)
		}
	}
}

func TestDiscover_NoManifest(t *testing.T) {
	root := t.TempDir()
	_, err := Discover(root)
	if err == nil {
		t.Fatal("expected error for missing manifest, got nil")
	}
}

func TestDiscover_EmptyRoot(t *testing.T) {
	_, err := Discover("")
	if err == nil {
		t.Fatal("expected error for empty root, got nil")
	}
}

func TestDiscover_MultipleManifestsInFallback(t *testing.T) {
	// Multi-módulo sin directorio `app/`: dos manifests
	// arbitrarios. El fallback `**/AndroidManifest.xml` debe detectar
	// la ambigüedad y devolver error en vez de elegir uno al azar.
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "featureA/src/main/AndroidManifest.xml"), classicManifest)
	writeFile(t, filepath.Join(root, "featureB/src/main/AndroidManifest.xml"), classicManifest)
	_, err := Discover(root)
	if err == nil {
		t.Fatal("expected error for multi-module project without app/, got nil")
	}
}

func TestDiscover_NoLibsVersionsTomlIsOK(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "app/src/main/AndroidManifest.xml"), classicManifest)

	md, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if md.LibsVersionsPath != "" {
		t.Errorf("expected empty LibsVersionsPath, got %q", md.LibsVersionsPath)
	}
	if len(md.LibsVersions) != 0 {
		t.Errorf("expected empty LibsVersions, got %v", md.LibsVersions)
	}
}
