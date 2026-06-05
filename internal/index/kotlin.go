package index

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

type Symbol struct {
	Name      string
	Kind      string
	Signature string
	Line      int
	Feature   string
}

type SymbolExtractor interface {
	ExtractSymbols(filePath string, content string) []Symbol
	InferPackage(content string) string
	InferModule(filePath string) string
	InferLayer(filePath string, content string) string
}

type KotlinExtractor struct {
	// Regex patterns for different symbol types
	composablePattern *regexp.Regexp
	viewModelPattern  *regexp.Regexp
	useCasePattern    *regexp.Regexp
	repositoryPattern *regexp.Regexp
	daoPattern        *regexp.Regexp
	modulePattern     *regexp.Regexp
	routePattern      *regexp.Regexp
	screenPattern     *regexp.Regexp
}

func NewKotlinExtractor() *KotlinExtractor {
	return &KotlinExtractor{
		composablePattern: regexp.MustCompile(`@Composable\s+fun\s+(\w+)`),
		viewModelPattern:  regexp.MustCompile(`(?:class|interface)\s+(\w+(?:ViewModel))\s*(?::\s*ViewModel)?`),
		useCasePattern:    regexp.MustCompile(`(?:class|interface)\s+(\w+(?:UseCase))\s*(?::\s*\w+)?`),
		repositoryPattern: regexp.MustCompile(`(?:class|interface)\s+(\w+(?:Repository(?:Impl)?))\s*(?::\s*\w+)?`),
		daoPattern:        regexp.MustCompile(`(?:class|interface)\s+(\w+(?:Dao))\s*(?::\s*\w+)?`),
		modulePattern:     regexp.MustCompile(`@Module\s+@InstallIn\((\w+)\)\s+class\s+(\w+)`),
		routePattern:      regexp.MustCompile(`composable\("(\w+)"\)`),
		screenPattern:     regexp.MustCompile(`(?:class|fun)\s+(\w+(?:Screen))\s*(?::\s*\w+)?`),
	}
}

func (e *KotlinExtractor) ExtractSymbols(filePath string, content string) []Symbol {
	var symbols []Symbol
	lines := strings.Split(content, "\n")
	hasComposableAnnotation := false

	for i, text := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(text)

		// Track @Composable annotation on its own line
		if trimmed == "@Composable" {
			hasComposableAnnotation = true
			continue
		}

		// Check for @Composable functions (handles both inline and multi-line annotation)
		if hasComposableAnnotation || e.composablePattern.MatchString(text) {
			if funMatches := regexp.MustCompile(`fun\s+(\w+)`).FindStringSubmatch(text); funMatches != nil {
				symbols = append(symbols, Symbol{
					Name:      funMatches[1],
					Kind:      "composable",
					Signature: text,
					Line:      lineNum,
				})
				hasComposableAnnotation = false
				continue
			}
		}
		hasComposableAnnotation = false

		// Check for ViewModel
		if matches := e.viewModelPattern.FindStringSubmatch(text); matches != nil {
			symbols = append(symbols, Symbol{
				Name:      matches[1],
				Kind:      "viewmodel",
				Signature: text,
				Line:      lineNum,
			})
		}

		// Check for UseCase
		if matches := e.useCasePattern.FindStringSubmatch(text); matches != nil {
			symbols = append(symbols, Symbol{
				Name:      matches[1],
				Kind:      "usecase",
				Signature: text,
				Line:      lineNum,
			})
		}

		// Check for Repository
		if matches := e.repositoryPattern.FindStringSubmatch(text); matches != nil {
			symbols = append(symbols, Symbol{
				Name:      matches[1],
				Kind:      "repository",
				Signature: text,
				Line:      lineNum,
			})
		}

		// Check for DAO
		if matches := e.daoPattern.FindStringSubmatch(text); matches != nil {
			symbols = append(symbols, Symbol{
				Name:      matches[1],
				Kind:      "dao",
				Signature: text,
				Line:      lineNum,
			})
		}

		// Check for Module (multi-line: @Module @InstallIn(...) class ...)
		if trimmed == "@Module" || strings.HasPrefix(trimmed, "@Module ") {
			// Look ahead for class declaration
			for j := i + 1; j < len(lines) && j < i+5; j++ {
				if modMatches := e.modulePattern.FindStringSubmatch(lines[j]); modMatches != nil {
					symbols = append(symbols, Symbol{
						Name:      modMatches[2],
						Kind:      "module",
						Signature: strings.TrimSpace(lines[j]),
						Line:      j + 1,
					})
					break
				}
			}
		}

		// Check for Route
		if matches := e.routePattern.FindStringSubmatch(text); matches != nil {
			symbols = append(symbols, Symbol{
				Name:      matches[1],
				Kind:      "route",
				Signature: text,
				Line:      lineNum,
			})
		}

		// Check for Screen
		if matches := e.screenPattern.FindStringSubmatch(text); matches != nil {
			symbols = append(symbols, Symbol{
				Name:      matches[1],
				Kind:      "screen",
				Signature: text,
				Line:      lineNum,
			})
		}
	}

	return symbols
}

func (e *KotlinExtractor) InferPackage(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(text, "package ") {
			return strings.TrimPrefix(text, "package ")
		}
	}
	return ""
}

func (e *KotlinExtractor) InferModule(filePath string) string {
	// Try to extract module from path (e.g., app/src/main/java/com/example/...)
	parts := strings.Split(filePath, string(os.PathSeparator))
	for i, part := range parts {
		if part == "src" && i > 0 {
			return parts[i-1]
		}
	}
	return "app" // default module
}

func (e *KotlinExtractor) InferLayer(filePath string, content string) string {
	lowerPath := strings.ToLower(filePath)
	lowerContent := strings.ToLower(content)

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
	if strings.Contains(lowerPath, "di") || strings.Contains(lowerContent, "@module") {
		return "di"
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

	return "ui" // default layer
}

func (e *KotlinExtractor) ExtractFeature(symbols []Symbol) string {
	for _, sym := range symbols {
		if sym.Kind == "screen" || sym.Kind == "composable" {
			// Try to extract feature name from symbol name
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
