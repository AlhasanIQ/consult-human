package provider

import (
	"path/filepath"
	"testing"
	"time"
)

func TestTelegramPendingStoreCRUD(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telegram-pending.json")
	store := &telegramPendingStore{
		path: path,
		lock: path + ".lock",
	}

	recA := telegramPendingRecord{
		RequestID: "req-a",
		ChatID:    1001,
		MessageID: 5001,
		CreatedAt: time.Now().UTC(),
	}
	recB := telegramPendingRecord{
		RequestID: "req-b",
		ChatID:    1001,
		MessageID: 5002,
		CreatedAt: time.Now().UTC(),
	}
	if err := store.Upsert(recA); err != nil {
		t.Fatalf("Upsert recA: %v", err)
	}
	if err := store.Upsert(recB); err != nil {
		t.Fatalf("Upsert recB: %v", err)
	}

	got, ok, err := store.Get("req-a")
	if err != nil {
		t.Fatalf("Get req-a: %v", err)
	}
	if !ok {
		t.Fatalf("expected req-a to exist")
	}
	if got.ChatID != recA.ChatID || got.MessageID != recA.MessageID {
		t.Fatalf("unexpected req-a record: %#v", got)
	}

	count, err := store.CountByChat(1001)
	if err != nil {
		t.Fatalf("CountByChat: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected chat count 2, got %d", count)
	}

	if err := store.Delete("req-a"); err != nil {
		t.Fatalf("Delete req-a: %v", err)
	}
	_, ok, err = store.Get("req-a")
	if err != nil {
		t.Fatalf("Get deleted req-a: %v", err)
	}
	if ok {
		t.Fatalf("expected req-a to be deleted")
	}
}

func TestTelegramPendingStorePersistsAcrossInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telegram-pending.json")
	store1 := &telegramPendingStore{
		path: path,
		lock: path + ".lock",
	}
	store2 := &telegramPendingStore{
		path: path,
		lock: path + ".lock",
	}

	if err := store1.Upsert(telegramPendingRecord{
		RequestID: "req-z",
		ChatID:    2002,
		MessageID: 7007,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, ok, err := store2.Get("req-z")
	if err != nil {
		t.Fatalf("Get from second instance: %v", err)
	}
	if !ok {
		t.Fatalf("expected req-z to exist")
	}
	if got.ChatID != 2002 || got.MessageID != 7007 {
		t.Fatalf("unexpected persisted record: %#v", got)
	}
}
