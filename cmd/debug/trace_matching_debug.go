package main

import (
	"fmt"
	"strings"
)

// Reproduce the pattern matching logic to debug the issue
func main() {
	fmt.Println("=== TRACING PATTERN MATCHING DEBUG ===")

	// Test pattern: "abc/**/"
	patternStr := "abc/**/"
	path := "abc"
	isDir := true

	fmt.Printf("Pattern: %s\n", patternStr)
	fmt.Printf("Path: %s\n", path)
	fmt.Printf("IsDir: %v\n", isDir)

	// Step 1: Parse pattern
	fmt.Println("\nStep 1: Pattern parsing...")
	dirOnly := strings.HasSuffix(patternStr, "/")
	if dirOnly {
		patternStr = strings.TrimSuffix(patternStr, "/")
	}
	fmt.Printf("  dirOnly: %v\n", dirOnly)
	fmt.Printf("  pattern after removing /: %s\n", patternStr)

	// Step 2: Check if it's isDirDescendants
	fmt.Println("\nStep 2: Check isDirDescendants...")
	isDirDesc := isDirDescendantsDebug(patternStr, dirOnly)
	fmt.Printf("  isDirDescendants: %v\n", isDirDesc)

	if isDirDesc {
		fmt.Println("\nStep 3: Processing dirDescendants logic...")

		// stripTrailingSuffix logic
		base := stripTrailingSuffixDebug(patternStr, false)
		fmt.Printf("  base after stripping: '%s'\n", base)

		// Check if base is meaningful
		meaningful := base != "" && !endsWithDoubleStarSegmentDebug(base)
		fmt.Printf("  base is meaningful: %v\n", meaningful)

		if meaningful {
			// Check if candidate equals base
			baseMatch := matchRawGlobDebug(base, path)
			fmt.Printf("  matchRawGlob('%s', '%s'): %v\n", base, path, baseMatch)

			if baseMatch {
				fmt.Println("  -> Should return FALSE (base match)")
			} else {
				// Check if it matches base/**
				descendantMatch := matchRawGlobDebug(base+"/**", path)
				fmt.Printf("  matchRawGlob('%s', '%s'): %v\n", base+"/**", path, descendantMatch)
				fmt.Printf("  -> Should return %v (descendant match)\n", descendantMatch)
			}
		}
	}
}

func isDirDescendantsDebug(pattern string, dirOnly bool) bool {
	if !dirOnly {
		return false
	}
	classifyNoRoot := strings.TrimPrefix(pattern, "/")
	if strings.HasSuffix(classifyNoRoot, "/**/") {
		base := strings.TrimSuffix(classifyNoRoot, "/**/")
		return base != ""
	}
	return false
}

func stripTrailingSuffixDebug(glob string, allowDoubleSlash bool) string {
	doubleStarSlash := "/**"
	for strings.HasSuffix(glob, doubleStarSlash) {
		if !allowDoubleSlash || !strings.HasSuffix(glob, "/**/") {
			glob = strings.TrimSuffix(glob, doubleStarSlash)
		} else {
			break
		}
	}
	return glob
}

func endsWithDoubleStarSegmentDebug(glob string) bool {
	doubleStar := "**"
	doubleStarSlash := "/**"
	if glob == doubleStar {
		return true
	}
	return strings.HasSuffix(glob, doubleStarSlash) && strings.TrimSuffix(glob, doubleStarSlash) != ""
}

func matchRawGlobDebug(pattern, path string) bool {
	fmt.Printf("    [GLOB] Matching '%s' against '%s'\n", pattern, path)
	// Simple approximation - in real code this uses doublestar
	if pattern == path {
		return true
	}
	if pattern == "*" && !strings.Contains(path, "/") {
		return true
	}
	return false
}
