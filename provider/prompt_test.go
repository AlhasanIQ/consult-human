package provider

import (
	"strings"
	"testing"

	"github.com/AlhasanIQ/consult-human/contract"
)

func TestRenderTelegramPromptOpen(t *testing.T) {
	req := contract.AskRequest{
		RequestID: "req-123",
		Question:  "Should I ship this now?",
		Type:      contract.QuestionTypeOpen,
	}

	got := RenderTelegramPrompt(req)
	if got != "Should I ship this now?" {
		t.Fatalf("unexpected prompt: %q", got)
	}
	if strings.Contains(got, "Request ID") || strings.Contains(got, "consult-human request") {
		t.Fatalf("prompt should not include request metadata, got: %q", got)
	}
}

func TestRenderTelegramPromptChoice(t *testing.T) {
	req := contract.AskRequest{
		RequestID: "req-xyz",
		Question:  "Pick one",
		Type:      contract.QuestionTypeChoice,
		Choices: []contract.Choice{
			{ID: "A", Text: "Ship"},
			{ID: "B", Text: "Wait"},
		},
		AllowOther: true,
	}

	got := RenderTelegramPrompt(req)
	if !strings.Contains(got, "Pick one") {
		t.Fatalf("missing question in prompt: %q", got)
	}
	if !strings.Contains(got, "A) Ship") || !strings.Contains(got, "B) Wait") {
		t.Fatalf("missing choices in prompt: %q", got)
	}
	if !strings.Contains(got, "other) write your own answer") {
		t.Fatalf("missing other option in prompt: %q", got)
	}
	if strings.Contains(got, "Request ID") || strings.Contains(got, "consult-human request") {
		t.Fatalf("prompt should not include request metadata, got: %q", got)
	}
}
