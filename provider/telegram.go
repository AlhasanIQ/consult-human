package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/AlhasanIQ/consult-human/config"
	"github.com/AlhasanIQ/consult-human/contract"
)

const telegramReplyReminderCooldown = 20 * time.Second

type TelegramProvider struct {
	chatID       int64
	pollInterval time.Duration
	baseURL      string
	client       *http.Client
	pendingStore *telegramPendingStore

	mu             sync.Mutex
	nextUpdateID   int64
	pending        map[string]int64
	lastReminderAt time.Time
}

func NewTelegram(cfg config.Config) (*TelegramProvider, error) {
	token := strings.TrimSpace(cfg.Telegram.BotToken)
	if token == "" {
		return nil, fmt.Errorf(
			"telegram.bot_token is required.\n" +
				"First-time Telegram setup:\n" +
				"1) Open Telegram and chat with @BotFather\n" +
				"2) Run /newbot and copy the bot token\n" +
				"3) Run: `consult-human config set telegram.bot_token \"<BOT_TOKEN>\"`\n" +
				"4) Run: `consult-human ask ...` then send /start to your bot to link chat",
		)
	}
	pendingStore, err := newTelegramPendingStore()
	if err != nil {
		return nil, err
	}

	pollSeconds := cfg.Telegram.PollIntervalSeconds
	if pollSeconds <= 0 {
		pollSeconds = 2
	}

	return &TelegramProvider{
		chatID:       cfg.Telegram.ChatID,
		pollInterval: time.Duration(pollSeconds) * time.Second,
		baseURL:      fmt.Sprintf("https://api.telegram.org/bot%s", token),
		client: &http.Client{
			Timeout: 45 * time.Second,
		},
		pending:      make(map[string]int64),
		pendingStore: pendingStore,
	}, nil
}

func (p *TelegramProvider) Name() string { return "telegram" }

func (p *TelegramProvider) Close() error { return nil }

func (p *TelegramProvider) Send(ctx context.Context, req contract.AskRequest) (string, error) {
	if err := p.ensureChatID(ctx); err != nil {
		return "", err
	}

	// Drain stale updates so Receive only sees replies after this point.
	if _, err := p.getUpdates(ctx); err != nil && ctx.Err() == nil {
		return "", err
	}

	chatID := p.chatIDValue()
	prompt := RenderTelegramPrompt(req)
	messageID, err := p.sendTelegramMessage(ctx, chatID, prompt)
	if err != nil {
		return "", err
	}

	if err := p.registerPending(req.RequestID, chatID, messageID); err != nil {
		return "", err
	}

	return req.RequestID, nil
}

func (p *TelegramProvider) Receive(ctx context.Context, requestID string) (contract.Reply, error) {
	chatID, targetMessageID, err := p.lookupPending(requestID)
	if err != nil {
		return contract.Reply{}, err
	}
	defer p.clearPending(requestID)

	for {
		select {
		case <-ctx.Done():
			return contract.Reply{}, ctx.Err()
		default:
		}

		updates, err := p.getUpdates(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return contract.Reply{}, ctx.Err()
			}
			return contract.Reply{}, err
		}

		for _, up := range updates {
			msg := up.Message
			if msg == nil {
				continue
			}
			if msg.Chat.ID != chatID {
				continue
			}

			text := strings.TrimSpace(msg.Text)
			if text == "" {
				continue
			}

			matchesByReply := msg.ReplyToMessage != nil && msg.ReplyToMessage.MessageID == targetMessageID
			if !matchesByReply {
				pendingCount := p.pendingCountForChat(chatID)
				if pendingCount > 1 {
					if msg.ReplyToMessage == nil {
						p.maybeSendThreadingReminder(chatID, pendingCount)
					}
					continue
				}
				// Backward-compatible fallback for single active request.
			}

			reply := contract.Reply{
				RequestID:         requestID,
				Text:              text,
				Raw:               text,
				ProviderMessageID: fmt.Sprintf("%d", msg.MessageID),
				ReceivedAt:        time.Unix(msg.Date, 0).UTC(),
			}
			if msg.From != nil {
				if strings.TrimSpace(msg.From.Username) != "" {
					reply.From = msg.From.Username
				} else {
					reply.From = strings.TrimSpace(strings.Join([]string{msg.From.FirstName, msg.From.LastName}, " "))
				}
			}

			return reply, nil
		}
	}
}

func (p *TelegramProvider) chatIDValue() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.chatID
}

func (p *TelegramProvider) ensureChatID(ctx context.Context) error {
	if p.chatIDValue() != 0 {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("telegram chat is not linked; send /start to the bot first: %w", ctx.Err())
		default:
		}

		updates, err := p.getUpdates(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("telegram chat is not linked; send /start to the bot first: %w", ctx.Err())
			}
			return err
		}

		for _, up := range updates {
			msg := up.Message
			if msg == nil {
				continue
			}
			if !isTelegramStartCommand(msg.Text) {
				continue
			}

			p.mu.Lock()
			p.chatID = msg.Chat.ID
			p.mu.Unlock()
			persistTelegramChatID(msg.Chat.ID)
			return nil
		}
	}
}

func isTelegramStartCommand(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return false
	}
	token := strings.Fields(t)[0]
	if token == "/start" {
		return true
	}
	return strings.HasPrefix(token, "/start@") && len(token) > len("/start@")
}

func persistTelegramChatID(chatID int64) {
	if chatID == 0 {
		return
	}
	cfg, err := config.Load()
	if err != nil {
		return
	}
	if cfg.Telegram.ChatID == chatID {
		return
	}
	cfg.Telegram.ChatID = chatID
	_ = config.Save(cfg)
}

func (p *TelegramProvider) registerPending(requestID string, chatID, messageID int64) error {
	if strings.TrimSpace(requestID) == "" || chatID == 0 || messageID == 0 {
		return fmt.Errorf("invalid telegram pending request")
	}

	p.mu.Lock()
	p.pending[requestID] = messageID
	p.mu.Unlock()

	if p.pendingStore == nil {
		return nil
	}

	err := p.pendingStore.Upsert(telegramPendingRecord{
		RequestID: requestID,
		ChatID:    chatID,
		MessageID: messageID,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		p.mu.Lock()
		delete(p.pending, requestID)
		p.mu.Unlock()
		return err
	}
	return nil
}

func (p *TelegramProvider) lookupPending(requestID string) (int64, int64, error) {
	if p.pendingStore != nil {
		rec, ok, err := p.pendingStore.Get(requestID)
		if err == nil && ok && rec.ChatID != 0 && rec.MessageID != 0 {
			return rec.ChatID, rec.MessageID, nil
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: telegram pending store read failed: %v\n", err)
		}
	}

	p.mu.Lock()
	msgID, ok := p.pending[requestID]
	chatID := p.chatID
	p.mu.Unlock()
	if !ok || msgID == 0 || chatID == 0 {
		return 0, 0, fmt.Errorf("unknown request id %q", requestID)
	}
	return chatID, msgID, nil
}

func (p *TelegramProvider) clearPending(requestID string) {
	p.mu.Lock()
	delete(p.pending, requestID)
	p.mu.Unlock()

	if p.pendingStore != nil {
		if err := p.pendingStore.Delete(requestID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: telegram pending store delete failed: %v\n", err)
		}
	}
}

func (p *TelegramProvider) pendingCountForChat(chatID int64) int {
	if chatID == 0 {
		return 0
	}
	if p.pendingStore != nil {
		if n, err := p.pendingStore.CountByChat(chatID); err == nil {
			return n
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.pending)
}

func (p *TelegramProvider) maybeSendThreadingReminder(chatID int64, pendingCount int) {
	p.mu.Lock()
	now := time.Now()
	if pendingCount <= 1 || chatID == 0 || (!p.lastReminderAt.IsZero() && now.Sub(p.lastReminderAt) < telegramReplyReminderCooldown) {
		p.mu.Unlock()
		return
	}
	p.lastReminderAt = now
	p.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = p.sendTelegramMessage(ctx, chatID, telegramThreadingReminderText(pendingCount))
}

func telegramThreadingReminderText(pendingCount int) string {
	if pendingCount <= 1 {
		return "Please reply directly to the message you are answering."
	}
	return fmt.Sprintf(
		"You have %d unanswered consult-human questions. Please reply directly to the exact message you are answering.",
		pendingCount,
	)
}

func (p *TelegramProvider) sendTelegramMessage(ctx context.Context, chatID int64, text string) (int64, error) {
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return 0, fmt.Errorf("telegram sendMessage status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var tr telegramSendResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return 0, err
	}
	if !tr.OK {
		return 0, fmt.Errorf("telegram sendMessage failed")
	}

	return tr.Result.MessageID, nil
}

func (p *TelegramProvider) getUpdates(ctx context.Context) ([]telegramUpdate, error) {
	p.mu.Lock()
	offset := p.nextUpdateID
	p.mu.Unlock()

	timeoutSeconds := int(p.pollInterval / time.Second)
	if timeoutSeconds <= 0 {
		timeoutSeconds = 1
	} else if timeoutSeconds > 50 {
		timeoutSeconds = 50
	}
	payload := map[string]any{
		"timeout": timeoutSeconds,
	}
	if offset > 0 {
		payload["offset"] = offset
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/getUpdates", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("telegram getUpdates status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var gr telegramGetUpdatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, err
	}
	if !gr.OK {
		return nil, fmt.Errorf("telegram getUpdates failed")
	}

	var maxUpdate int64
	for _, up := range gr.Result {
		if up.UpdateID > maxUpdate {
			maxUpdate = up.UpdateID
		}
	}
	if maxUpdate > 0 {
		p.mu.Lock()
		if maxUpdate+1 > p.nextUpdateID {
			p.nextUpdateID = maxUpdate + 1
		}
		p.mu.Unlock()
	}

	return gr.Result, nil
}

type telegramSendResponse struct {
	OK     bool            `json:"ok"`
	Result telegramMessage `json:"result"`
}

type telegramGetUpdatesResponse struct {
	OK     bool             `json:"ok"`
	Result []telegramUpdate `json:"result"`
}

type telegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *telegramMessage `json:"message"`
}

type telegramMessage struct {
	MessageID      int64            `json:"message_id"`
	Date           int64            `json:"date"`
	Text           string           `json:"text"`
	Chat           telegramChat     `json:"chat"`
	From           *telegramUser    `json:"from"`
	ReplyToMessage *telegramMessage `json:"reply_to_message"`
}

type telegramChat struct {
	ID int64 `json:"id"`
}

type telegramUser struct {
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}
