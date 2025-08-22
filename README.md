# gitignore

[![Go Reference](https://pkg.go.dev/badge/github.com/idelchi/go-gitignore.svg)](https://pkg.go.dev/github.com/idelchi/go-gitignore)

Package **gitignore** provides Git-compatible `.gitignore` pattern matching with close parity to Git's behavior.


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
	gi := gitignore.New([]string{"*.log", "build/", "!important.log"})

	fmt.Println(gi.IsIgnored("app.log", false))        // true
	fmt.Println(gi.IsIgnored("important.log", false))  // false
	fmt.Println(gi.IsIgnored("build/file.txt", false)) // true
}
```

## Testing

The package includes an extensive test suite verifying behavior across edge cases and Git's own specification:

```bash
go test ./...
```

## License

MIT
