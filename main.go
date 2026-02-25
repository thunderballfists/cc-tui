package main

import (
	"cc-tui/cmd"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		cmd.RunServe()
		return
	}
	cmd.RunClient()
}
