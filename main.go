package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/AlhasanIQ/consult-human/cmd"
)

//go:embed SKILL.md
var embeddedSkillDoc []byte

func main() {
	cmd.SetEmbeddedSkillTemplate(embeddedSkillDoc)
	if err := cmd.Execute(os.Args[1:], cmd.IO{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
