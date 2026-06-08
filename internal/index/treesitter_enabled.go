//go:build !no_treesitter
// +build !no_treesitter

package index

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/kotlin"
)

type TreeSitterExtractor struct {
	parser *sitter.Parser
	lang   *sitter.Language
}

func NewTreeSitterExtractor() *TreeSitterExtractor {
	parser := sitter.NewParser()
	lang := kotlin.GetLanguage()
	parser.SetLanguage(lang)

	return &TreeSitterExtractor{
		parser: parser,
		lang:   lang,
	}
}

func (e *TreeSitterExtractor) ExtractSymbols(filePath string, content string) []Symbol {
	var symbols []Symbol

	tree, err := e.parser.ParseCtx(context.Background(), nil, []byte(content))
	if err != nil {
		fmt.Printf("Warning: Error parsing %s: %v\n", filePath, err)
		return symbols
	}
	defer tree.Close()

	root := tree.RootNode()
	e.walkNode(root, content, &symbols)

	return symbols
}

func (e *TreeSitterExtractor) walkNode(node *sitter.Node, content string, symbols *[]Symbol) {
	nodeType := node.Type()

	switch nodeType {
	case "function_declaration":
		e.extractFunction(node, content, symbols)
	case "class_declaration":
		e.extractClass(node, content, symbols)
	case "annotation":
		e.extractAnnotation(node, content, symbols)
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		e.walkNode(child, content, symbols)
	}
}

func (e *TreeSitterExtractor) extractFunction(node *sitter.Node, content string, symbols *[]Symbol) {
	nameNode := e.getChildByType(node, "simple_identifier")
	if nameNode == nil {
		return
	}

	name := e.getNodeText(nameNode, content)
	signature := e.getNodeText(node, content)
	line := int(node.StartPoint().Row) + 1

	kind := "function"
	annotations := e.getAnnotations(node, content)

	for _, ann := range annotations {
		if ann == "Composable" {
			kind = "composable"
			break
		}
	}

	*symbols = append(*symbols, Symbol{
		Name:      name,
		Kind:      kind,
		Signature: signature,
		Line:      line,
	})
}

func (e *TreeSitterExtractor) extractClass(node *sitter.Node, content string, symbols *[]Symbol) {
	nameNode := e.getChildByType(node, "type_identifier")
	if nameNode == nil {
		nameNode = e.getChildByType(node, "simple_identifier")
	}
	if nameNode == nil {
		return
	}

	name := e.getNodeText(nameNode, content)
	signature := e.getNodeText(node, content)
	line := int(node.StartPoint().Row) + 1

	kind := "class"

	annotations := e.getAnnotations(node, content)
	for _, ann := range annotations {
		if ann == "HiltViewModel" {
			kind = "viewmodel"
			break
		}
		if ann == "Module" {
			kind = "module"
			break
		}
	}

	if strings.HasSuffix(name, "ViewModel") {
		kind = "viewmodel"
	} else if strings.HasSuffix(name, "UseCase") {
		kind = "usecase"
	} else if strings.HasSuffix(name, "Repository") || strings.HasSuffix(name, "RepositoryImpl") {
		kind = "repository"
	} else if strings.HasSuffix(name, "Dao") {
		kind = "dao"
	} else if strings.HasSuffix(name, "Screen") {
		kind = "screen"
	} else if strings.HasSuffix(name, "Fragment") {
		kind = "fragment"
	} else if strings.HasSuffix(name, "Activity") {
		kind = "activity"
	}

	*symbols = append(*symbols, Symbol{
		Name:      name,
		Kind:      kind,
		Signature: signature,
		Line:      line,
	})
}

func (e *TreeSitterExtractor) extractAnnotation(node *sitter.Node, content string, symbols *[]Symbol) {
}

func (e *TreeSitterExtractor) getAnnotations(node *sitter.Node, content string) []string {
	var annotations []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "modifiers" {
			for j := 0; j < int(child.ChildCount()); j++ {
				annNode := child.Child(j)
				if annNode.Type() == "annotation" {
					annName := e.getChildByType(annNode, "user_type")
					if annName != nil {
						typeId := e.getChildByType(annName, "type_identifier")
						if typeId != nil {
							annotations = append(annotations, e.getNodeText(typeId, content))
						}
					}
					constructorInv := e.getChildByType(annNode, "constructor_invocation")
					if constructorInv != nil {
						userType := e.getChildByType(constructorInv, "user_type")
						if userType != nil {
							typeId := e.getChildByType(userType, "type_identifier")
							if typeId != nil {
								annotations = append(annotations, e.getNodeText(typeId, content))
							}
						}
					}
				}
			}
		}
	}

	return annotations
}

func (e *TreeSitterExtractor) getChildByType(node *sitter.Node, nodeType string) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == nodeType {
			return child
		}
	}
	return nil
}

func (e *TreeSitterExtractor) getNodeText(node *sitter.Node, content string) string {
	start := int(node.StartByte())
	end := int(node.EndByte())
	if start >= len(content) || end > len(content) {
		return ""
	}
	return content[start:end]
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
	// Primero intenta detectar por prefijo común en ViewModel/UseCase/Repository
	for _, sym := range symbols {
		if sym.Kind == "viewmodel" || sym.Kind == "usecase" || sym.Kind == "repository" {
			name := sym.Name
			// Buscar prefijo común: CounterViewModel -> counter, LoginUseCase -> login
			for _, suffix := range []string{"ViewModel", "UseCase", "Repository", "RepositoryImpl", "Screen", "Composable", "Effect", "Event", "State", "Module", "Route"} {
				if strings.HasSuffix(name, suffix) {
					return strings.ToLower(strings.TrimSuffix(name, suffix))
				}
			}
		}
	}
	// Fallback: buscar en screen/composable
	for _, sym := range symbols {
		if sym.Kind == "screen" || sym.Kind == "composable" {
			name := sym.Name
			for _, suffix := range []string{"Screen", "Composable"} {
				if strings.HasSuffix(name, suffix) {
					return strings.ToLower(strings.TrimSuffix(name, suffix))
				}
			}
		}
	}
	return ""
}
