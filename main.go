package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		fmt.Println("daemon mode (not yet implemented)")
		os.Exit(0)
	}
	fmt.Println("client mode (not yet implemented)")
}
