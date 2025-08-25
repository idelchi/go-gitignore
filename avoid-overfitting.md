## Comprehensive Instructions for Removing Overfitting

### Core Problem Analysis

The current implementation is overfitted because it tries to **classify patterns into specific types** and handle each type differently. This leads to:
1. Special cases for specific patterns (like ".")
2. Inconsistent behavior between similar patterns
3. Arbitrary restrictions (like rejecting wildcards in sandwich patterns)
4. Complex, test-driven logic

### General Approach: Simplify and Unify

The solution is to **stop classifying patterns** and instead use a **unified matching algorithm** that relies on Git's actual rules:

## Step-by-Step Refactoring Guide

### 1. **Remove Pattern Classification**

**Current Problem:** Too many pattern "forms" (`formSandwich`, `formContentsOnly`, etc.)

**Solution:** Keep only essential pattern attributes:
```go
type pattern struct {
    original string
    pattern  string
    negated  bool
    dirOnly  bool
    rooted   bool
}
```

Remove all derived forms. They're attempting to pre-classify patterns, which leads to overfitting.

### 2. **Implement Uniform Wildcard Handling**

**Current Problem:** Different behavior for `/*` vs `*` with dotfiles

**Solution:** Git's actual rules are:
- `*` matches everything including dotfiles (except `.` and `..` as special directory entries)
- The distinction is NOT about dotfiles, but about path segments

```go
// Correct approach: Let the glob library handle it
// Only special case: . and .. as directory references
if pattern == "*" && (targetPath == "." || targetPath == "..") {
    // These are special directory references, not regular paths
    return false
}
// Otherwise, * matches everything including dotfiles
```

### 3. **Simplify Escape Processing**

**Current Problem:** Complex escape handling with special cases for character classes

**Solution:** Process escapes in one pass during parsing:
```go
func processPatternEscapes(pattern string) string {
    // Single, consistent escape processing:
    // 1. \# and \! at start become literals
    // 2. Trailing spaces handling
    // 3. Let the glob library handle the rest

    // Don't try to be clever about character classes
    // The glob library knows how to handle them
}
```

### 4. **Use Direct Glob Matching**

**Current Problem:** Too much pre-processing and pattern manipulation

**Solution:** Trust the glob library more:
```go
func matches(pat pattern, path string, isDir bool) bool {
    // Step 1: Handle directory-only patterns
    if pat.dirOnly && !isDir {
        return false
    }

    // Step 2: Prepare the pattern
    glob := pat.pattern
    if pat.dirOnly {
        // For dir-only, we already checked isDir above
        // Remove trailing / for matching
        glob = strings.TrimSuffix(glob, "/")
    }

    // Step 3: Determine what to match against
    var matchPath string
    if pat.rooted || strings.Contains(glob, "/") {
        // Anchored pattern - match full path
        matchPath = path
    } else {
        // Unanchored pattern - match basename only
        matchPath = filepath.Base(path)
    }

    // Step 4: Let doublestar handle the actual matching
    matched, _ := doublestar.Match(glob, matchPath)
    return matched
}
```

### 5. **Implement Contents-Only Semantics Correctly**

**Current Problem:** Complex logic for `/**` patterns

**Solution:** Simple rule - if pattern ends with `/**`, it matches contents but not the base:
```go
func matchesContentsOnly(pattern, path string) bool {
    if !strings.HasSuffix(pattern, "/**") {
        return false
    }

    base := strings.TrimSuffix(pattern, "/**")

    // Check if path is the base itself
    if matched, _ := doublestar.Match(base, path); matched {
        return false // Don't match the base
    }

    // Check if path is under the base
    return doublestar.Match(pattern, path)
}
```

### 6. **Remove Special Cases**

**Current Problem:** Special case for "." pattern

**Solution:** Remove it. If tests fail, the issue is likely elsewhere in the logic. The general algorithm should handle it.

### 7. **Simplify Parent Exclusion**

**Current Problem:** Complex two-pass algorithm

**Solution:** Simpler approach:
```go
func (g *GitIgnore) Ignored(p string, isDir bool) bool {
    // Normalize path once
    p = normalizePath(p)

    // Check each pattern in order
    ignored := false
    for _, pat := range g.patterns {
        if matches(pat, p, isDir) {
            if pat.negated {
                // Can only un-ignore if no parent is excluded
                if !hasExcludedParent(g, p) {
                    ignored = false
                }
            } else {
                ignored = true
            }
        }
    }

    return ignored
}

func hasExcludedParent(g *GitIgnore, path string) bool {
    parts := strings.Split(path, "/")
    for i := 1; i < len(parts); i++ {
        parent := strings.Join(parts[:i], "/")
        if g.isPathExcluded(parent, true) {
            return true
        }
    }
    return false
}
```

### 8. **Testing Strategy**

Instead of making the code pass tests by adding special cases:

1. **Start with the simplest implementation**
2. **Run tests to identify failures**
3. **For each failure, ask:** "What's the general Git rule here?"
4. **Implement the general rule, not a special case**
5. **If a test seems wrong, verify against actual Git behavior**

### 9. **Key Principles to Follow**

1. **Consistency:** Same rules apply everywhere (no special cases for specific patterns)
2. **Simplicity:** Fewer pattern classifications, simpler logic
3. **Trust the glob library:** Don't over-process patterns
4. **Follow Git's rules:** Not what we think the rules should be

### Example: Refactored Core Matching Function

```go
func matches(pat pattern, p string, isDir bool) bool {
    // Handle directory-only patterns
    if pat.dirOnly {
        if !isDir {
            return false
        }
        // Remove trailing slash for matching
        p = strings.TrimSuffix(p, "/")
    }

    glob := pat.pattern

    // Handle ** patterns ending with /
    if strings.HasSuffix(glob, "/**/") && isDir {
        // This matches directories under the base, not the base itself
        base := strings.TrimSuffix(glob, "/**/")
        if base != "" && glob != "**/" {
            // Check if this IS the base
            if matched, _ := doublestar.Match(base, p); matched {
                return false
            }
        }
    }

    // Handle ** patterns ending without /
    if strings.HasSuffix(glob, "/**") && !strings.HasSuffix(glob, "/**/") {
        // Contents-only pattern
        base := strings.TrimSuffix(glob, "/**")
        if base != "" {
            // Check if this IS the base
            if matched, _ := doublestar.Match(base, p); matched {
                return false
            }
        }
    }

    // Determine what to match against
    if pat.rooted || strings.Contains(glob, "/") {
        // Anchored - match full path
        matched, _ := doublestar.Match(glob, p)
        return matched
    } else {
        // Unanchored - match basename
        basename := path.Base(p)
        matched, _ := doublestar.Match(glob, basename)
        return matched
    }
}
```

### Summary

The key to removing overfitting is to:
1. **Simplify the pattern model** - Remove complex classifications
2. **Use consistent rules** - No special cases for specific patterns
3. **Trust the glob library** - Don't over-process
4. **Implement general Git rules** - Not test-specific hacks
5. **Verify against real Git** - When in doubt, test with actual Git

The result should be a simpler, more maintainable implementation that handles edge cases through general rules rather than special cases.
