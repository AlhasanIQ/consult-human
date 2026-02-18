package provider

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestTelegramPendingStorePrunesExpiredRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telegram-pending.json")
	store := &telegramPendingStore{
		path: path,
		lock: path + ".lock",
	}

	now := time.Now().UTC()
	expired := telegramPendingRecord{
		RequestID: "req-expired",
		ChatID:    3001,
		MessageID: 8001,
		CreatedAt: now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-1 * time.Minute),
	}
	active := telegramPendingRecord{
		RequestID: "req-active",
		ChatID:    3001,
		MessageID: 8002,
		CreatedAt: now,
		ExpiresAt: now.Add(5 * time.Minute),
	}

	if err := store.Upsert(expired); err != nil {
		t.Fatalf("Upsert expired: %v", err)
	}
	if err := store.Upsert(active); err != nil {
		t.Fatalf("Upsert active: %v", err)
	}

	if _, ok, err := store.Get("req-expired"); err != nil {
		t.Fatalf("Get expired: %v", err)
	} else if ok {
		t.Fatalf("expected expired record to be pruned")
	}

	count, err := store.CountByChat(3001)
	if err != nil {
		t.Fatalf("CountByChat: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 active record, got %d", count)
	}
}

func TestTelegramPendingStorePrunesLegacyRecordWithoutExpiresAt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telegram-pending.json")
	store := &telegramPendingStore{
		path: path,
		lock: path + ".lock",
	}

	old := telegramPendingRecord{
		RequestID: "req-legacy-old",
		ChatID:    4001,
		MessageID: 9001,
		CreatedAt: time.Now().UTC().Add(-2 * telegramPendingLegacyTTL),
	}
	if err := store.Upsert(old); err != nil {
		t.Fatalf("Upsert old: %v", err)
	}

	// Simulate a legacy file by removing ExpiresAt after writing.
	state, err := store.loadLocked()
	if err != nil {
		t.Fatalf("loadLocked: %v", err)
	}
	rec := state["req-legacy-old"]
	rec.ExpiresAt = time.Time{}
	state["req-legacy-old"] = rec
	if err := store.saveLocked(state); err != nil {
		t.Fatalf("saveLocked: %v", err)
	}

	if _, ok, err := store.Get("req-legacy-old"); err != nil {
		t.Fatalf("Get old: %v", err)
	} else if ok {
		t.Fatalf("expected legacy record to be pruned using CreatedAt fallback")
	}
}

func TestTelegramPendingStoreRecoversFromStaleLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telegram-pending.json")
	store := &telegramPendingStore{
		path: path,
		lock: path + ".lock",
	}

	// Simulate a stale lock from a dead PID.
	if err := os.WriteFile(store.lock, []byte("999999\n"), 0o600); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}

	rec := telegramPendingRecord{
		RequestID: "req-lock",
		ChatID:    5001,
		MessageID: 10001,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}
	if err := store.Upsert(rec); err != nil {
		t.Fatalf("Upsert with stale lock: %v", err)
	}

	got, ok, err := store.Get("req-lock")
	if err != nil {
		t.Fatalf("Get req-lock: %v", err)
	}
	if !ok || got.MessageID != rec.MessageID {
		t.Fatalf("unexpected record after stale-lock recovery: %#v", got)
	}
}

func TestTelegramPendingStorePrunesDeadOwnerPID(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("owner-pid pruning uses unix process checks")
	}

	deadPID := findDeadPIDForTest()
	if deadPID == 0 {
		t.Skip("could not find a dead pid for this environment")
	}

	path := filepath.Join(t.TempDir(), "telegram-pending.json")
	store := &telegramPendingStore{
		path: path,
		lock: path + ".lock",
	}

	rec := telegramPendingRecord{
		RequestID: "req-dead-owner",
		ChatID:    6001,
		MessageID: 11001,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
		OwnerPID:  deadPID,
		OwnerHost: telegramLocalHostname,
	}
	if err := store.Upsert(rec); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	if _, ok, err := store.Get(rec.RequestID); err != nil {
		t.Fatalf("Get: %v", err)
	} else if ok {
		t.Fatalf("expected dead-owner pending record to be pruned")
	}
}

func TestTelegramPendingStoreKeepsForeignHostOwner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("owner-pid pruning uses unix process checks")
	}

	deadPID := findDeadPIDForTest()
	if deadPID == 0 {
		t.Skip("could not find a dead pid for this environment")
	}

	path := filepath.Join(t.TempDir(), "telegram-pending.json")
	store := &telegramPendingStore{
		path: path,
		lock: path + ".lock",
	}

	rec := telegramPendingRecord{
		RequestID: "req-foreign-owner",
		ChatID:    6002,
		MessageID: 11002,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
		OwnerPID:  deadPID,
		OwnerHost: "remote-machine.example",
	}
	if err := store.Upsert(rec); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	if got, ok, err := store.Get(rec.RequestID); err != nil {
		t.Fatalf("Get: %v", err)
	} else if !ok {
		t.Fatalf("expected foreign-host record to remain")
	} else if got.MessageID != rec.MessageID {
		t.Fatalf("unexpected record: %#v", got)
	}
}

func findDeadPIDForTest() int {
	candidates := []int{999999, 4194304, 2147483000}
	for _, pid := range candidates {
		if pid > 0 && !processExists(pid) {
			return pid
		}
	}
	return 0
}
