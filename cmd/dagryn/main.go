package main

import (
	"os"

	"github.com/mujhtech/dagryn/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
