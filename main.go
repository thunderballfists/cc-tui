package main

import (
	"cc-tui/cmd"
	"fmt"
	"os"
)

const Version = "0.2.0"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "serve":
			cmd.RunServe()
			return
		case "--version", "-v":
			fmt.Println("cc-tui " + Version)
			return
		}
	}
	cmd.RunClient()
}
