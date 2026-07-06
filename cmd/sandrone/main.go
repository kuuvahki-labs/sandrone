package main

import (
	"context"
	"os"

	"github.com/kuuvahki-labs/sandrone/internal/entry/cli"
)

func main() {
	os.Exit(cli.Execute(context.Background()))
}
