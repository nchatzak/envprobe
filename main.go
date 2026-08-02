package main

import (
	"os"

	"github.com/nchatzak/envprobe/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
