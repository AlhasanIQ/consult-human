package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AlhasanIQ/consult-human/config"
	"github.com/AlhasanIQ/consult-human/contract"
)

type telegramAPIMock struct {
	mu         sync.Mutex
	batches    [][]telegramUpdate
	getIndex   int
	sendCount  int
	sendTexts  []string
	nextMsgID  int64
	statusCode int
}

func newTelegramAPIMock() *telegramAPIMock {
	return &telegramAPIMock{
		nextMsgID:  1000,
		statusCode: http.StatusOK,
	}
}

func (m *telegramAPIMock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/sendMessage":
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)

		m.mu.Lock()
		m.sendCount++
		if text, ok := payload["text"].(string); ok {
			m.sendTexts = append(m.sendTexts, text)
		}
		m.nextMsgID++
		msgID := m.nextMsgID
		status := m.statusCode
		m.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(telegramSendResponse{
			OK: true,
			Result: telegramMessage{
				MessageID: msgID,
			},
		})
	case "/getUpdates":
		m.mu.Lock()
		var batch []telegramUpdate
		if m.getIndex < len(m.batches) {
			batch = m.batches[m.getIndex]
			m.getIndex++
		}
		m.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(telegramGetUpdatesResponse{
			OK:     true,
			Result: batch,
		})
	default:
		http.NotFound(w, r)
	}
}

func (m *telegramAPIMock) sendMessageCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sendCount
}

func (m *telegramAPIMock) sentTexts() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.sendTexts))
	copy(out, m.sendTexts)
	return out
}

func TestTelegramReceiveUnknownRequestID(t *testing.T) {
	p := &TelegramProvider{
		pending: make(map[string]int64),
	}

	if _, err := p.Receive(context.Background(), "missing"); err == nil {
		t.Fatalf("expected error for unknown request id")
	}
}

func TestTelegramSendUsesMinimalPrompt(t *testing.T) {
	mock := newTelegramAPIMock()
	mock.batches = [][]telegramUpdate{{}}
	srv := httptest.NewServer(mock)
	defer srv.Close()

	p := &TelegramProvider{
		chatID:       777,
		pollInterval: 10 * time.Millisecond,
		baseURL:      srv.URL,
		client:       srv.Client(),
		pending:      make(map[string]int64),
	}

	req := contract.AskRequest{
		RequestID: "req-min",
		Question:  "test",
		Type:      contract.QuestionTypeOpen,
	}
	if _, err := p.Send(context.Background(), req); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	texts := mock.sentTexts()
	if len(texts) != 1 {
		t.Fatalf("expected one sent message, got %d", len(texts))
	}
	got := texts[0]
	if got != "test" {
		t.Fatalf("expected minimal prompt %q, got %q", "test", got)
	}
	if strings.Contains(got, "consult-human request") || strings.Contains(got, "Request ID:") || strings.Contains(got, "Telegram: reply directly to this message.") {
		t.Fatalf("unexpected metadata in telegram prompt: %q", got)
	}
}

func TestNewTelegramAllowsMissingChatID(t *testing.T) {
	cfg := config.Default()
	cfg.Telegram.BotToken = "test-token"
	cfg.Telegram.ChatID = 0

	if _, err := NewTelegram(cfg); err != nil {
		t.Fatalf("expected provider initialization without chat id, got %v", err)
	}
}

func TestNewTelegramMissingTokenShowsSetupInstructions(t *testing.T) {
	cfg := config.Default()
	cfg.Telegram.BotToken = ""

	_, err := NewTelegram(cfg)
	if err == nil {
		t.Fatalf("expected error for missing token")
	}
	msg := err.Error()
	if !strings.Contains(msg, "@BotFather") || !strings.Contains(msg, "telegram.bot_token") {
		t.Fatalf("expected setup instructions in error, got: %q", msg)
	}
}

func TestIsTelegramStartCommand(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{input: "/start", want: true},
		{input: " /start ", want: true},
		{input: "/start please", want: true},
		{input: "/start@my_bot", want: true},
		{input: "/start@my_bot hello", want: true},
		{input: "/stop", want: false},
		{input: "start", want: false},
	}

	for _, tc := range cases {
		got := isTelegramStartCommand(tc.input)
		if got != tc.want {
			t.Fatalf("input %q: want %v got %v", tc.input, tc.want, got)
		}
	}
}

func TestTelegramReceiveAllowsFallbackWhenSinglePending(t *testing.T) {
	mock := newTelegramAPIMock()
	mock.batches = [][]telegramUpdate{
		{
			{
				UpdateID: 1,
				Message: &telegramMessage{
					MessageID: 2001,
					Date:      time.Now().Unix(),
					Text:      "req-123",
					Chat:      telegramChat{ID: 777},
				},
			},
		},
	}
	srv := httptest.NewServer(mock)
	defer srv.Close()

	p := &TelegramProvider{
		chatID:       777,
		pollInterval: 10 * time.Millisecond,
		baseURL:      srv.URL,
		client:       srv.Client(),
		pending: map[string]int64{
			"req-123": 1111,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	reply, err := p.Receive(ctx, "req-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply.Text != "req-123" {
		t.Fatalf("unexpected reply text: %q", reply.Text)
	}
	if got := mock.sendMessageCount(); got != 0 {
		t.Fatalf("expected no reminder message when only one pending request, got %d", got)
	}
}

func TestTelegramReceiveSendsReminderWhenMultiplePendingAndNotReply(t *testing.T) {
	mock := newTelegramAPIMock()
	mock.batches = [][]telegramUpdate{
		{
			{
				UpdateID: 1,
				Message: &telegramMessage{
					MessageID: 2002,
					Date:      time.Now().Unix(),
					Text:      "I answered above",
					Chat:      telegramChat{ID: 888},
				},
			},
		},
	}
	srv := httptest.NewServer(mock)
	defer srv.Close()

	p := &TelegramProvider{
		chatID:       888,
		pollInterval: 10 * time.Millisecond,
		baseURL:      srv.URL,
		client:       srv.Client(),
		pending: map[string]int64{
			"req-a": 9001,
			"req-b": 9002,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	_, err := p.Receive(ctx, "req-a")
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
	if got := mock.sendMessageCount(); got != 1 {
		t.Fatalf("expected one reminder message, got %d", got)
	}
	texts := mock.sentTexts()
	if len(texts) == 0 || !strings.Contains(texts[0], "2 unanswered consult-human questions") {
		t.Fatalf("unexpected reminder text: %#v", texts)
	}
}

func TestTelegramReceiveReturnsReplyOnlyOnExactReply(t *testing.T) {
	mock := newTelegramAPIMock()
	mock.batches = [][]telegramUpdate{
		{
			{
				UpdateID: 1,
				Message: &telegramMessage{
					MessageID: 3001,
					Date:      time.Now().Unix(),
					Text:      "Ship it",
					Chat:      telegramChat{ID: 999},
					ReplyToMessage: &telegramMessage{
						MessageID: 7001,
					},
				},
			},
		},
	}
	srv := httptest.NewServer(mock)
	defer srv.Close()

	p := &TelegramProvider{
		chatID:       999,
		pollInterval: 10 * time.Millisecond,
		baseURL:      srv.URL,
		client:       srv.Client(),
		pending: map[string]int64{
			"req-z": 7001,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	reply, err := p.Receive(ctx, "req-z")
	if err != nil {
		t.Fatalf("Receive returned error: %v", err)
	}
	if reply.Text != "Ship it" {
		t.Fatalf("unexpected reply text: %q", reply.Text)
	}
}
