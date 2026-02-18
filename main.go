package main

import (
	"fmt"
	"os"

	"github.com/AlhasanIQ/consult-human/cmd"
)

func main() {
	if err := cmd.Execute(os.Args[1:], cmd.IO{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
