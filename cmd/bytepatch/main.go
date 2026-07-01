package main

import (
	"os"

	"github.com/EdgarOrtegaRamirez/bytepatch/internal/cli"
)

func main() {
	if err := cli.Run(os.Args); err != nil {
		os.Stderr.WriteString("Error: " + err.Error() + "\n")
		os.Exit(1)
	}
}
