package idempotency

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type benchStore struct{}

func (benchStore) Get(context.Context, string) (Record, bool, error) {
	return Record{}, false, nil
}

func (benchStore) TryBegin(context.Context, string, Record, time.Duration) (bool, error) {
	return true, nil
}

func (benchStore) Save(context.Context, string, Record, time.Duration) error {
	return nil
}

type benchReplayStore struct {
	record Record
}

func (s benchReplayStore) Get(context.Context, string) (Record, bool, error) {
	return s.record, true, nil
}

func (benchReplayStore) TryBegin(context.Context, string, Record, time.Duration) (bool, error) {
	return false, nil
}

func (benchReplayStore) Save(context.Context, string, Record, time.Duration) error {
	return nil
}

func (benchStore) Release(context.Context, string) error {
	return nil
}

func (benchStore) ReleaseReservation(context.Context, string, string) error {
	return nil
}

func (benchReplayStore) Release(context.Context, string) error {
	return nil
}

func (benchReplayStore) ReleaseReservation(context.Context, string, string) error {
	return nil
}

func BenchmarkIdempotencyNew(b *testing.B) {
	mw, err := New(Options{
		Store:        benchStore{},
		MaxBodyBytes: 1 << 20,
	})
	if err != nil {
		b.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))

	body := []byte(`{"amount":42}`)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "bench-key")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkIdempotencyReplay(b *testing.B) {
	body := []byte(`{"amount":42}`)
	sample := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", nil)
	sample.Header.Set("Content-Type", "application/json")
	hash, err := DefaultHash(sample, body)
	if err != nil {
		b.Fatalf("hash: %v", err)
	}

	record := Record{
		State:       StateCompleted,
		Status:      http.StatusCreated,
		RequestHash: hash,
		Body:        []byte("ok"),
	}

	mw, err := New(Options{
		Store:        benchReplayStore{record: record},
		MaxBodyBytes: 1 << 20,
	})
	if err != nil {
		b.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "bench-key")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}
