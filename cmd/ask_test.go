package cmd

import (
	"reflect"
	"testing"

	"github.com/AlhasanIQ/consult-human/contract"
)

func TestParseChoices(t *testing.T) {
	choices, err := parseChoices([]string{"A:Use shared package", "B:Inline", "Custom fallback"})
	if err != nil {
		t.Fatalf("parseChoices failed: %v", err)
	}
	if len(choices) != 3 {
		t.Fatalf("expected 3 choices, got %d", len(choices))
	}
	if choices[2].ID == "" || choices[2].Text != "Custom fallback" {
		t.Fatalf("unexpected third choice: %#v", choices[2])
	}
}

func TestClassifyChoiceReplyByID(t *testing.T) {
	req := contract.AskRequest{
		Type: contract.QuestionTypeChoice,
		Choices: []contract.Choice{
			{ID: "A", Text: "Shared"},
			{ID: "B", Text: "Inline"},
		},
	}

	selected, other := classifyChoiceReply(req, "B")
	if !reflect.DeepEqual(selected, []string{"B"}) {
		t.Fatalf("unexpected selected: %#v", selected)
	}
	if other != "" {
		t.Fatalf("unexpected other: %q", other)
	}
}

func TestClassifyChoiceReplyOther(t *testing.T) {
	req := contract.AskRequest{
		Type:       contract.QuestionTypeChoice,
		AllowOther: true,
		Choices: []contract.Choice{
			{ID: "A", Text: "Shared"},
			{ID: "B", Text: "Inline"},
		},
	}

	selected, other := classifyChoiceReply(req, "Let's do a third option")
	if len(selected) != 0 {
		t.Fatalf("unexpected selected: %#v", selected)
	}
	if other == "" {
		t.Fatalf("expected other text")
	}
}
