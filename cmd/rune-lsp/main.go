package main

import (
	"fmt"
	"os"

	"github.com/restartfu/rune/internal/rune_lsp"
)

func main() {
	if err := rune_lsp.Run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "runerlsp: %v\n", err)
		os.Exit(1)
	}
}
