package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AlhasanIQ/consult-human/config"
)

const (
	telegramInboxReplyTTL      = 20 * time.Minute
	telegramInboxLooseTTL      = 5 * time.Minute
	telegramInboxLockWait      = 3 * time.Second
	telegramInboxLockMaxAge    = 10 * time.Second
	telegramInboxMaxEntries    = 4096
	telegramPollerLockMaxAge   = 2 * time.Minute
	telegramPollerWaitInterval = 150 * time.Millisecond
)

type telegramInboxEntry struct {
	UpdateID         int64     `json:"update_id"`
	ChatID           int64     `json:"chat_id"`
	MessageID        int64     `json:"message_id"`
	ReplyToMessageID int64     `json:"reply_to_message_id,omitempty"`
	Text             string    `json:"text"`
	Date             int64     `json:"date"`
	Username         string    `json:"username,omitempty"`
	FirstName        string    `json:"first_name,omitempty"`
	LastName         string    `json:"last_name,omitempty"`
	IngestedAt       time.Time `json:"ingested_at"`
	ExpiresAt        time.Time `json:"expires_at"`
}

type telegramInboxState struct {
	NextUpdateID int64                `json:"next_update_id"`
	Entries      []telegramInboxEntry `json:"entries"`
}

type telegramInboxStore struct {
	path string
	lock string
}

type telegramPollerLock struct {
	path string
}

func newTelegramInboxStore(cfg config.Config) (*telegramInboxStore, error) {
	raw, err := config.EffectiveTelegramInboxStorePath(cfg)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("invalid telegram inbox store path")
	}
	return &telegramInboxStore{
		path: raw,
		lock: raw + ".lock",
	}, nil
}

func newTelegramPollerLock(cfg config.Config) (*telegramPollerLock, error) {
	inboxPath, err := config.EffectiveTelegramInboxStorePath(cfg)
	if err != nil {
		return nil, err
	}
	return &telegramPollerLock{
		path: filepath.Join(filepath.Dir(inboxPath), "telegram-poller.lock"),
	}, nil
}

func (s *telegramInboxStore) NextOffset() (int64, error) {
	var offset int64
	err := s.withLock(func() error {
		state, changed, err := s.loadPrunedLocked(time.Now().UTC())
		if err != nil {
			return err
		}
		if changed {
			if err := s.saveLocked(state); err != nil {
				return err
			}
		}
		offset = state.NextUpdateID
		return nil
	})
	if err != nil {
		return 0, err
	}
	return offset, nil
}

func (s *telegramInboxStore) AppendUpdates(updates []telegramUpdate) (int, int64, error) {
	if len(updates) == 0 {
		offset, err := s.NextOffset()
		return 0, offset, err
	}

	var added int
	var nextOffset int64
	err := s.withLock(func() error {
		now := time.Now().UTC()
		state, _, err := s.loadPrunedLocked(now)
		if err != nil {
			return err
		}

		existing := make(map[int64]struct{}, len(state.Entries))
		for _, rec := range state.Entries {
			existing[rec.UpdateID] = struct{}{}
		}

		maxUpdate := int64(0)
		for _, up := range updates {
			if up.UpdateID > maxUpdate {
				maxUpdate = up.UpdateID
			}
			if _, ok := existing[up.UpdateID]; ok {
				continue
			}
			msg := up.Message
			if msg == nil {
				continue
			}
			text := strings.TrimSpace(msg.Text)
			if text == "" {
				continue
			}

			expiresAt := now.Add(telegramInboxLooseTTL)
			replyToID := int64(0)
			if msg.ReplyToMessage != nil {
				replyToID = msg.ReplyToMessage.MessageID
				if replyToID > 0 {
					expiresAt = now.Add(telegramInboxReplyTTL)
				}
			}

			entry := telegramInboxEntry{
				UpdateID:         up.UpdateID,
				ChatID:           msg.Chat.ID,
				MessageID:        msg.MessageID,
				ReplyToMessageID: replyToID,
				Text:             text,
				Date:             msg.Date,
				IngestedAt:       now,
				ExpiresAt:        expiresAt,
			}
			if msg.From != nil {
				entry.Username = strings.TrimSpace(msg.From.Username)
				entry.FirstName = strings.TrimSpace(msg.From.FirstName)
				entry.LastName = strings.TrimSpace(msg.From.LastName)
			}
			state.Entries = append(state.Entries, entry)
			existing[up.UpdateID] = struct{}{}
			added++
		}

		if maxUpdate > 0 && maxUpdate+1 > state.NextUpdateID {
			state.NextUpdateID = maxUpdate + 1
		}
		sort.Slice(state.Entries, func(i, j int) bool {
			return state.Entries[i].UpdateID < state.Entries[j].UpdateID
		})
		if len(state.Entries) > telegramInboxMaxEntries {
			state.Entries = state.Entries[len(state.Entries)-telegramInboxMaxEntries:]
		}

		nextOffset = state.NextUpdateID
		return s.saveLocked(state)
	})
	if err != nil {
		return 0, 0, err
	}
	return added, nextOffset, nil
}

func (s *telegramInboxStore) ClaimForRequest(chatID, targetMessageID int64, pendingCount int) (*telegramInboxEntry, bool, error) {
	var claimed *telegramInboxEntry
	var needsReminder bool

	err := s.withLock(func() error {
		state, _, err := s.loadPrunedLocked(time.Now().UTC())
		if err != nil {
			return err
		}

		changed := false
		for i := 0; i < len(state.Entries); {
			entry := state.Entries[i]
			if entry.ChatID != chatID {
				i++
				continue
			}

			matchesByReply := targetMessageID > 0 && entry.ReplyToMessageID == targetMessageID
			if matchesByReply {
				c := entry
				claimed = &c
				state.Entries = append(state.Entries[:i], state.Entries[i+1:]...)
				changed = true
				break
			}

			if pendingCount > 1 {
				if entry.ReplyToMessageID == 0 && entry.MessageID > targetMessageID {
					// Ambiguous free-text message while multiple requests are pending.
					needsReminder = true
					state.Entries = append(state.Entries[:i], state.Entries[i+1:]...)
					changed = true
					continue
				}
				if entry.ReplyToMessageID == 0 && entry.MessageID <= targetMessageID {
					// Stale pre-request chatter for this receiver; drop quietly.
					state.Entries = append(state.Entries[:i], state.Entries[i+1:]...)
					changed = true
					continue
				}
				i++
				continue
			}

			// Single-pending fallback for backward compatibility.
			if entry.ReplyToMessageID != 0 && entry.ReplyToMessageID != targetMessageID {
				// Not for this request; drop to avoid inbox buildup from abandoned/irrelevant replies.
				state.Entries = append(state.Entries[:i], state.Entries[i+1:]...)
				changed = true
				continue
			}
			if entry.MessageID <= targetMessageID {
				// Pre-request chatter can never satisfy this request.
				state.Entries = append(state.Entries[:i], state.Entries[i+1:]...)
				changed = true
				continue
			}

			if entry.MessageID > targetMessageID {
				c := entry
				claimed = &c
				state.Entries = append(state.Entries[:i], state.Entries[i+1:]...)
				changed = true
				break
			}
			i++
		}

		if changed {
			return s.saveLocked(state)
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return claimed, needsReminder, nil
}

func (s *telegramInboxStore) withLock(fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	deadline := time.Now().Add(telegramInboxLockWait)
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
		stale, staleErr := s.isStaleLock()
		if staleErr == nil && stale {
			_ = os.Remove(s.lock)
			continue
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for telegram inbox store lock")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (s *telegramInboxStore) isStaleLock() (bool, error) {
	st, err := os.Stat(s.lock)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	rawPID, _ := os.ReadFile(s.lock)
	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(rawPID)))
	if parseErr == nil && pid > 0 && runtime.GOOS != "windows" {
		if !processExists(pid) {
			return true, nil
		}
		return false, nil
	}
	return time.Since(st.ModTime()) > telegramInboxLockMaxAge, nil
}

func (s *telegramInboxStore) loadPrunedLocked(now time.Time) (telegramInboxState, bool, error) {
	state, err := s.loadLocked()
	if err != nil {
		return telegramInboxState{}, false, err
	}
	changed := pruneExpiredInboxEntries(&state, now)
	return state, changed, nil
}

func pruneExpiredInboxEntries(state *telegramInboxState, now time.Time) bool {
	if state == nil {
		return false
	}
	changed := false
	out := state.Entries[:0]
	now = now.UTC()
	for _, rec := range state.Entries {
		if !rec.ExpiresAt.IsZero() && !rec.ExpiresAt.UTC().After(now) {
			changed = true
			continue
		}
		out = append(out, rec)
	}
	state.Entries = out
	return changed
}

func (s *telegramInboxStore) loadLocked() (telegramInboxState, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return telegramInboxState{}, nil
		}
		return telegramInboxState{}, err
	}
	if len(b) == 0 {
		return telegramInboxState{}, nil
	}
	var state telegramInboxState
	if err := json.Unmarshal(b, &state); err != nil {
		return telegramInboxState{}, fmt.Errorf("parse telegram inbox store: %w", err)
	}
	return state, nil
}

func (s *telegramInboxStore) saveLocked(state telegramInboxState) error {
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

func (l *telegramPollerLock) TryWithLock(fn func() error) (bool, error) {
	if l == nil {
		return false, fmt.Errorf("nil telegram poller lock")
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return false, err
	}

	for {
		f, err := os.OpenFile(l.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = f.WriteString(fmt.Sprintf("%d\n", os.Getpid()))
			_ = f.Close()
			defer os.Remove(l.path)
			return true, fn()
		}
		if !os.IsExist(err) {
			return false, err
		}
		stale, staleErr := l.isStale()
		if staleErr == nil && stale {
			_ = os.Remove(l.path)
			continue
		}
		return false, nil
	}
}

func (l *telegramPollerLock) isStale() (bool, error) {
	st, err := os.Stat(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	rawPID, _ := os.ReadFile(l.path)
	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(rawPID)))
	if parseErr == nil && pid > 0 && runtime.GOOS != "windows" {
		if !processExists(pid) {
			return true, nil
		}
		return false, nil
	}
	return time.Since(st.ModTime()) > telegramPollerLockMaxAge, nil
}
