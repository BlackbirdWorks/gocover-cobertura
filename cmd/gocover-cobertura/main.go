package main

import "os"

func main() {
	if code := runMain(os.Args[1:]); code != 0 {
		os.Exit(code)
	}
}

func runMain(args []string) int {
	if err := RunCLI(args); err != nil {
		return 1
	}

	return 0
}
