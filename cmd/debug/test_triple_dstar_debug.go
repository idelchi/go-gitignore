package main

import (
	"fmt"

	gitignore "github.com/idelchi/go-gitignore"
)

func main() {
	fmt.Println("=== DEBUGGING TRIPLE DOUBLE STAR PATTERN ===")
	
	// The failing case: Pattern [**/**/foo a/**/b/**/c **/**/** **/x**y/**]
	// Should match "anything/anywhere" but doesn't
	
	patterns := []string{
		"**/**/foo",
		"a/**/b/**/c", 
		"**/**/**",    // This should match "anything/anywhere"
		"**/x**y/**",
	}
	
	gi := gitignore.New(patterns)
	
	testPath := "anything/anywhere"
	result := gi.Ignored(testPath, false)
	
	fmt.Printf("Patterns: %v\n", patterns)
	fmt.Printf("Test path: '%s'\n", testPath)
	fmt.Printf("Result: ignored=%v (expected=true)\n", result)
	
	if !result {
		fmt.Println("\n❌ FAILED - Let's debug each pattern individually:")
		
		for i, pattern := range patterns {
			singleGi := gitignore.New([]string{pattern})
			singleResult := singleGi.Ignored(testPath, false)
			fmt.Printf("  Pattern %d '%s': ignored=%v\n", i+1, pattern, singleResult)
			
			if pattern == "**/**/**" {
				fmt.Println("    ^ This pattern should match any path with 2+ components")
				fmt.Println("    'anything/anywhere' has 2 components, should match!")
			}
		}
	}
	
	// Test some simpler cases to understand the issue
	fmt.Println("\n=== TESTING SIMPLER CASES ===")
	simpleCases := []struct {
		pattern string
		path    string
		desc    string
	}{
		{"**", "anything/anywhere", "double star should match any path"},
		{"**//**", "anything/anywhere", "double-double star should match"},
		{"**/**/**", "anything/anywhere", "triple double star should match"},
		{"**/**/**", "a/b/c", "triple double star vs 3 components"},
		{"**/**/**", "single", "triple double star vs single component"},
	}
	
	for _, tc := range simpleCases {
		gi := gitignore.New([]string{tc.pattern})
		result := gi.Ignored(tc.path, false)
		fmt.Printf("'%s' vs '%s': ignored=%v (%s)\n", tc.pattern, tc.path, result, tc.desc)
	}
}