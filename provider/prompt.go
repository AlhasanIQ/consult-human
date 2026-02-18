package provider

import (
	"fmt"
	"strings"

	"github.com/AlhasanIQ/consult-human/contract"
)

func RenderTelegramPrompt(req contract.AskRequest) string {
	var b strings.Builder

	question := strings.TrimSpace(req.Question)
	if question != "" {
		b.WriteString(question)
	}

	if req.Type == contract.QuestionTypeChoice && len(req.Choices) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		for _, choice := range req.Choices {
			b.WriteString(fmt.Sprintf("%s) %s\n", choice.ID, choice.Text))
		}
		if req.AllowOther {
			b.WriteString("other) write your own answer\n")
		}
		b.WriteString("\nReply with option ID or text.")
	}

	return strings.TrimSpace(b.String())
}

func RenderPrompt(req contract.AskRequest) string {
	var b strings.Builder

	b.WriteString("consult-human request\n")
	b.WriteString(fmt.Sprintf("Request ID: %s\n\n", req.RequestID))
	b.WriteString(req.Question)
	b.WriteString("\n\n")

	if req.Type == contract.QuestionTypeChoice && len(req.Choices) > 0 {
		b.WriteString("Options:\n")
		for _, choice := range req.Choices {
			b.WriteString(fmt.Sprintf("- %s) %s\n", choice.ID, choice.Text))
		}
		if req.AllowOther {
			b.WriteString("- other) reply with your own text\n")
		}
		b.WriteString("\nReply with the option ID, or write the full answer.\n")
	} else {
		b.WriteString("Reply with your answer as plain text.\n")
	}

	b.WriteString(fmt.Sprintf("If needed, include Request ID %s in your reply.\n", req.RequestID))
	return b.String()
}
