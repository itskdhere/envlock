package main

import (
	"os"

	"github.com/itskdhere/envlock/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
