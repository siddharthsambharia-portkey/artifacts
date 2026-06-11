package main

import (
	"fmt"
	"os"

	"github.com/siddharthsambharia-portkey/artifacts/internal/cli"
)

var version = "0.1.0"

func main() {
	if err := cli.Execute(version); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
