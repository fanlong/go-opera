package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/Fantom-foundation/go-opera/cmd/opera/launcher"
)

func main() {
	runtime.GOMAXPROCS(100)
	if err := launcher.Launch(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
