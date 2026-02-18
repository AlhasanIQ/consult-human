package contract

import "time"

type QuestionType string

const (
	QuestionTypeOpen   QuestionType = "open"
	QuestionTypeChoice QuestionType = "choice"
)

type Choice struct {
	ID   string `json:"id" yaml:"id"`
	Text string `json:"text" yaml:"text"`
}

type AskRequest struct {
	RequestID  string       `json:"request_id"`
	Question   string       `json:"question"`
	Type       QuestionType `json:"type"`
	Choices    []Choice     `json:"choices,omitempty"`
	AllowOther bool         `json:"allow_other,omitempty"`
	SentAt     time.Time    `json:"sent_at"`
}

type Reply struct {
	RequestID         string    `json:"request_id"`
	Text              string    `json:"text"`
	From              string    `json:"from,omitempty"`
	ProviderMessageID string    `json:"provider_message_id,omitempty"`
	ReceivedAt        time.Time `json:"received_at"`
	Raw               string    `json:"raw,omitempty"`
}

type AskResult struct {
	RequestID    string       `json:"request_id"`
	Provider     string       `json:"provider"`
	QuestionType QuestionType `json:"question_type"`
	Text         string       `json:"text,omitempty"`
	SelectedIDs  []string     `json:"selected_ids,omitempty"`
	OtherText    string       `json:"other_text,omitempty"`
	RawReply     string       `json:"raw_reply,omitempty"`
	ReceivedAt   time.Time    `json:"received_at"`
}
