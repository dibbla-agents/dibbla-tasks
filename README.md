# dibbla-tasks

Go library for executing `dibbla-task.yaml` files: a portable, declarative format for setup pipelines, tool installation, and project automation.

Used by the [Dibbla desktop app](https://github.com/dibbla-agents/dibbla-desktop-app) and [Dibbla CLI](https://github.com/dibbla-agents/dibbla-cli) to run task files locally.

## Status

**Pre-1.0.** Any `0.x` release may contain breaking changes. Pin a specific version in your `go.mod`. The format spec (`SPEC.md`) is `version: "1"` and is stable; the Go API is what may change.

## Install

Requires Go 1.24+ (transitive `golang.org/x/sys` requirement).

```bash
go get github.com/dibbla-agents/dibbla-tasks
```

## Quickstart

```go
package main

import (
	"context"
	"log"
	"os"

	steprunner "github.com/dibbla-agents/dibbla-tasks"
	"github.com/dibbla-agents/dibbla-tasks/output"
)

func main() {
	ctx := context.Background()
	result, err := steprunner.Run(ctx, "./dibbla-task.yaml",
		steprunner.WithFormatter(output.NewGHActionsFormatter(os.Stdout)),
		steprunner.WithEnv(map[string]string{
			"DATABASE_URL": "postgres://localhost/mydb",
		}),
	)
	if err != nil {
		log.Fatal(err)
	}
	if !result.Success {
		os.Exit(1)
	}
}
```

## Task file format

See [SPEC.md](./SPEC.md) for the full `dibbla-task.yaml` format and the GitHub Actions-style output format.

## History

Extracted from [`dibbla-desktop-app/pkg/steprunner`](https://github.com/dibbla-agents/dibbla-desktop-app) where it originated.

## License

MIT — see [LICENSE](./LICENSE).
