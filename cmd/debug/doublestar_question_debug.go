package main

import (
	"fmt"
	"github.com/bmatcuk/doublestar/v4"
)

func main() {
	pattern := "*?/0"
	targets := []string{"0/0", "ab/0", "abc/0", "0/x/0"}
	
	fmt.Printf("Testing doublestar directly with pattern '%s':\n", pattern)
	
	for _, target := range targets {
		result := doublestar.MatchUnvalidated(pattern, target)
		fmt.Printf("  %s -> %v\n", target, result)
	}
}