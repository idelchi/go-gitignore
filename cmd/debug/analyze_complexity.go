package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
)

func main() {
	// Parse gitignore.go
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "/home/user/ws/gitignore.go", nil, parser.ParseComments)
	if err != nil {
		panic(err)
	}

	// Count functions and their complexity
	functions := make(map[string]int)
	var totalLines int
	
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			start := fset.Position(x.Pos()).Line
			end := fset.Position(x.End()).Line
			lines := end - start + 1
			functions[x.Name.Name] = lines
			totalLines += lines
		}
		return true
	})

	// Sort by complexity
	type funcInfo struct {
		name  string
		lines int
	}
	var sorted []funcInfo
	for name, lines := range functions {
		sorted = append(sorted, funcInfo{name, lines})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].lines > sorted[j].lines
	})

	fmt.Println("=== Complexity Analysis ===")
	fmt.Printf("Total functions: %d\n", len(functions))
	fmt.Printf("Total function lines: %d\n\n", totalLines)
	
	fmt.Println("Functions by complexity (lines):")
	for _, f := range sorted {
		fmt.Printf("  %-35s %4d lines\n", f.name, f.lines)
	}
	
	// Identify potential simplifications
	fmt.Println("\n=== Simplification Opportunities ===")
	fmt.Println("1. Multiple escape processing functions could be unified")
	fmt.Println("2. Pattern matching has redundant logic between file/directory patterns")
	fmt.Println("3. Many small helper functions could be inlined or consolidated")
	fmt.Println("4. Sandwich pattern logic is duplicated in multiple places")
	fmt.Println("5. Character class escaping logic is overly complex")
}