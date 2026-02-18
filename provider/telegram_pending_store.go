package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlhasanIQ/consult-human/config"
)

const envTelegramPendingStorePath = "CONSULT_HUMAN_TELEGRAM_PENDING_STORE"

type telegramPendingRecord struct {
	RequestID string    `json:"request_id"`
	ChatID    int64     `json:"chat_id"`
	MessageID int64     `json:"message_id"`
	CreatedAt time.Time `json:"created_at"`
}

type telegramPendingStore struct {
	path string
	lock string
}

func newTelegramPendingStore() (*telegramPendingStore, error) {
	raw := strings.TrimSpace(os.Getenv(envTelegramPendingStorePath))
	if raw == "" {
		stateDir, err := config.DefaultStateDir()
		if err != nil {
			return nil, err
		}
		raw = filepath.Join(stateDir, "telegram-pending.json")
	} else {
		expanded, err := config.ExpandPath(raw)
		if err != nil {
			return nil, err
		}
		raw = expanded
	}
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("invalid telegram pending store path")
	}
	return &telegramPendingStore{
		path: raw,
		lock: raw + ".lock",
	}, nil
}

func (s *telegramPendingStore) Upsert(rec telegramPendingRecord) error {
	return s.withLock(func() error {
		state, err := s.loadLocked()
		if err != nil {
			return err
		}
		state[rec.RequestID] = rec
		return s.saveLocked(state)
	})
}

func (s *telegramPendingStore) Get(requestID string) (telegramPendingRecord, bool, error) {
	var out telegramPendingRecord
	var ok bool
	err := s.withLock(func() error {
		state, err := s.loadLocked()
		if err != nil {
			return err
		}
		out, ok = state[requestID]
		return nil
	})
	if err != nil {
		return telegramPendingRecord{}, false, err
	}
	return out, ok, nil
}

func (s *telegramPendingStore) Delete(requestID string) error {
	return s.withLock(func() error {
		state, err := s.loadLocked()
		if err != nil {
			return err
		}
		delete(state, requestID)
		return s.saveLocked(state)
	})
}

func (s *telegramPendingStore) CountByChat(chatID int64) (int, error) {
	var count int
	err := s.withLock(func() error {
		state, err := s.loadLocked()
		if err != nil {
			return err
		}
		for _, rec := range state {
			if rec.ChatID == chatID {
				count++
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *telegramPendingStore) withLock(fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		lockFile, err := os.OpenFile(s.lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = lockFile.WriteString(fmt.Sprintf("%d\n", os.Getpid()))
			_ = lockFile.Close()
			defer os.Remove(s.lock)
			return fn()
		}
		if !os.IsExist(err) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for telegram pending store lock")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (s *telegramPendingStore) loadLocked() (map[string]telegramPendingRecord, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]telegramPendingRecord), nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return make(map[string]telegramPendingRecord), nil
	}

	state := make(map[string]telegramPendingRecord)
	if err := json.Unmarshal(b, &state); err != nil {
		return nil, fmt.Errorf("parse telegram pending store: %w", err)
	}
	return state, nil
}

func (s *telegramPendingStore) saveLocked(state map[string]telegramPendingRecord) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
