package main

import (
	"os"

	"mutate4go"
)

func main() {
	workingDir, err := os.Getwd()
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
	exit := mutate4go.Run(os.Args[1:], workingDir, os.Stdout, os.Stderr)
	os.Exit(exit)
}
