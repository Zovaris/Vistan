package main

import (
	"os"

	"github.com/zovaris/vistan/internal/vistan"
)

func main() {
	os.Exit(vistan.Run(os.Args[1:]))
}
