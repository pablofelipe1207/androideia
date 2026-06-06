// Package project expone utilidades para descubrir metadatos del proyecto
// Android en el que se está ejecutando el agente. El objetivo principal es
// evitar que el agente invente nombres de paquete o de librería:
//
//   - applicationId / namespace reales extraídos del AndroidManifest.xml
//     (no de MainActivity.kt ni de conjeturas).
//   - Versiones y aliases que el proyecto YA tiene declarados en
//     gradle/libs.versions.toml, para reutilizarlos en lugar de inventar
//     coordenadas nuevas.
//
// Todas las funciones son puras y tolerantes a fallos: si el archivo no
// existe o está mal formado devuelven un error explícito en vez de
// adivinar, para que la persona operadora vea el problema en vez de un
// build silenciosamente mal nombrado.
package project

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Metadata agrupa la información relevante del proyecto que el agente
// necesita conocer antes de empezar a crear archivos.
type Metadata struct {
	// AppPath es el directorio raíz del proyecto Android (el que
	// contiene settings.gradle.kts / gradlew).
	AppPath string

	// ManifestPath es la ruta absoluta al AndroidManifest.xml que se
	// usó para extraer el applicationId / namespace.
	ManifestPath string

	// ApplicationID es el applicationId declarado en el manifest
	// (atributo "package" en AGP < 8, atributo "namespace" en AGP >= 8).
	ApplicationID string

	// ManifestActivities lista las activities declaradas en el
	// manifest (incluye su nombre completo, p. ej.
	// "com.example.myapplication.MainActivity"). Sirve para que el
	// agente no invente packages distintos.
	ManifestActivities []string

	// LibsVersionsPath es la ruta absoluta a gradle/libs.versions.toml
	// (o "" si no existe).
	LibsVersionsPath string

	// LibsVersions contiene los pares alias = "version" declarados en
	// el archivo toml. NO incluye el bloque [libraries] porque el
	// agente debe decidir la coordenada exacta que quiere usar.
	LibsVersions map[string]string

	// LibsLibraries contiene los pares alias = "group:artifact" del
	// bloque [libraries], para detectar conflictos antes de proponer
	// una coordenada nueva.
	LibsLibraries map[string]string
}

// Discover intenta localizar AndroidManifest.xml y libs.versions.toml a
// partir de un directorio raíz. Busca manifest en el camino "moderno"
// (app/src/main/AndroidManifest.xml) y en el legado, y acepta
// configuración multi-módulo si hay un solo módulo de aplicación.
func Discover(root string) (*Metadata, error) {
	if root == "" {
		return nil, errors.New("project root is empty")
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("project root not found: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("project root is not a directory: %s", root)
	}

	md := &Metadata{
		AppPath:         root,
		LibsVersions:    map[string]string{},
		LibsLibraries:   map[string]string{},
	}

	manifestPath, err := findManifest(root)
	if err != nil {
		return nil, err
	}
	md.ManifestPath = manifestPath
	md.ApplicationID, md.ManifestActivities, err = parseManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", manifestPath, err)
	}

	libsPath := findLibsVersionsToml(root)
	if libsPath != "" {
		md.LibsVersionsPath = libsPath
		versions, libraries, err := parseLibsVersionsToml(libsPath)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", libsPath, err)
		}
		md.LibsVersions = versions
		md.LibsLibraries = libraries
	}

	return md, nil
}

// findManifest busca app/src/main/AndroidManifest.xml. Si hay varios
// módulos con AndroidManifest.xml (multi-módulo) devuelve el del
// primer módulo de aplicación que encuentre y registra un warning
// en stderr — en ese caso el llamador puede decidir re-ejecutar
// pasándole el módulo que quiera.
func findManifest(root string) (string, error) {
	candidates, err := filepath.Glob(filepath.Join(root, "app", "src", "main", "AndroidManifest.xml"))
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		// fallback: cualquier AndroidManifest.xml bajo root.
		any, _ := filepath.Glob(filepath.Join(root, "**", "AndroidManifest.xml"))
		if len(any) == 0 {
			return "", fmt.Errorf("AndroidManifest.xml not found under %s; ¿es esto un proyecto Android?", root)
		}
		if len(any) > 1 {
			return "", fmt.Errorf("multiple AndroidManifest.xml found under %s; specify a single-module project or pass the module path explicitly", root)
		}
		return any[0], nil
	}
	if len(candidates) > 1 {
		return "", fmt.Errorf("multiple app/src/main/AndroidManifest.xml under %s; pass the specific module path", root)
	}
	return candidates[0], nil
}

// parseManifest extrae applicationId y las activities declaradas.
//
//   - En AGP < 8 el applicationId viene del atributo `package` del
//     elemento <manifest>.
//   - En AGP >= 8 el applicationId viene del bloque namespace del
//     build.gradle.kts; el manifest YA NO tiene `package`. Para esos
//     casos esta función devuelve applicationId = "" y el llamador
//     debe leer namespace del build.gradle.kts. (Esta función
//     también devuelve el namespace del build script si lo encuentra.)
func parseManifest(path string) (string, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	type manifestT struct {
		XMLName     xml.Name `xml:"manifest"`
		Package     string   `xml:"package,attr"`
		Application struct {
			Activities []struct {
				Name string `xml:"name,attr"`
			} `xml:"activity"`
		} `xml:"application"`
	}
	var m manifestT
	if err := xml.Unmarshal(data, &m); err != nil {
		return "", nil, fmt.Errorf("malformed XML: %w", err)
	}

	activities := make([]string, 0, len(m.Application.Activities))
	for _, a := range m.Application.Activities {
		name := strings.TrimSpace(a.Name)
		if name != "" {
			activities = append(activities, name)
		}
	}

	// Si el manifest no tiene `package` (AGP >= 8), intentamos
	// extraerlo del namespace declarado en build.gradle.kts del
	// módulo que contiene el manifest.
	appID := strings.TrimSpace(m.Package)
	if appID == "" {
		moduleDir := filepath.Dir(filepath.Dir(filepath.Dir(path))) // .../app/src/main → .../app
		if ns, err := readAndroidNamespace(moduleDir); err == nil && ns != "" {
			appID = ns
		}
	}
	return appID, activities, nil
}

var gradleNamespaceRe = regexp.MustCompile(`(?m)^\s*namespace\s*=\s*"([^"]+)"`)

func readAndroidNamespace(moduleDir string) (string, error) {
	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
		p := filepath.Join(moduleDir, name)
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		m := gradleNamespaceRe.FindStringSubmatch(string(data))
		if len(m) == 2 {
			return strings.TrimSpace(m[1]), nil
		}
	}
	return "", fmt.Errorf("namespace not found in %s build.gradle*", moduleDir)
}

func findLibsVersionsToml(root string) string {
	candidates := []string{
		filepath.Join(root, "gradle", "libs.versions.toml"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// [versions] alias = "version"
// [libraries] alias = { module = "group:artifact", ... }
var tomlVersionLineRe = regexp.MustCompile(`(?m)^\s*([A-Za-z0-9_\-.]+)\s*=\s*"([^"]+)"\s*$`)
var tomlLibraryLineRe = regexp.MustCompile(`(?m)^\s*([A-Za-z0-9_\-.]+)\s*=\s*\{\s*module\s*=\s*"([^"]+)"`)

func parseLibsVersionsToml(path string) (map[string]string, map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	text := string(data)

	versions := map[string]string{}
	libraries := map[string]string{}

	currentSection := ""
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		switch currentSection {
		case "versions":
			if m := tomlVersionLineRe.FindStringSubmatch(line); len(m) == 3 {
				versions[m[1]] = m[2]
			}
		case "libraries":
			if m := tomlLibraryLineRe.FindStringSubmatch(line); len(m) == 3 {
				libraries[m[1]] = m[2]
			}
		}
	}
	return versions, libraries, nil
}
