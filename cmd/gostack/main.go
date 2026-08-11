package main

import (
	"fmt"
	"os"

	"github.com/gos0001/gostack/internal/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
