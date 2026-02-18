package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/term"
)

const (
	ansiReset = "\033[0m"
	ansiBold  = "\033[1m"
	ansiDim   = "\033[2m"
	ansiRed   = "\033[31m"
	ansiGreen = "\033[32m"
	ansiCyan  = "\033[36m"
)

// sty wraps an io.Writer with optional ANSI color support.
type sty struct {
	w     io.Writer
	color bool
}

func newSty(w io.Writer) *sty {
	if os.Getenv("NO_COLOR") != "" {
		return &sty{w: w}
	}
	if os.Getenv("TERM") == "dumb" {
		return &sty{w: w}
	}
	if f, ok := w.(*os.File); ok {
		if term.IsTerminal(int(f.Fd())) {
			return &sty{w: w, color: true}
		}
	}
	return &sty{w: w}
}

func (s *sty) ansi(codes, text string) string {
	if !s.color {
		return text
	}
	return codes + text + ansiReset
}

func (s *sty) bold(t string) string      { return s.ansi(ansiBold, t) }
func (s *sty) dim(t string) string       { return s.ansi(ansiDim, t) }
func (s *sty) cyan(t string) string      { return s.ansi(ansiCyan, t) }
func (s *sty) red(t string) string       { return s.ansi(ansiRed, t) }
func (s *sty) boldCyan(t string) string  { return s.ansi(ansiBold+ansiCyan, t) }
func (s *sty) boldGreen(t string) string { return s.ansi(ansiBold+ansiGreen, t) }

// header prints a boxed title.
func (s *sty) header(title string) {
	w := utf8.RuneCountInString(title) + 4
	bar := strings.Repeat("─", w)
	padded := "  " + title + "  "
	fmt.Fprintln(s.w)
	fmt.Fprintf(s.w, "  %s\n", s.dim("┌"+bar+"┐"))
	fmt.Fprintf(s.w, "  %s%s%s\n", s.dim("│"), s.boldCyan(padded), s.dim("│"))
	fmt.Fprintf(s.w, "  %s\n", s.dim("└"+bar+"┘"))
	fmt.Fprintln(s.w)
}

// section prints a titled divider line.
func (s *sty) section(title string) {
	after := 40 - utf8.RuneCountInString(title)
	if after < 3 {
		after = 3
	}
	fmt.Fprintln(s.w)
	fmt.Fprintf(s.w, "  %s %s %s\n", s.dim("───"), s.bold(title), s.dim(strings.Repeat("─", after)))
	fmt.Fprintln(s.w)
}

// step prints a numbered instruction.
func (s *sty) step(n int, text string) {
	fmt.Fprintf(s.w, "    %s %s\n", s.dim(fmt.Sprintf("%d.", n)), text)
}

// success prints a green checkmark with message.
func (s *sty) success(text string) {
	fmt.Fprintf(s.w, "  %s %s\n", s.boldGreen("✓"), text)
}

// info prints an indented line.
func (s *sty) info(text string) {
	fmt.Fprintf(s.w, "    %s\n", text)
}

// errMsg prints a red X mark with message.
func (s *sty) errMsg(text string) {
	fmt.Fprintf(s.w, "  %s %s\n", s.red("✗"), text)
}

// choice prints a numbered menu item with optional dim note.
func (s *sty) choice(n int, label string, note string) {
	suffix := ""
	if note != "" {
		suffix = " " + s.dim(note)
	}
	fmt.Fprintf(s.w, "    %s %s%s\n", s.cyan(fmt.Sprintf("%d)", n)), label, suffix)
}

// clearUp moves the cursor up n lines and clears to end of screen (TTY only).
func (s *sty) clearUp(n int) {
	if !s.color || n <= 0 {
		return
	}
	fmt.Fprintf(s.w, "\033[%dA\033[J", n)
}

// promptLabel returns a styled prompt string for inline input.
func (s *sty) promptLabel(label string) string {
	return fmt.Sprintf("  %s %s", s.boldGreen("?"), s.bold(label))
}

// spinner displays an animated waiting indicator.
type spinner struct {
	s    *sty
	done chan struct{}
	wg   sync.WaitGroup
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (s *sty) startSpinner(msg string) *spinner {
	sp := &spinner{s: s, done: make(chan struct{})}
	if !s.color {
		fmt.Fprintf(s.w, "  %s\n", msg)
		return sp
	}
	sp.wg.Add(1)
	go func() {
		defer sp.wg.Done()
		i := 0
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-sp.done:
				fmt.Fprintf(s.w, "\r\033[2K")
				return
			case <-ticker.C:
				fmt.Fprintf(s.w, "\r  %s %s", s.cyan(spinnerFrames[i%len(spinnerFrames)]), msg)
				i++
			}
		}
	}()
	return sp
}

func (sp *spinner) stop() {
	select {
	case <-sp.done:
		return
	default:
		close(sp.done)
	}
	sp.wg.Wait()
}
