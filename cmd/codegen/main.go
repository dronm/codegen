package main

import (
	"fmt"
	"os"

	"github.com/dronm/codegen/internal/codegen"
)

func main() {
	if err := codegen.Run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "codegen: %v\n", err)
		os.Exit(1)
	}
}
