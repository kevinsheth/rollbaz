package main

import (
	"os"

	"github.com/kevinsheth/rollbaz/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
