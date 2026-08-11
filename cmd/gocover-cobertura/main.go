package main

import (
	"os"

	cobertura "github.com/blackbirdworks/gocover-cobertura"
)

func main() {
	cobertura.Convert(os.Stdin, os.Stdout)
}
