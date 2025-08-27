# gitignore

[![GitHub release](https://img.shields.io/github/v/release/idelchi/go-gitignore)](https://github.com/idelchi/go-gitignore/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/idelchi/go-gitignore.svg)](https://pkg.go.dev/github.com/idelchi/go-gitignore)
[![Go Report Card](https://goreportcard.com/badge/github.com/idelchi/go-gitignore)](https://goreportcard.com/report/github.com/idelchi/go-gitignore)
[![Build Status](https://github.com/idelchi/go-gitignore/actions/workflows/github-actions.yml/badge.svg)](https://github.com/idelchi/go-gitignore/actions/workflows/github-actions.yml/badge.svg)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Git-compatible `.gitignore` pattern matching for Go.

Tested against [a large number of cases](./tests) validated with `git check-ignore` to ensure behavior matches Git exactly.

## Installation

```bash
go get github.com/idelchi/go-gitignore
```

## Usage

```go
package main

import (
    "fmt"

    gitignore "github.com/idelchi/go-gitignore"
)

func main() {
    // Pass patterns to construct the gitignorer
    gi := gitignore.New("*.log", "build/", "!important.log")

    // Pass a path as well as a boolean indicating if it's a directory or not
    fmt.Println(gi.Ignored("app.log", false))        // true
    fmt.Println(gi.Ignored("important.log", false))  // false
    fmt.Println(gi.Ignored("build/file.txt", false)) // true
}
```

## Contributing

Contributors are encouraged to add more test cases to prevent over-fitting in the algorithm.
