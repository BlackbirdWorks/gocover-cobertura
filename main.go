package main

import "os"

func main() {
	if code := runMain(os.Args[1:]); code != 0 {
		os.Exit(code)
	}
}

func runMain(args []string) int {
	err := RunCLI(args)
	if err != nil {
		return 1
	}

	return 0
}
