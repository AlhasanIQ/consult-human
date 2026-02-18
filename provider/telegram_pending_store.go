package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/AlhasanIQ/consult-human/config"
)

const (
	telegramPendingLegacyTTL  = 24 * time.Hour
	telegramPendingLockWait   = 3 * time.Second
	telegramPendingLockMaxAge = 10 * time.Second
)

type telegramPendingRecord struct {
	RequestID string    `json:"request_id"`
	ChatID    int64     `json:"chat_id"`
	MessageID int64     `json:"message_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	OwnerPID  int       `json:"owner_pid,omitempty"`
	OwnerHost string    `json:"owner_host,omitempty"`
}

type telegramPendingStore struct {
	path string
	lock string
}

var telegramLocalHostname = loadTelegramLocalHostname()

func newTelegramPendingStore(cfg config.Config) (*telegramPendingStore, error) {
	raw, err := config.EffectiveTelegramPendingStorePath(cfg)
	if err != nil {
		return nil, err
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
		now := time.Now().UTC()
		if rec.CreatedAt.IsZero() {
			rec.CreatedAt = now
		}
		if rec.ExpiresAt.IsZero() {
			rec.ExpiresAt = rec.CreatedAt.Add(telegramPendingLegacyTTL)
		}

		state, _, err := s.loadPrunedLocked(now)
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
		state, changed, err := s.loadPrunedLocked(time.Now().UTC())
		if err != nil {
			return err
		}
		if changed {
			if err := s.saveLocked(state); err != nil {
				return err
			}
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
		state, _, err := s.loadPrunedLocked(time.Now().UTC())
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
		state, changed, err := s.loadPrunedLocked(time.Now().UTC())
		if err != nil {
			return err
		}
		if changed {
			if err := s.saveLocked(state); err != nil {
				return err
			}
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

	deadline := time.Now().Add(telegramPendingLockWait)
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
			return fmt.Errorf("timeout waiting for telegram pending store lock")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (s *telegramPendingStore) isStaleLock() (bool, error) {
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

	// If we can't parse the PID, fall back to lock-file age.
	return time.Since(st.ModTime()) > telegramPendingLockMaxAge, nil
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if errno == syscall.EPERM {
			return true
		}
		if errno == syscall.ESRCH {
			return false
		}
	}
	return false
}

func (s *telegramPendingStore) loadPrunedLocked(now time.Time) (map[string]telegramPendingRecord, bool, error) {
	state, err := s.loadLocked()
	if err != nil {
		return nil, false, err
	}
	changed := pruneExpiredPendingRecords(state, now)
	return state, changed, nil
}

func pruneExpiredPendingRecords(state map[string]telegramPendingRecord, now time.Time) bool {
	changed := false
	for requestID, rec := range state {
		if isTelegramPendingExpired(rec, now) {
			delete(state, requestID)
			changed = true
		}
	}
	return changed
}

func isTelegramPendingExpired(rec telegramPendingRecord, now time.Time) bool {
	if isTelegramPendingOrphaned(rec) {
		return true
	}

	now = now.UTC()
	if !rec.ExpiresAt.IsZero() {
		return !rec.ExpiresAt.UTC().After(now)
	}
	if !rec.CreatedAt.IsZero() {
		return !rec.CreatedAt.UTC().Add(telegramPendingLegacyTTL).After(now)
	}
	return false
}

func isTelegramPendingOrphaned(rec telegramPendingRecord) bool {
	if rec.OwnerPID <= 0 || runtime.GOOS == "windows" {
		return false
	}
	if rec.OwnerHost != "" {
		if telegramLocalHostname == "" {
			return false
		}
		if !strings.EqualFold(strings.TrimSpace(rec.OwnerHost), telegramLocalHostname) {
			return false
		}
	}
	return !processExists(rec.OwnerPID)
}

func loadTelegramLocalHostname() string {
	host, err := os.Hostname()
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(host))
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
