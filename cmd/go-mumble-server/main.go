package main

import (
	"fmt"
	"os"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("go-mumble-server %s (commit: %s, built: %s)\n", version, commit, buildTime)
		os.Exit(0)
	}

	fmt.Printf("go-mumble-server %s\n", version)
	fmt.Println("Not yet implemented. See docs/ for the design.")
}
