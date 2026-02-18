package provider

import (
	"path/filepath"
	"testing"
	"time"
)

func TestTelegramInboxStoreAppendAndClaimReply(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telegram-inbox.json")
	store := &telegramInboxStore{
		path: path,
		lock: path + ".lock",
	}

	now := time.Now().Unix()
	updates := []telegramUpdate{
		{
			UpdateID: 11,
			Message: &telegramMessage{
				MessageID: 9001,
				Date:      now,
				Text:      "Ship it",
				Chat:      telegramChat{ID: 7001},
				ReplyToMessage: &telegramMessage{
					MessageID: 5001,
				},
			},
		},
	}

	if _, _, err := store.AppendUpdates(updates); err != nil {
		t.Fatalf("AppendUpdates: %v", err)
	}

	got, needsReminder, err := store.ClaimForRequest(7001, 5001, 2)
	if err != nil {
		t.Fatalf("ClaimForRequest: %v", err)
	}
	if needsReminder {
		t.Fatalf("did not expect reminder")
	}
	if got == nil {
		t.Fatalf("expected claimed entry")
	}
	if got.Text != "Ship it" || got.MessageID != 9001 {
		t.Fatalf("unexpected claimed entry: %#v", got)
	}
}

func TestTelegramInboxStoreDropsAmbiguousMessageWhenMultiplePending(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telegram-inbox.json")
	store := &telegramInboxStore{
		path: path,
		lock: path + ".lock",
	}

	now := time.Now().Unix()
	updates := []telegramUpdate{
		{
			UpdateID: 12,
			Message: &telegramMessage{
				MessageID: 9002,
				Date:      now,
				Text:      "random free text",
				Chat:      telegramChat{ID: 7002},
			},
		},
	}

	if _, _, err := store.AppendUpdates(updates); err != nil {
		t.Fatalf("AppendUpdates: %v", err)
	}

	got, needsReminder, err := store.ClaimForRequest(7002, 5002, 3)
	if err != nil {
		t.Fatalf("ClaimForRequest: %v", err)
	}
	if got != nil {
		t.Fatalf("did not expect claimed entry: %#v", got)
	}
	if !needsReminder {
		t.Fatalf("expected reminder flag")
	}

	// The ambiguous message should be removed once observed in multi-pending mode.
	got, needsReminder, err = store.ClaimForRequest(7002, 5002, 3)
	if err != nil {
		t.Fatalf("ClaimForRequest second call: %v", err)
	}
	if got != nil || needsReminder {
		t.Fatalf("expected no leftover ambiguous message, got entry=%#v reminder=%v", got, needsReminder)
	}
}

func TestTelegramInboxStorePrunesExpiredEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telegram-inbox.json")
	store := &telegramInboxStore{
		path: path,
		lock: path + ".lock",
	}

	state := telegramInboxState{
		NextUpdateID: 20,
		Entries: []telegramInboxEntry{
			{
				UpdateID:   19,
				ChatID:     7003,
				MessageID:  9003,
				Text:       "expired",
				IngestedAt: time.Now().UTC().Add(-10 * time.Minute),
				ExpiresAt:  time.Now().UTC().Add(-1 * time.Minute),
			},
		},
	}
	if err := store.saveLocked(state); err != nil {
		t.Fatalf("saveLocked: %v", err)
	}

	if _, err := store.NextOffset(); err != nil {
		t.Fatalf("NextOffset: %v", err)
	}

	reloaded, err := store.loadLocked()
	if err != nil {
		t.Fatalf("loadLocked: %v", err)
	}
	if len(reloaded.Entries) != 0 {
		t.Fatalf("expected expired entries to be pruned, got %#v", reloaded.Entries)
	}
}

func TestTelegramInboxStoreDropsIrrelevantEntriesForSinglePending(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telegram-inbox.json")
	store := &telegramInboxStore{
		path: path,
		lock: path + ".lock",
	}

	now := time.Now().Unix()
	updates := []telegramUpdate{
		{
			UpdateID: 20,
			Message: &telegramMessage{
				MessageID:      5000,
				Date:           now,
				Text:           "old chatter",
				Chat:           telegramChat{ID: 7004},
				ReplyToMessage: nil,
			},
		},
		{
			UpdateID: 21,
			Message: &telegramMessage{
				MessageID: 5003,
				Date:      now,
				Text:      "reply to something else",
				Chat:      telegramChat{ID: 7004},
				ReplyToMessage: &telegramMessage{
					MessageID: 4999,
				},
			},
		},
	}
	if _, _, err := store.AppendUpdates(updates); err != nil {
		t.Fatalf("AppendUpdates: %v", err)
	}

	got, needsReminder, err := store.ClaimForRequest(7004, 5001, 1)
	if err != nil {
		t.Fatalf("ClaimForRequest: %v", err)
	}
	if got != nil {
		t.Fatalf("did not expect a claim, got %#v", got)
	}
	if needsReminder {
		t.Fatalf("did not expect reminder for single pending")
	}

	state, err := store.loadLocked()
	if err != nil {
		t.Fatalf("loadLocked: %v", err)
	}
	if len(state.Entries) != 0 {
		t.Fatalf("expected irrelevant entries to be dropped, got %#v", state.Entries)
	}
}
