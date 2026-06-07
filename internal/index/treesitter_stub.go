//go:build no_treesitter
// +build no_treesitter

package index

import (
	"strings"
)

type TreeSitterExtractor struct{}

func NewTreeSitterExtractor() *TreeSitterExtractor {
	return &TreeSitterExtractor{}
}

func (e *TreeSitterExtractor) ExtractSymbols(filePath string, content string) []Symbol {
	return nil
}

func (e *TreeSitterExtractor) InferPackage(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			return strings.TrimPrefix(line, "package ")
		}
	}
	return ""
}

func (e *TreeSitterExtractor) InferModule(filePath string) string {
	parts := strings.Split(filePath, "/")
	for i, part := range parts {
		if part == "src" && i > 0 {
			return parts[i-1]
		}
	}
	return "app"
}

func (e *TreeSitterExtractor) InferLayer(filePath string, content string) string {
	lowerPath := strings.ToLower(filePath)
	lowerContent := strings.ToLower(content)

	if strings.Contains(lowerPath, "di") || strings.Contains(lowerContent, "@module") {
		return "di"
	}
	if strings.Contains(lowerPath, "ui") || strings.Contains(lowerPath, "screen") ||
		strings.Contains(lowerContent, "@composable") {
		return "ui"
	}
	if strings.Contains(lowerPath, "viewmodel") || strings.Contains(lowerContent, "viewmodel") {
		return "domain"
	}
	if strings.Contains(lowerPath, "usecase") || strings.Contains(lowerContent, "usecase") {
		return "domain"
	}
	if strings.Contains(lowerPath, "repository") || strings.Contains(lowerPath, "data") {
		return "data"
	}
	if strings.Contains(lowerPath, "nav") || strings.Contains(lowerContent, "navhost") {
		return "nav"
	}
	if strings.Contains(lowerPath, "test") {
		return "test"
	}

	return "ui"
}

func (e *TreeSitterExtractor) ExtractFeature(symbols []Symbol) string {
	for _, sym := range symbols {
		if sym.Kind == "screen" || sym.Kind == "composable" {
			name := sym.Name
			if strings.HasSuffix(name, "Screen") {
				name = strings.TrimSuffix(name, "Screen")
			}
			if strings.HasSuffix(name, "Composable") {
				name = strings.TrimSuffix(name, "Composable")
			}
			return strings.ToLower(name)
		}
	}
	return ""
}
