package index

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Walker struct {
	rootDir    string
	gitignore  map[string]bool
	extensions map[string]bool
}

func NewWalker(rootDir string) *Walker {
	return &Walker{
		rootDir: rootDir,
		gitignore: make(map[string]bool),
		extensions: map[string]bool{
			".kt":  true,
			".kts": true,
		},
	}
}

func (w *Walker) LoadGitignore() error {
	gitignorePath := filepath.Join(w.rootDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		return fmt.Errorf("error reading .gitignore: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			w.gitignore[line] = true
		}
	}

	return nil
}

func (w *Walker) shouldIgnore(path string) bool {
	relPath, err := filepath.Rel(w.rootDir, path)
	if err != nil {
		return false
	}

	// Check if path matches any gitignore pattern
	for pattern := range w.gitignore {
		if strings.Contains(relPath, pattern) {
			return true
		}
	}

	return false
}

func (w *Walker) isKotlinFile(path string) bool {
	ext := filepath.Ext(path)
	return w.extensions[ext]
}

func (w *Walker) CalculateHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("error calculating hash: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

type File struct {
	Path     string
	Package  string
	Module   string
	Layer    string
	Hash     string
}

func (w *Walker) Walk() ([]File, error) {
	var files []File

	err := filepath.Walk(w.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip ignored files
		if w.shouldIgnore(path) {
			return nil
		}

		// Only process Kotlin files
		if !w.isKotlinFile(path) {
			return nil
		}

		hash, err := w.CalculateHash(path)
		if err != nil {
			return fmt.Errorf("error calculating hash for %s: %w", path, err)
		}

		files = append(files, File{
			Path: path,
			Hash: hash,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	return files, nil
}
