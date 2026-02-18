package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/AlhasanIQ/consult-human/config"
	"github.com/AlhasanIQ/consult-human/contract"
	"github.com/AlhasanIQ/consult-human/provider"
)

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSliceFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func runAsk(args []string, io IO) error {
	fs := flag.NewFlagSet("ask", flag.ContinueOnError)
	fs.SetOutput(io.ErrOut)

	var choicesRaw stringSliceFlag
	var allowOther bool
	var providerOverride string
	var timeoutOverride string

	fs.Var(&choicesRaw, "choice", "Choice in the form id:text or plain text. Repeatable.")
	fs.BoolVar(&allowOther, "allow-other", false, "Allow a free-text answer outside predefined choices")
	fs.StringVar(&providerOverride, "provider", "", "Override configured provider (telegram)")
	fs.StringVar(&timeoutOverride, "timeout", "", "Override configured timeout (e.g. 5m, 30s)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	question := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if question == "" {
		return fmt.Errorf("missing question")
	}

	choices, err := parseChoices(choicesRaw)
	if err != nil {
		return err
	}
	if len(choices) == 0 && allowOther {
		return fmt.Errorf("--allow-other requires at least one --choice")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	timeout, err := config.EffectiveTimeout(cfg)
	if err != nil {
		return err
	}
	if strings.TrimSpace(timeoutOverride) != "" {
		timeout, err = time.ParseDuration(strings.TrimSpace(timeoutOverride))
		if err != nil {
			return fmt.Errorf("invalid --timeout: %w", err)
		}
		if timeout <= 0 {
			return fmt.Errorf("--timeout must be > 0")
		}
	}

	reqID, err := newRequestID()
	if err != nil {
		return err
	}

	qType := contract.QuestionTypeOpen
	if len(choices) > 0 {
		qType = contract.QuestionTypeChoice
	}

	req := contract.AskRequest{
		RequestID:  reqID,
		Question:   question,
		Type:       qType,
		Choices:    choices,
		AllowOther: allowOther,
		SentAt:     time.Now().UTC(),
	}

	p, err := provider.New(cfg, providerOverride)
	if err != nil {
		return err
	}
	defer p.Close()

	fmt.Fprintf(io.ErrOut, "Sending request %s via %s...\n", req.RequestID, p.Name())

	baseCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	ctx, cancel := context.WithTimeout(baseCtx, timeout)
	defer cancel()

	if _, err := p.Send(ctx, req); err != nil {
		return err
	}

	fmt.Fprintln(io.ErrOut, "Waiting for human reply...")
	reply, err := p.Receive(ctx, req.RequestID)
	if err != nil {
		return err
	}

	result := contract.AskResult{
		RequestID:    req.RequestID,
		Provider:     p.Name(),
		QuestionType: req.Type,
		RawReply:     reply.Raw,
		ReceivedAt:   reply.ReceivedAt,
	}

	if req.Type == contract.QuestionTypeOpen {
		result.Text = strings.TrimSpace(reply.Text)
	} else {
		selected, other := classifyChoiceReply(req, reply.Text)
		result.SelectedIDs = selected
		result.OtherText = other
		result.Text = strings.TrimSpace(reply.Text)
	}

	enc := json.NewEncoder(io.Out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(result); err != nil {
		return err
	}

	return nil
}

func parseChoices(raw []string) ([]contract.Choice, error) {
	choices := make([]contract.Choice, 0, len(raw))
	seen := map[string]struct{}{}

	for i, item := range raw {
		rawItem := strings.TrimSpace(item)
		if rawItem == "" {
			continue
		}

		id := ""
		text := ""
		parts := strings.SplitN(rawItem, ":", 2)
		if len(parts) == 2 {
			id = normalizeChoiceID(parts[0])
			text = strings.TrimSpace(parts[1])
		} else {
			id = autoChoiceID(i)
			text = rawItem
		}

		if id == "" || text == "" {
			return nil, fmt.Errorf("invalid choice %q", item)
		}
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("duplicate choice id %q", id)
		}
		seen[id] = struct{}{}
		choices = append(choices, contract.Choice{ID: id, Text: text})
	}

	return choices, nil
}

func normalizeChoiceID(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	v = strings.ToUpper(v)
	v = strings.Trim(v, "()[]{}<>.")
	return v
}

func autoChoiceID(i int) string {
	if i < 26 {
		return string(rune('A' + i))
	}
	return fmt.Sprintf("C%d", i+1)
}

func classifyChoiceReply(req contract.AskRequest, raw string) ([]string, string) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, ""
	}

	byID := map[string]contract.Choice{}
	byText := map[string]string{}
	for _, c := range req.Choices {
		id := normalizeChoiceID(c.ID)
		byID[id] = c
		byText[strings.ToLower(strings.TrimSpace(c.Text))] = id
	}

	// If the reply is a sentence (space-separated, no explicit delimiters),
	// avoid falsely matching incidental tokens like "a" to choice "A".
	if !strings.ContainsAny(text, ",;\n") && strings.Contains(text, " ") {
		if id, ok := byText[strings.ToLower(strings.TrimSpace(text))]; ok {
			return []string{id}, ""
		}
		if req.AllowOther {
			trimmedLower := strings.ToLower(strings.TrimSpace(text))
			if strings.HasPrefix(trimmedLower, "other:") {
				return nil, strings.TrimSpace(text[len("other:"):])
			}
			return nil, text
		}
		return nil, ""
	}

	tokens := splitReplyTokens(text)
	selected := make([]string, 0, len(tokens))
	selectedSet := map[string]struct{}{}

	for _, token := range tokens {
		n := normalizeChoiceID(token)
		if _, ok := byID[n]; ok {
			if _, seen := selectedSet[n]; !seen {
				selectedSet[n] = struct{}{}
				selected = append(selected, n)
			}
			continue
		}

		if idx, err := strconv.Atoi(n); err == nil {
			if idx >= 1 && idx <= len(req.Choices) {
				id := normalizeChoiceID(req.Choices[idx-1].ID)
				if _, seen := selectedSet[id]; !seen {
					selectedSet[id] = struct{}{}
					selected = append(selected, id)
				}
			}
			continue
		}

		if id, ok := byText[strings.ToLower(strings.TrimSpace(token))]; ok {
			if _, seen := selectedSet[id]; !seen {
				selectedSet[id] = struct{}{}
				selected = append(selected, id)
			}
		}
	}

	slices.Sort(selected)
	if len(selected) > 0 {
		if strings.HasPrefix(strings.ToLower(text), "other:") {
			return selected, strings.TrimSpace(text[len("other:"):])
		}
		return selected, ""
	}

	if req.AllowOther {
		trimmed := strings.TrimSpace(text)
		trimmedLower := strings.ToLower(trimmed)
		if strings.HasPrefix(trimmedLower, "other:") {
			return nil, strings.TrimSpace(trimmed[len("other:"):])
		}
		if strings.EqualFold(trimmed, "other") {
			return nil, ""
		}
		return nil, trimmed
	}

	return nil, ""
}

func splitReplyTokens(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\t', ' ':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		trimmed := strings.TrimSpace(strings.Trim(f, "()[]{}<>."))
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func newRequestID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
