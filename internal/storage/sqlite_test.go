package storage

import (
	"context"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestNewSQLiteStore(t *testing.T) {
	s := newTestStore(t)
	if s == nil {
		t.Fatal("store is nil")
	}
}

func TestSQLiteStore_PutGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	err := s.Put(ctx, Record{
		Key:   "greeting",
		Value: []byte("hello world"),
		Metadata: map[string]string{
			"lang": "en",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	rec, err := s.Get(ctx, "greeting")
	if err != nil {
		t.Fatal(err)
	}
	if rec == nil {
		t.Fatal("record not found")
	}
	if string(rec.Value) != "hello world" {
		t.Errorf("Value = %q", string(rec.Value))
	}
	if rec.Metadata["lang"] != "en" {
		t.Errorf("Metadata = %v", rec.Metadata)
	}
}

func TestSQLiteStore_Get_NotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	rec, err := s.Get(ctx, "missing")
	if err != nil {
		t.Fatal(err)
	}
	if rec != nil {
		t.Error("expected nil for missing key")
	}
}

func TestSQLiteStore_Put_Upsert(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.Put(ctx, Record{Key: "k1", Value: []byte("v1")})
	s.Put(ctx, Record{Key: "k1", Value: []byte("v2")}) // Update.

	rec, _ := s.Get(ctx, "k1")
	if string(rec.Value) != "v2" {
		t.Errorf("Value = %q, want v2", string(rec.Value))
	}

	count, _ := s.Count(ctx)
	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
	}
}

func TestSQLiteStore_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.Put(ctx, Record{Key: "k1", Value: []byte("v1")})
	if err := s.Delete(ctx, "k1"); err != nil {
		t.Fatal(err)
	}

	rec, _ := s.Get(ctx, "k1")
	if rec != nil {
		t.Error("expected nil after delete")
	}
}

func TestSQLiteStore_Delete_NotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Should not error on missing key.
	if err := s.Delete(ctx, "missing"); err != nil {
		t.Fatal(err)
	}
}

func TestSQLiteStore_List(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.Put(ctx, Record{Key: "user:alice", Value: []byte("a")})
	s.Put(ctx, Record{Key: "user:bob", Value: []byte("b")})
	s.Put(ctx, Record{Key: "task:1", Value: []byte("t1")})

	keys, err := s.List(ctx, "user:", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("List = %d, want 2", len(keys))
	}

	taskKeys, _ := s.List(ctx, "task:", 10)
	if len(taskKeys) != 1 {
		t.Errorf("task keys = %d", len(taskKeys))
	}
}

func TestSQLiteStore_List_Limit(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		s.Put(ctx, Record{Key: "item:" + string(rune('a'+i)), Value: []byte("v")})
	}

	keys, _ := s.List(ctx, "item:", 3)
	if len(keys) != 3 {
		t.Errorf("List with limit 3 = %d", len(keys))
	}
}

func TestSQLiteStore_List_Empty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	keys, err := s.List(ctx, "none:", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Errorf("List = %d", len(keys))
	}
}

func TestSQLiteStore_Search(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.Put(ctx, Record{Key: "doc:1", Value: []byte("Go programming language tutorial")})
	s.Put(ctx, Record{Key: "doc:2", Value: []byte("Python machine learning guide")})
	s.Put(ctx, Record{Key: "doc:3", Value: []byte("Go concurrency patterns with goroutines")})

	results, err := s.Search(ctx, "Go", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("Search 'Go' = %d, want 2", len(results))
	}
}

func TestSQLiteStore_Search_Empty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	results, err := s.Search(ctx, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("empty search = %d", len(results))
	}
}

func TestSQLiteStore_Search_NoMatch(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.Put(ctx, Record{Key: "doc:1", Value: []byte("hello world")})

	results, err := s.Search(ctx, "nonexistent", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("Search = %d, want 0", len(results))
	}
}

func TestSQLiteStore_Count(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	count, _ := s.Count(ctx)
	if count != 0 {
		t.Errorf("Count = %d", count)
	}

	s.Put(ctx, Record{Key: "a", Value: []byte("1")})
	s.Put(ctx, Record{Key: "b", Value: []byte("2")})

	count, _ = s.Count(ctx)
	if count != 2 {
		t.Errorf("Count = %d, want 2", count)
	}
}

func TestSQLiteStore_Expiry(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Record that already expired.
	s.Put(ctx, Record{
		Key:       "expired",
		Value:     []byte("old"),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	})

	rec, err := s.Get(ctx, "expired")
	if err != nil {
		t.Fatal(err)
	}
	if rec != nil {
		t.Error("expired record should return nil")
	}
}

func TestSQLiteStore_Expiry_Future(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.Put(ctx, Record{
		Key:       "valid",
		Value:     []byte("fresh"),
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})

	rec, _ := s.Get(ctx, "valid")
	if rec == nil {
		t.Error("future expiry should return record")
	}
}

func TestSQLiteStore_Timestamps(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.Put(ctx, Record{Key: "ts", Value: []byte("test")})

	rec, _ := s.Get(ctx, "ts")
	if rec.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if rec.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestSQLiteStore_NoMetadata(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.Put(ctx, Record{Key: "bare", Value: []byte("minimal")})

	rec, _ := s.Get(ctx, "bare")
	if rec == nil {
		t.Fatal("record not found")
	}
	if len(rec.Metadata) != 0 {
		t.Errorf("Metadata = %v, want empty", rec.Metadata)
	}
}

// Verify Store interface compliance.
func TestSQLiteStore_ImplementsStore(t *testing.T) {
	var _ Store = (*SQLiteStore)(nil)
}
