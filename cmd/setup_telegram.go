package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/AlhasanIQ/consult-human/config"
)

const setupTelegramLinkTimeout = 2 * time.Minute

var telegramSetupLinkFn = waitForTelegramStartForSetup

func runTelegramSetup(reader *bufio.Reader, s *sty, cfg *config.Config) error {
	s.section("Telegram")

	defaultToken := strings.TrimSpace(cfg.Telegram.BotToken)
	var token string
	if defaultToken == "" {
		fmt.Fprintf(s.w, "  Create a bot and get your token:\n\n")
		s.step(1, "Open Telegram and chat with "+s.bold("@BotFather"))
		s.step(2, "Send "+s.bold("/newbot")+" and follow the prompts")
		s.step(3, "Copy the bot token")
		fmt.Fprintln(s.w)

		line, err := promptRequiredLine(reader, s, s.promptLabel("Bot token: "))
		if err != nil {
			return err
		}
		token = line
	} else {
		token = defaultToken
		s.info(s.dim("Using saved Telegram bot token from config."))
	}
	cfg.Telegram.BotToken = token

	fmt.Fprintln(s.w)
	fmt.Fprintf(s.w, "  Now send %s to your bot from the chat you want to use.\n\n", s.bold("/start"))

	sp := s.startSpinner("Waiting for /start message...")
	chatID, err := telegramSetupLinkFn(cfg.Telegram.BotToken, setupTelegramLinkTimeout, s.w)
	sp.stop()
	if err != nil {
		return fmt.Errorf("could not link Telegram chat: %w", err)
	}

	cfg.Telegram.ChatID = chatID
	s.success(fmt.Sprintf("Linked to chat %d", chatID))
	return nil
}

func waitForTelegramStartForSetup(token string, timeout time.Duration, w io.Writer) (int64, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, fmt.Errorf("missing telegram token")
	}
	baseURL := fmt.Sprintf("https://api.telegram.org/bot%s", token)
	return waitForTelegramStartWithBaseURL(baseURL, timeout, w)
}

func waitForTelegramStartWithBaseURL(baseURL string, timeout time.Duration, w io.Writer) (int64, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return 0, fmt.Errorf("missing telegram api base URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := &http.Client{Timeout: 45 * time.Second}
	var offset int64

	for {
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("timed out waiting for /start")
		default:
		}

		updates, nextOffset, err := fetchTelegramSetupUpdates(ctx, client, baseURL, offset)
		if err != nil {
			if ctx.Err() != nil {
				return 0, fmt.Errorf("timed out waiting for /start")
			}
			return 0, err
		}
		offset = nextOffset

		for _, up := range updates {
			if up.Message == nil {
				continue
			}
			if !isSetupTelegramStartCommand(up.Message.Text) {
				continue
			}
			if up.Message.Chat.ID == 0 {
				continue
			}
			return up.Message.Chat.ID, nil
		}
	}
}

func fetchTelegramSetupUpdates(ctx context.Context, client *http.Client, baseURL string, offset int64) ([]setupTelegramUpdate, int64, error) {
	payload := map[string]any{"timeout": 20}
	if offset > 0 {
		payload["offset"] = offset
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, offset, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/getUpdates", bytes.NewReader(body))
	if err != nil {
		return nil, offset, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, offset, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, offset, fmt.Errorf("telegram getUpdates status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var decoded setupTelegramGetUpdatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, offset, err
	}
	if !decoded.OK {
		return nil, offset, fmt.Errorf("telegram getUpdates failed")
	}

	nextOffset := offset
	for _, up := range decoded.Result {
		if up.UpdateID+1 > nextOffset {
			nextOffset = up.UpdateID + 1
		}
	}
	return decoded.Result, nextOffset, nil
}

func isSetupTelegramStartCommand(text string) bool {
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

type setupTelegramGetUpdatesResponse struct {
	OK     bool                  `json:"ok"`
	Result []setupTelegramUpdate `json:"result"`
}

type setupTelegramUpdate struct {
	UpdateID int64                 `json:"update_id"`
	Message  *setupTelegramMessage `json:"message"`
}

type setupTelegramMessage struct {
	Text string            `json:"text"`
	Chat setupTelegramChat `json:"chat"`
}

type setupTelegramChat struct {
	ID int64 `json:"id"`
}
