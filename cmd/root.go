package cmd

import (
	"fmt"
	"io"
	"strings"
)

type IO struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}

func Execute(args []string, io IO) error {
	if io.In == nil || io.Out == nil || io.ErrOut == nil {
		return fmt.Errorf("invalid IO")
	}

	if len(args) == 0 {
		printRootUsage(io.ErrOut)
		return fmt.Errorf("missing command")
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "ask":
		return runAsk(args[1:], io)
	case "config":
		return runConfig(args[1:], io)
	case "setup":
		return runSetup(args[1:], io)
	case "help", "--help", "-h":
		printRootUsage(io.Out)
		return nil
	default:
		printRootUsage(io.ErrOut)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printRootUsage(w io.Writer) {
	fmt.Fprintln(w, "consult-human: relay AI-agent questions to a human via messaging apps")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  consult-human ask [flags] <question>")
	fmt.Fprintln(w, "  consult-human config <path|show|init|set|reset>")
	fmt.Fprintln(w, "  consult-human setup [flags]")
}
