package idempotency

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v4/authorization"
	"github.com/aatuh/api-toolkit/v4/httpx"
	"github.com/aatuh/api-toolkit/v4/ports"
)

func TestIdempotencyReplay(t *testing.T) {
	mem := newMemoryStore()
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"body": string(body)})
	}))

	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req1.Header.Set("Idempotency-Key", "key-1")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec1.Code)
	}

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req2.Header.Set("Idempotency-Key", "key-1")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected 201 on replay, got %d", rec2.Code)
	}
	if rec2.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("expected replay header to be set")
	}
	if rec2.Body.String() != rec1.Body.String() {
		t.Fatalf("expected replayed body to match original")
	}

	req3 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("beta"))
	req3.Header.Set("Idempotency-Key", "key-1")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusConflict {
		t.Fatalf("expected conflict on key reuse, got %d", rec3.Code)
	}
}

func FuzzDefaultHashAndReplayMetadata(f *testing.F) {
	f.Add("POST", "/widgets", "b=2&a=1", "application/json", `{"name":"widget"}`, "replay-value")
	f.Add("get", "/widgets/42", "tag=a&tag=b", "application/json; charset=utf-8", "", "")

	f.Fuzz(func(t *testing.T, method, path, rawQuery, contentType, rawBody, replayValue string) {
		request := &http.Request{
			Method: limitIdempotencyFuzzString(method, 32),
			URL: &url.URL{
				Path:     limitIdempotencyFuzzString(path, 1024),
				RawQuery: limitIdempotencyFuzzString(rawQuery, 1024),
			},
			Header: http.Header{"Content-Type": []string{limitIdempotencyFuzzString(contentType, 256)}},
		}
		body := []byte(limitIdempotencyFuzzString(rawBody, 4096))

		first, err := DefaultHash(request, body)
		if err != nil {
			t.Fatalf("DefaultHash returned an error: %v", err)
		}
		if len(first) != sha256.Size*2 {
			t.Fatalf("DefaultHash length = %d, want %d", len(first), sha256.Size*2)
		}
		if _, err := hex.DecodeString(first); err != nil {
			t.Fatalf("DefaultHash returned non-hex output: %v", err)
		}
		second, err := DefaultHash(request, body)
		if err != nil || first != second {
			t.Fatalf("DefaultHash must be deterministic: equal=%t err=%v", first == second, err)
		}

		headers := http.Header{
			"Idempotency-Key":      []string{"raw-key"},
			"Idempotency-Replayed": []string{"true"},
			"X-Replay-Token":       []string{limitIdempotencyFuzzString(replayValue, 1024)},
		}
		stripped := stripReplayOwnedHeaders(headers, "X-Replay-Token")
		for _, denied := range []string{"Idempotency-Key", "Idempotency-Replayed", "X-Replay-Token"} {
			if values := stripped.Values(denied); len(values) != 0 {
				t.Fatalf("replay metadata retained denied header %q", denied)
			}
		}
		if headers.Get("Idempotency-Key") != "raw-key" {
			t.Fatal("replay metadata filtering mutated the source headers")
		}
	})
}

func limitIdempotencyFuzzString(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
func TestIdempotencyRequireKeyRejectsMissingKey(t *testing.T) {
	mem := newMemoryStore()
	var events []OutcomeEvent
	called := false
	mw, err := New(Options{
		Store:      mem,
		RequireKey: true,
		OnOutcome: func(_ context.Context, event OutcomeEvent) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("handler was called without an idempotency key")
	}
	if !strings.Contains(rec.Body.String(), "idempotency key is required") {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if len(events) != 1 || events[0].Outcome != IdempotencyOutcomeMissingKey || events[0].Status != http.StatusBadRequest {
		t.Fatalf("events = %#v", events)
	}
}

func TestIdempotencyEmitsLowCardinalityOutcomeEvents(t *testing.T) {
	mem := newMemoryStore()
	var events []OutcomeEvent
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
		OnOutcome: func(_ context.Context, event OutcomeEvent) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"ok": "true"})
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge/123?token=secret", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", "key-outcome")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first status = %d", rec.Code)
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge/123?token=secret", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", "key-outcome")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("replay status = %d", rec.Code)
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge/123?token=secret", strings.NewReader("beta"))
	req.Header.Set("Idempotency-Key", "key-outcome")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("conflict status = %d", rec.Code)
	}

	if len(events) != 3 {
		t.Fatalf("events = %#v", events)
	}
	if events[0].Outcome != IdempotencyOutcomeCompletedStored || events[0].Method != http.MethodPost || events[0].Status != http.StatusCreated {
		t.Fatalf("first event = %#v", events[0])
	}
	if events[1].Outcome != IdempotencyOutcomeReplayed || events[1].Status != http.StatusCreated {
		t.Fatalf("replay event = %#v", events[1])
	}
	if events[2].Outcome != IdempotencyOutcomeConflict || events[2].Status != http.StatusConflict {
		t.Fatalf("conflict event = %#v", events[2])
	}

	labels := events[0].MetricLabels()
	for _, key := range []string{"method", "store_class", "outcome", "status_class"} {
		if labels[key] == "" {
			t.Fatalf("metric labels missing %q: %#v", key, labels)
		}
	}
	for _, forbidden := range []string{"path", "key", "tenant_id", "request_id", "error"} {
		if _, ok := labels[forbidden]; ok {
			t.Fatalf("metric labels include forbidden high-cardinality label %q: %#v", forbidden, labels)
		}
	}
}

func TestIdempotencyRecoversLegacyTokenlessInflightRecordFromMemoryStore(t *testing.T) {
	const key = "key-legacy-memory-recovery"
	now := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	mem := newMemoryStore()
	if err := mem.Save(context.Background(), key, Record{
		State:       StateInFlight,
		RequestHash: "legacy-hash",
		CreatedAt:   now.Add(-15 * time.Minute),
	}, 24*time.Hour); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	calls := 0
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
		InFlightTTL:  10 * time.Minute,
		Clock:        fixedClock{now: now},
		HashFunc: func(*http.Request, []byte) (string, error) {
			return "legacy-hash", nil
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"ok": "memory"})
	}))

	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req1.Header.Set("Idempotency-Key", key)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected first request to recover legacy and succeed, got %d", rec1.Code)
	}

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req2.Header.Set("Idempotency-Key", key)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected replay after recovery, got %d", rec2.Code)
	}
	if calls != 1 {
		t.Fatalf("expected handler to execute once, got %d", calls)
	}
}

func TestIdempotencyEmitsLegacyCompatibilityEventsWithoutStoreCallbacks(t *testing.T) {
	const key = "key-legacy-no-store-callback"
	now := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	mem := newMemoryStore()
	if err := mem.Save(context.Background(), key, Record{
		State:       StateInFlight,
		RequestHash: "legacy-hash",
		CreatedAt:   now.Add(-15 * time.Minute),
	}, 24*time.Hour); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	events := make([]LegacyInFlightCompatibilityEvent, 0, 2)
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
		InFlightTTL:  10 * time.Minute,
		Clock:        fixedClock{now: now},
		HashFunc: func(*http.Request, []byte) (string, error) {
			return "legacy-hash", nil
		},
		OnLegacyInFlightCompatibility: func(_ context.Context, event LegacyInFlightCompatibilityEvent) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"ok": "memory"})
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected first request to recover legacy and succeed, got %d", rec.Code)
	}
	if len(events) != 2 {
		t.Fatalf("expected entered+recovered events, got %d", len(events))
	}
	if got := events[0].Outcome; got != LegacyInFlightCompatibilityEntered {
		t.Fatalf("expected first compatibility event = %q, got %q", LegacyInFlightCompatibilityEntered, got)
	}
	if got := events[1].Outcome; got != LegacyInFlightCompatibilityRecovered {
		t.Fatalf("expected second compatibility event = %q, got %q", LegacyInFlightCompatibilityRecovered, got)
	}
	if events[0].Method != http.MethodPost {
		t.Fatalf("expected compatibility event method %q, got %q", http.MethodPost, events[0].Method)
	}
	if events[0].Path != "/charge" {
		t.Fatalf("expected compatibility event path %q, got %q", "/charge", events[0].Path)
	}
	if events[0].StoreType == "" {
		t.Fatal("expected compatibility event to include store type")
	}
}

func TestLegacyCompatibilityAsyncSinkDropsWhenQueueIsFull(t *testing.T) {
	block := make(chan struct{})
	log := &captureLogger{}
	sink := newLegacyInFlightCompatibilityAsyncSink(LegacyInFlightCompatibilitySinkFunc(func(context.Context, LegacyInFlightCompatibilityEvent) {
		<-block
	}), log)

	for i := 0; i < legacyInFlightCompatibilityAsyncQueueSize*2; i++ {
		sink.Emit(context.Background(), LegacyInFlightCompatibilityEvent{
			Method:  http.MethodPost,
			Path:    "/charge",
			Outcome: LegacyInFlightCompatibilityEntered,
		})
	}

	if sink.dropped.Load() == 0 {
		t.Fatal("expected bounded async sink to drop events when queue is full")
	}
	if log.WarnCount() == 0 {
		t.Fatal("expected drop warning")
	}
	if got := log.LastWarnMessage(); !strings.Contains(got, "telemetry dropped") {
		t.Fatalf("expected drop warning, got %q", got)
	}
	close(block)
}

func TestLegacyCompatibilityAsyncSinkRecoversFromSinkPanic(t *testing.T) {
	sink := newLegacyInFlightCompatibilityAsyncSink(LegacyInFlightCompatibilitySinkFunc(func(context.Context, LegacyInFlightCompatibilityEvent) {
		panic("boom")
	}), &captureLogger{})

	sink.Emit(context.Background(), LegacyInFlightCompatibilityEvent{
		Method:  http.MethodPost,
		Path:    "/charge",
		Outcome: LegacyInFlightCompatibilityEntered,
	})
}

func TestIdempotencyWarnsOnLegacyInflightClockSkewRisk(t *testing.T) {
	now := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	mem := newMemoryStore()
	key := "key-legacy-clock-skew"
	if err := mem.Save(context.Background(), key, Record{
		State:       StateInFlight,
		RequestHash: "legacy-hash",
		CreatedAt:   now.Add(30 * time.Second),
	}, 24*time.Hour); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	log := &captureLogger{}
	events := make([]LegacyInFlightCompatibilityEvent, 0, 1)
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
		InFlightTTL:  10 * time.Minute,
		Clock:        fixedClock{now: now},
		HashFunc: func(*http.Request, []byte) (string, error) {
			return "legacy-hash", nil
		},
		Logger: log,
		OnLegacyInFlightCompatibility: func(_ context.Context, event LegacyInFlightCompatibilityEvent) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", key)
	rec := httptest.NewRecorder()
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"ok": "memory"})
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 while clock-skew recovery is suppressed, got %d", rec.Code)
	}
	if log.WarnCount() != 1 {
		t.Fatalf("expected one clock-skew warning, got %d", log.WarnCount())
	}
	if got := log.LastWarnMessage(); got == "" || !strings.Contains(got, "clock-skew-sensitive") {
		t.Fatalf("expected clock-skew warning message, got %q", got)
	}
	if len(events) != 0 {
		t.Fatalf("expected no middleware compatibility event when legacy inflight is rejected for clock skew, got %d", len(events))
	}
}

func TestIdempotencyEmitsDefaultLegacyCompatibilitySinkWithHashedKey(t *testing.T) {
	const key = "key-legacy-no-store-callback"
	now := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	mem := newMemoryStore()
	if err := mem.Save(context.Background(), key, Record{
		State:       StateInFlight,
		RequestHash: "legacy-hash",
		CreatedAt:   now.Add(-15 * time.Minute),
	}, 24*time.Hour); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	log := &captureLogger{}
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
		InFlightTTL:  10 * time.Minute,
		Clock:        fixedClock{now: now},
		HashFunc: func(_ *http.Request, _ []byte) (string, error) {
			return "legacy-hash", nil
		},
		Logger: log,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"ok": "memory"})
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected first request to recover legacy and succeed, got %d", rec.Code)
	}

	warns := log.WarnValues()
	if len(warns) != 2 {
		t.Fatalf("expected two compatibility warnings from default sink, got %d", len(warns))
	}
	entered := warnKeyValue(warns[0])
	recovered := warnKeyValue(warns[1])
	if got := entered["outcome"]; got != string(LegacyInFlightCompatibilityEntered) {
		t.Fatalf("expected first compatibility outcome %q, got %q", LegacyInFlightCompatibilityEntered, got)
	}
	if got := recovered["outcome"]; got != string(LegacyInFlightCompatibilityRecovered) {
		t.Fatalf("expected second compatibility outcome %q, got %q", LegacyInFlightCompatibilityRecovered, got)
	}
	expectedKey := hashValue(key)
	if entered["key"] != expectedKey {
		t.Fatalf("expected hashed default key for entered event, got %q", entered["key"])
	}
	if recovered["key"] != expectedKey {
		t.Fatalf("expected hashed default key for recovered event, got %q", recovered["key"])
	}
}

func TestIdempotencyCanEmitRawLegacyCompatibilityKeyWhenOptedIn(t *testing.T) {
	const key = "key-legacy-raw-key"
	now := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	mem := newMemoryStore()
	if err := mem.Save(context.Background(), key, Record{
		State:       StateInFlight,
		RequestHash: "legacy-hash",
		CreatedAt:   now.Add(-15 * time.Minute),
	}, 24*time.Hour); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	events := make([]LegacyInFlightCompatibilityEvent, 0, 2)
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
		InFlightTTL:  10 * time.Minute,
		Clock:        fixedClock{now: now},
		HashFunc: func(_ *http.Request, _ []byte) (string, error) {
			return "legacy-hash", nil
		},
		LegacyInFlightCompatibilityRawKey: true,
		OnLegacyInFlightCompatibility: func(_ context.Context, event LegacyInFlightCompatibilityEvent) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"ok": "memory"})
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected first request to recover legacy and succeed, got %d", rec.Code)
	}
	if len(events) != 2 {
		t.Fatalf("expected entered+recovered events, got %d", len(events))
	}
	if events[0].Key != key {
		t.Fatalf("expected raw compatibility key when opted in, got %q", events[0].Key)
	}
	if events[1].Key != key {
		t.Fatalf("expected raw compatibility key when opted in, got %q", events[1].Key)
	}
}

func TestIdempotencyWarnsOnClockSkewPreflightInStartupWhenNotStrict(t *testing.T) {
	log := &captureLogger{}
	clock := &sequenceClock{timestamps: []time.Time{
		time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC),
		time.Date(2026, time.April, 30, 9, 59, 59, 999000000, time.UTC),
	}}
	_, err := New(Options{
		Store:        newMemoryStore(),
		MaxBodyBytes: 1024,
		Clock:        clock,
		Logger:       log,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	if log.WarnCount() != 1 {
		t.Fatalf("expected startup warning on skew risk, got %d", log.WarnCount())
	}
	if got := log.LastWarnMessage(); got == "" || !strings.Contains(got, "clock preflight risk") {
		t.Fatalf("expected preflight risk warning, got %q", got)
	}
}

func TestIdempotencyWarnsOnClockSkewPreflightRiskMatrix(t *testing.T) {
	log := &captureLogger{}
	base := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	scenarios := []struct {
		name      string
		clock     ports.Clock
		strict    bool
		wantWarn  int
		wantError bool
	}{
		{
			name:     "normal increasing clock",
			clock:    &sequenceClock{timestamps: []time.Time{base, base.Add(5 * time.Second)}},
			wantWarn: 0,
		},
		{
			name:     "fixed-resolution duplicate reads",
			clock:    &sequenceClock{timestamps: []time.Time{base, base}},
			wantWarn: 0,
		},
		{
			name:      "strict mode rejects slight backward jitter",
			clock:     &sequenceClock{timestamps: []time.Time{base, base.Add(-50 * time.Millisecond)}},
			strict:    true,
			wantWarn:  1,
			wantError: true,
		},
		{
			name:      "advisory mode tolerates fixed backward jitter once",
			clock:     &sequenceClock{timestamps: []time.Time{base.Add(250 * time.Millisecond), base}},
			wantWarn:  1,
			wantError: false,
		},
		{
			name:      "strict mode rejects hard backward-step clock",
			clock:     &sequenceClock{timestamps: []time.Time{base, base.Add(-2 * time.Minute)}},
			strict:    true,
			wantWarn:  1,
			wantError: true,
		},
		{
			name:      "advisory mode flags hard backward-step clock",
			clock:     &sequenceClock{timestamps: []time.Time{base, base.Add(-2 * time.Minute)}},
			wantWarn:  1,
			wantError: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			log.messages = nil
			log.warnings = nil
			_, err := New(Options{
				Store:                            newMemoryStore(),
				MaxBodyBytes:                     1024,
				Clock:                            scenario.clock,
				Logger:                           log,
				FailOnInFlightClockSkewPreflight: scenario.strict,
			})
			if scenario.wantError {
				if err == nil {
					t.Fatalf("expected startup fail-fast on preflight risk in %q", scenario.name)
				}
				if !errors.Is(err, ErrLegacyInFlightClockSkewPreflightRisk) {
					t.Fatalf("expected wrapped preflight risk error in %q, got %v", scenario.name, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected startup error in %q: %v", scenario.name, err)
			}
			if got := log.WarnCount(); got != scenario.wantWarn {
				t.Fatalf("expected %d warning(s) in %q, got %d", scenario.wantWarn, scenario.name, got)
			}
		})
	}
}

func TestIdempotencyCanFailStartupOnClockSkewPreflightWhenStrictEnabled(t *testing.T) {
	log := &captureLogger{}
	clock := &sequenceClock{timestamps: []time.Time{
		time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC),
		time.Date(2026, time.April, 30, 9, 59, 59, 999000000, time.UTC),
	}}
	_, err := New(Options{
		Store:                            newMemoryStore(),
		MaxBodyBytes:                     1024,
		InFlightTTL:                      2 * time.Minute,
		Clock:                            clock,
		Logger:                           log,
		FailOnInFlightClockSkewPreflight: true,
	})
	if err == nil {
		t.Fatal("expected startup fail-fast on clock preflight")
	}
	if !errors.Is(err, ErrLegacyInFlightClockSkewPreflightRisk) {
		t.Fatalf("expected ErrLegacyInFlightClockSkewPreflightRisk, got %v", err)
	}
	if !strings.Contains(err.Error(), "startup clock moved backwards across preflight window") {
		t.Fatalf("expected remediation-oriented startup error, got %v", err)
	}
	if log.WarnCount() != 1 {
		t.Fatalf("expected warning before startup fail-fast, got %d", log.WarnCount())
	}
}

func TestIdempotencyDefaultLegacyCompatibilitySinkMatchesExplicitSinkContract(t *testing.T) {
	now := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	seedLegacyInflight := func(key string) *memoryStore {
		mem := newMemoryStore()
		if err := mem.Save(context.Background(), key, Record{
			State:       StateInFlight,
			RequestHash: "legacy-hash",
			CreatedAt:   now.Add(-15 * time.Minute),
		}, 24*time.Hour); err != nil {
			t.Fatalf("seed store: %v", err)
		}
		return mem
	}

	runWithSink := func(sink LegacyInFlightCompatibilityEventSink, log *captureLogger) {
		key := "key-legacy-contract-equivalence"
		mem := seedLegacyInflight(key)
		options := Options{
			Store:        mem,
			MaxBodyBytes: 1024,
			InFlightTTL:  10 * time.Minute,
			Clock:        fixedClock{now: now},
			HashFunc: func(_ *http.Request, _ []byte) (string, error) {
				return "legacy-hash", nil
			},
			Logger: log,
		}
		if sink != nil {
			options.LegacyInFlightCompatibilitySink = sink
		}

		mw, err := New(options)
		if err != nil {
			t.Fatalf("new middleware: %v", err)
		}
		mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteJSON(w, http.StatusCreated, map[string]string{"ok": "memory"})
		})).ServeHTTP(httptest.NewRecorder(), func() *http.Request {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
			req.Header.Set("Idempotency-Key", key)
			return req
		}())
	}

	loggerDefault := &captureLogger{}
	runWithSink(nil, loggerDefault)

	sinkEvents := make([]LegacyInFlightCompatibilityEvent, 0, 2)
	runWithSink(
		LegacyInFlightCompatibilitySinkFunc(func(_ context.Context, event LegacyInFlightCompatibilityEvent) {
			sinkEvents = append(sinkEvents, event)
		}),
		nil,
	)
	if len(sinkEvents) != 2 {
		t.Fatalf("expected two events from explicit compatibility sink, got %d", len(sinkEvents))
	}

	compatEventsFromLogger := func(log *captureLogger) []LegacyInFlightCompatibilityEvent {
		warns := log.WarnValues()
		out := make([]LegacyInFlightCompatibilityEvent, 0, len(warns))
		for _, warn := range warns {
			fields := warnKeyValue(warn)
			method, ok := fields["method"].(string)
			if !ok || method == "" {
				continue
			}
			out = append(out, LegacyInFlightCompatibilityEvent{
				Method:    method,
				Path:      warnValueAsString(fields, "path"),
				Key:       warnValueAsString(fields, "key"),
				StoreType: warnValueAsString(fields, "store_type"),
				Outcome:   LegacyInFlightCompatibilityEventName(warnValueAsString(fields, "outcome")),
				Error:     warnValueAsString(fields, "error"),
			})
		}
		return out
	}

	defaultSinkEvents := compatEventsFromLogger(loggerDefault)
	if len(defaultSinkEvents) != 2 {
		t.Fatalf("expected two default logger compatibility events, got %d", len(defaultSinkEvents))
	}
	if len(defaultSinkEvents) != len(sinkEvents) {
		t.Fatalf("expected matched default and explicit event counts")
	}

	for i := range sinkEvents {
		if defaultSinkEvents[i].Method != sinkEvents[i].Method {
			t.Fatalf("expected method match at index %d: default=%q explicit=%q", i, defaultSinkEvents[i].Method, sinkEvents[i].Method)
		}
		if defaultSinkEvents[i].Path != sinkEvents[i].Path {
			t.Fatalf("expected path match at index %d: default=%q explicit=%q", i, defaultSinkEvents[i].Path, sinkEvents[i].Path)
		}
		if defaultSinkEvents[i].StoreType != sinkEvents[i].StoreType {
			t.Fatalf("expected store_type match at index %d", i)
		}
		if defaultSinkEvents[i].Outcome != sinkEvents[i].Outcome {
			t.Fatalf("expected outcome match at index %d: default=%q explicit=%q", i, defaultSinkEvents[i].Outcome, sinkEvents[i].Outcome)
		}
		if defaultSinkEvents[i].Key != sinkEvents[i].Key {
			t.Fatalf("expected key match at index %d: default=%q explicit=%q", i, defaultSinkEvents[i].Key, sinkEvents[i].Key)
		}
		if defaultSinkEvents[i].Error != sinkEvents[i].Error {
			t.Fatalf("expected error match at index %d: default=%q explicit=%q", i, defaultSinkEvents[i].Error, sinkEvents[i].Error)
		}
	}
}

func TestIdempotencyCanEmitLegacyCompatibilityMetricSink(t *testing.T) {
	const key = "key-legacy-metric-sink"
	now := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	mem := newMemoryStore()
	if err := mem.Save(context.Background(), key, Record{
		State:       StateInFlight,
		RequestHash: "legacy-hash",
		CreatedAt:   now.Add(-15 * time.Minute),
	}, 24*time.Hour); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	sinkEvents := make([]LegacyInFlightCompatibilityMetricLabels, 0, 2)
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
		InFlightTTL:  10 * time.Minute,
		Clock:        fixedClock{now: now},
		HashFunc: func(_ *http.Request, _ []byte) (string, error) {
			return "legacy-hash", nil
		},
		LegacyInFlightCompatibilityMetricSink: LegacyInFlightCompatibilityMetricSinkFunc(func(_ context.Context, labels LegacyInFlightCompatibilityMetricLabels) {
			copied := make(LegacyInFlightCompatibilityMetricLabels, len(labels))
			for key, value := range labels {
				copied[key] = value
			}
			sinkEvents = append(sinkEvents, copied)
		}),
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"ok": "memory"})
	}))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected first request to recover legacy and succeed, got %d", rec.Code)
	}
	if len(sinkEvents) != 2 {
		t.Fatalf("expected two metric events, got %d", len(sinkEvents))
	}
	if sinkEvents[0][legacyInFlightCompatibilityEventMethodLabel] != http.MethodPost {
		t.Fatalf("expected metric method %q, got %q", http.MethodPost, sinkEvents[0][legacyInFlightCompatibilityEventMethodLabel])
	}
	if sinkEvents[0][legacyInFlightCompatibilityEventStoreClassLabel] == "" {
		t.Fatal("expected metric store_class")
	}
	if sinkEvents[0][legacyInFlightCompatibilityEventOutcomeLabel] != string(LegacyInFlightCompatibilityEntered) {
		t.Fatalf("expected entered outcome, got %q", sinkEvents[0][legacyInFlightCompatibilityEventOutcomeLabel])
	}
	if sinkEvents[1][legacyInFlightCompatibilityEventOutcomeLabel] != string(LegacyInFlightCompatibilityRecovered) {
		t.Fatalf("expected recovered outcome, got %q", sinkEvents[1][legacyInFlightCompatibilityEventOutcomeLabel])
	}
	for _, forbidden := range []string{
		legacyInFlightCompatibilityEventPathLabel,
		legacyInFlightCompatibilityEventKeyLabel,
		legacyInFlightCompatibilityEventErrorLabel,
	} {
		if _, ok := sinkEvents[0][forbidden]; ok {
			t.Fatalf("metric labels must not include unbounded label %q", forbidden)
		}
	}
}

func TestLegacyCompatibilityMetricLabelsStayBounded(t *testing.T) {
	event := LegacyInFlightCompatibilityEvent{
		Method:    "PROPFIND",
		Path:      "/tenant/acme/users/123?token=secret",
		Key:       "raw-or-hashed-key",
		StoreType: "github.com/example/customTenantStore",
		Outcome:   LegacyInFlightCompatibilityEventName("tenant-controlled-outcome"),
		Error:     "user supplied error with account id 123",
	}

	labels := event.MetricLabels()
	want := map[string]string{
		legacyInFlightCompatibilityEventMethodLabel:     "OTHER",
		legacyInFlightCompatibilityEventStoreClassLabel: "custom",
		legacyInFlightCompatibilityEventOutcomeLabel:    "unknown",
	}
	if len(labels) != len(want) {
		t.Fatalf("expected only bounded metric labels %v, got %v", want, labels)
	}
	for key, value := range want {
		if labels[key] != value {
			t.Fatalf("expected metric label %s=%q, got %q", key, value, labels[key])
		}
	}
	for _, forbidden := range []string{
		legacyInFlightCompatibilityEventPathLabel,
		legacyInFlightCompatibilityEventKeyLabel,
		legacyInFlightCompatibilityEventErrorLabel,
		"request_id",
		"tenant_id",
	} {
		if _, ok := labels[forbidden]; ok {
			t.Fatalf("metric labels must not include unbounded label %q", forbidden)
		}
	}
}

func TestIdempotencyCanSampleCompatibilitySinkTrafficInHighVolumeMode(t *testing.T) {
	const requests = 8
	now := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	const sampleEvery = 4
	mem := newMemoryStore()

	for i := 0; i < requests; i++ {
		key := "key-legacy-volume-" + strconv.Itoa(i)
		if err := mem.Save(context.Background(), key, Record{
			State:       StateInFlight,
			RequestHash: "legacy-hash",
			CreatedAt:   now.Add(-15 * time.Minute),
		}, 24*time.Hour); err != nil {
			t.Fatalf("seed store: %v", err)
		}
	}

	sinkEvents := make([]LegacyInFlightCompatibilityMetricLabels, 0, requests*2)
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
		InFlightTTL:  10 * time.Minute,
		Clock:        fixedClock{now: now},
		HashFunc: func(_ *http.Request, _ []byte) (string, error) {
			return "legacy-hash", nil
		},
		LegacyInFlightCompatibilitySampleEvery: sampleEvery,
		LegacyInFlightCompatibilityMetricSink: LegacyInFlightCompatibilityMetricSinkFunc(func(_ context.Context, labels LegacyInFlightCompatibilityMetricLabels) {
			copied := make(LegacyInFlightCompatibilityMetricLabels, len(labels))
			for key, value := range labels {
				copied[key] = value
			}
			sinkEvents = append(sinkEvents, copied)
		}),
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"ok": "memory"})
	}))

	for i := 0; i < requests; i++ {
		key := "key-legacy-volume-" + strconv.Itoa(i)
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
		req.Header.Set("Idempotency-Key", key)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected request %d to recover legacy and succeed, got %d", i+1, rec.Code)
		}
	}

	expectedEvents := (requests * 2) / sampleEvery
	if len(sinkEvents) != expectedEvents {
		t.Fatalf("expected %d sampled metric events, got %d", expectedEvents, len(sinkEvents))
	}
	for idx, event := range sinkEvents {
		if event[legacyInFlightCompatibilityEventMethodLabel] != http.MethodPost {
			t.Fatalf("sampled event %d expected post method, got %q", idx, event[legacyInFlightCompatibilityEventMethodLabel])
		}
		if _, ok := event[legacyInFlightCompatibilityEventPathLabel]; ok {
			t.Fatalf("sampled event %d included unbounded path metric label", idx)
		}
	}
}

func TestIdempotencyLegacyCompatibilityAsyncSinkSkipsRequestBackpressure(t *testing.T) {
	const key = "key-legacy-async-backpressure"
	now := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	mem := newMemoryStore()
	if err := mem.Save(context.Background(), key, Record{
		State:       StateInFlight,
		RequestHash: "legacy-hash",
		CreatedAt:   now.Add(-15 * time.Minute),
	}, 24*time.Hour); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	block := make(chan struct{})
	received := make(chan struct{}, 1)
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
		InFlightTTL:  10 * time.Minute,
		Clock:        fixedClock{now: now},
		HashFunc: func(_ *http.Request, _ []byte) (string, error) {
			return "legacy-hash", nil
		},
		LegacyInFlightCompatibilityAsync: true,
		LegacyInFlightCompatibilitySink: LegacyInFlightCompatibilitySinkFunc(func(_ context.Context, _ LegacyInFlightCompatibilityEvent) {
			<-block
			received <- struct{}{}
		}),
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", key)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteJSON(w, http.StatusCreated, map[string]string{"ok": "memory"})
		})).ServeHTTP(rec, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected async sink to avoid request backpressure when callback blocks")
	}

	close(block)
	select {
	case <-received:
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected request to succeed, got %d", rec.Code)
		}
	case <-time.After(time.Second):
		t.Fatal("expected at least one async callback emission")
	}
}

func TestIdempotencyLegacyCompatibilitySyncSinkCanApplyRequestBackpressure(t *testing.T) {
	const key = "key-legacy-sync-backpressure"
	now := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	mem := newMemoryStore()
	if err := mem.Save(context.Background(), key, Record{
		State:       StateInFlight,
		RequestHash: "legacy-hash",
		CreatedAt:   now.Add(-15 * time.Minute),
	}, 24*time.Hour); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	block := make(chan struct{})
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
		InFlightTTL:  10 * time.Minute,
		Clock:        fixedClock{now: now},
		HashFunc: func(_ *http.Request, _ []byte) (string, error) {
			return "legacy-hash", nil
		},
		LegacyInFlightCompatibilitySink: LegacyInFlightCompatibilitySinkFunc(func(_ context.Context, _ LegacyInFlightCompatibilityEvent) {
			<-block
		}),
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", key)
	rec := httptest.NewRecorder()
	done := make(chan struct{})

	go func() {
		mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteJSON(w, http.StatusCreated, map[string]string{"ok": "memory"})
		})).ServeHTTP(rec, req)
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("expected blocking sink to delay request completion while callback is blocked")
	case <-time.After(20 * time.Millisecond):
	}
	close(block)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected request to complete after sync callback unblocks")
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected request to succeed, got %d", rec.Code)
	}
}

func TestIdempotencyLegacyCompatibilitySinkPanicsAreRecovered(t *testing.T) {
	const key = "key-legacy-compat-sink-panic"
	now := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	mem := newMemoryStore()
	if err := mem.Save(context.Background(), key, Record{
		State:       StateInFlight,
		RequestHash: "legacy-hash",
		CreatedAt:   now.Add(-15 * time.Minute),
	}, 24*time.Hour); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
		InFlightTTL:  10 * time.Minute,
		Clock:        fixedClock{now: now},
		HashFunc: func(_ *http.Request, _ []byte) (string, error) {
			return "legacy-hash", nil
		},
		LegacyInFlightCompatibilitySink: LegacyInFlightCompatibilitySinkFunc(func(_ context.Context, _ LegacyInFlightCompatibilityEvent) {
			panic("compatibility sink failure")
		}),
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", key)
	rec := httptest.NewRecorder()
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"ok": "memory"})
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected request to succeed despite panic in compatibility sink, got %d", rec.Code)
	}
}

func TestIdempotencyLegacyCompatibilityAsyncSampledPanicsDoNotBlockRepeatedFallbacks(t *testing.T) {
	const requests = 6
	const sampleEvery = 3
	const expectedSampledEvents = (requests * 2) / sampleEvery
	now := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	mem := newMemoryStore()

	for i := 0; i < requests; i++ {
		key := "key-legacy-async-sampled-panic-" + strconv.Itoa(i)
		if err := mem.Save(context.Background(), key, Record{
			State:       StateInFlight,
			RequestHash: "legacy-hash",
			CreatedAt:   now.Add(-15 * time.Minute),
		}, 24*time.Hour); err != nil {
			t.Fatalf("seed store: %v", err)
		}
	}

	releaseSink := make(chan struct{})
	events := make(chan LegacyInFlightCompatibilityEventName, requests*2)
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
		InFlightTTL:  10 * time.Minute,
		Clock:        fixedClock{now: now},
		HashFunc: func(_ *http.Request, _ []byte) (string, error) {
			return "legacy-hash", nil
		},
		LegacyInFlightCompatibilityAsync:       true,
		LegacyInFlightCompatibilitySampleEvery: sampleEvery,
		LegacyInFlightCompatibilitySink: LegacyInFlightCompatibilitySinkFunc(func(_ context.Context, event LegacyInFlightCompatibilityEvent) {
			events <- event.Outcome
			<-releaseSink
			panic("sampled async compatibility sink failure")
		}),
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"ok": "memory"})
	}))

	statuses := make(chan int, requests)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < requests; i++ {
			key := "key-legacy-async-sampled-panic-" + strconv.Itoa(i)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
			req.Header.Set("Idempotency-Key", key)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			statuses <- rec.Code
		}
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		close(releaseSink)
		t.Fatal("expected sampled async compatibility telemetry not to block repeated fallback requests")
	}
	for i := 0; i < requests; i++ {
		if code := <-statuses; code != http.StatusCreated {
			close(releaseSink)
			t.Fatalf("expected request %d to recover legacy and succeed, got %d", i+1, code)
		}
	}

	var received []LegacyInFlightCompatibilityEventName
	for i := 0; i < expectedSampledEvents; i++ {
		select {
		case outcome := <-events:
			received = append(received, outcome)
		case <-time.After(time.Second):
			close(releaseSink)
			t.Fatalf("expected %d sampled async events, got %d", expectedSampledEvents, len(received))
		}
	}
	close(releaseSink)
	if len(received) != expectedSampledEvents {
		t.Fatalf("expected %d sampled async events, got %d", expectedSampledEvents, len(received))
	}
}

func TestIdempotencyClockSkewPreflightWarnsAndCanFailFast(t *testing.T) {
	first := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	second := first.Add(-time.Second)

	log := &captureLogger{}
	mw, err := New(Options{
		Store:        newMemoryStore(),
		MaxBodyBytes: 1024,
		Clock:        &sequenceClock{timestamps: []time.Time{first, second}},
		Logger:       log,
	})
	if err != nil {
		t.Fatalf("new middleware warning-only: %v", err)
	}
	if mw == nil {
		t.Fatal("expected middleware to be created in warning-only mode")
	}
	if log.WarnCount() != 1 {
		t.Fatalf("expected one clock preflight warning, got %d", log.WarnCount())
	}
	if got := log.LastWarnMessage(); got == "" || !strings.Contains(got, "clock preflight risk detected") {
		t.Fatalf("expected clock preflight warning, got %q", got)
	}

	strictLog := &captureLogger{}
	_, err = New(Options{
		Store:                            newMemoryStore(),
		MaxBodyBytes:                     1024,
		Clock:                            &sequenceClock{timestamps: []time.Time{first, second}},
		Logger:                           strictLog,
		FailOnInFlightClockSkewPreflight: true,
	})
	if !errors.Is(err, ErrLegacyInFlightClockSkewPreflightRisk) {
		t.Fatalf("strict clock preflight error = %v, want %v", err, ErrLegacyInFlightClockSkewPreflightRisk)
	}
	if strictLog.WarnCount() != 1 {
		t.Fatalf("expected strict mode to log before failing, got %d warnings", strictLog.WarnCount())
	}
}

func TestIdempotencyWarnsOnInFlightTTLMismatchAndCanFailFast(t *testing.T) {
	log := &captureLogger{}
	mw, err := New(Options{
		Store:        newMemoryStore(),
		MaxBodyBytes: 1024,
		InFlightTTL:  2 * time.Minute,
		KnownInFlightTTLs: map[string]time.Duration{
			"billing-svc:v1":  4 * time.Minute,
			"payments-svc:v2": 2 * time.Minute,
		},
		Logger: log,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	if mw == nil {
		t.Fatal("expected middleware to be created with advisory TTL mismatch warning")
	}
	if log.WarnCount() != 1 {
		t.Fatalf("expected one advisory warning, got %d", log.WarnCount())
	}
	if got := log.LastWarnMessage(); got == "" || !strings.Contains(got, "mixed-version in-flight TTL mismatch detected") {
		t.Fatalf("expected TTL mismatch warning, got %q", got)
	}

	_, err = New(Options{
		Store:        newMemoryStore(),
		MaxBodyBytes: 1024,
		InFlightTTL:  2 * time.Minute,
		KnownInFlightTTLs: map[string]time.Duration{
			"billing-svc:v1":  4 * time.Minute,
			"payments-svc:v2": 2 * time.Minute,
		},
		Logger:                    log,
		FailOnInFlightTTLMismatch: true,
	})
	if err == nil {
		t.Fatal("expected fail-fast on configured in-flight TTL mismatch")
	}
	if log.WarnCount() < 2 {
		t.Fatalf("expected warning to be emitted even on fail-fast, got %d", log.WarnCount())
	}
}

func TestIdempotencyDoesNotWarnOnAlignedInFlightTTLs(t *testing.T) {
	log := &captureLogger{}
	_, err := New(Options{
		Store:        newMemoryStore(),
		MaxBodyBytes: 1024,
		InFlightTTL:  3 * time.Minute,
		KnownInFlightTTLs: map[string]time.Duration{
			"billing-svc:v1":  3 * time.Minute,
			"payments-svc:v2": 3 * time.Minute,
		},
		Logger: log,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	if log.WarnCount() != 0 {
		t.Fatalf("expected no warning for aligned InFlightTTL values, got %d", log.WarnCount())
	}
}

func TestIdempotencyEmitsLegacyCompatibilityRejectionForTokenMismatch(t *testing.T) {
	mismatchStore := &legacyRecoveryErrorStore{
		record: Record{
			State:       StateInFlight,
			RequestHash: "legacy-hash",
			CreatedAt:   time.Date(2026, time.April, 30, 9, 0, 0, 0, time.UTC),
		},
		releaseErr: ErrLegacyInFlightTokenMismatch,
	}
	events := make([]LegacyInFlightCompatibilityEvent, 0, 2)

	mw, err := New(Options{
		Store:        mismatchStore,
		MaxBodyBytes: 1024,
		InFlightTTL:  10 * time.Minute,
		Clock:        fixedClock{now: time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)},
		HashFunc: func(_ *http.Request, _ []byte) (string, error) {
			return "legacy-hash", nil
		},
		OnLegacyInFlightCompatibility: func(_ context.Context, event LegacyInFlightCompatibilityEvent) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", "key-legacy-token-mismatch")
	rec := httptest.NewRecorder()
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expected recovery to fail before handler execution")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected mismatch scenario to be retriable, got %d", rec.Code)
	}
	if len(events) != 2 {
		t.Fatalf("expected entered+rejected outcomes, got %d", len(events))
	}
	if events[0].Outcome != LegacyInFlightCompatibilityEntered {
		t.Fatalf("expected entered outcome first, got %q", events[0].Outcome)
	}
	if events[1].Outcome != LegacyInFlightCompatibilityRejected {
		t.Fatalf("expected rejected outcome second, got %q", events[1].Outcome)
	}
	if events[1].Error != ErrLegacyInFlightTokenMismatch.Error() {
		t.Fatalf("expected ErrLegacyInFlightTokenMismatch event, got %q", events[1].Error)
	}
}

func TestIdempotencyEmitsLegacyCompatibilityUnknownOnUnexpectedReleaseError(t *testing.T) {
	unknownStore := &legacyRecoveryErrorStore{
		record: Record{
			State:       StateInFlight,
			RequestHash: "legacy-hash",
			CreatedAt:   time.Date(2026, time.April, 30, 9, 0, 0, 0, time.UTC),
		},
		releaseErr: errors.New("legacy state unavailable"),
	}
	events := make([]LegacyInFlightCompatibilityEvent, 0, 2)

	mw, err := New(Options{
		Store:        unknownStore,
		MaxBodyBytes: 1024,
		InFlightTTL:  10 * time.Minute,
		Clock:        fixedClock{now: time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)},
		HashFunc: func(_ *http.Request, _ []byte) (string, error) {
			return "legacy-hash", nil
		},
		OnLegacyInFlightCompatibility: func(_ context.Context, event LegacyInFlightCompatibilityEvent) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", "key-legacy-unexpected-error")
	rec := httptest.NewRecorder()
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expected unexpected-restore scenario to fail before handler execution")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected unknown scenario to be retriable, got %d", rec.Code)
	}
	if len(events) != 2 {
		t.Fatalf("expected entered+unknown outcomes, got %d", len(events))
	}
	if events[0].Outcome != LegacyInFlightCompatibilityEntered {
		t.Fatalf("expected entered outcome first, got %q", events[0].Outcome)
	}
	if events[1].Outcome != LegacyInFlightCompatibilityUnknown {
		t.Fatalf("expected unknown outcome second, got %q", events[1].Outcome)
	}
	if events[1].Error == "" {
		t.Fatal("expected unknown scenario to include error detail")
	}
}

func TestIdempotencyDoesNotRecoverFreshLegacyInflightBeforeTTL(t *testing.T) {
	const key = "key-legacy-fresh-not-recovered"
	now := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	mem := newMemoryStore()
	if err := mem.Save(context.Background(), key, Record{
		State:       StateInFlight,
		RequestHash: "legacy-hash",
		CreatedAt:   now.Add(-5 * time.Minute),
	}, 24*time.Hour); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	events := make([]LegacyInFlightCompatibilityEvent, 0, 1)
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
		InFlightTTL:  10 * time.Minute,
		Clock:        fixedClock{now: now},
		HashFunc: func(*http.Request, []byte) (string, error) {
			return "legacy-hash", nil
		},
		OnLegacyInFlightCompatibility: func(_ context.Context, event LegacyInFlightCompatibilityEvent) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expected legacy in-flight to remain active")
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 while stale TTL not reached, got %d", rec.Code)
	}
	if len(events) != 0 {
		t.Fatalf("expected no legacy compatibility telemetry events before expiry, got %d", len(events))
	}
}

func TestIdempotencyLegacyCompatibilityRecommendationsIncludeStartupChecksAndFallbackTelemetry(t *testing.T) {
	const key = "key-legacy-recommendation-check"
	log := &captureLogger{}
	now := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	mem := newMemoryStore()
	if err := mem.Save(context.Background(), key, Record{
		State:       StateInFlight,
		RequestHash: "legacy-hash",
		CreatedAt:   now.Add(-15 * time.Minute),
	}, 24*time.Hour); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	_, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
		InFlightTTL:  10 * time.Minute,
		Logger:       log,
		KnownInFlightTTLs: map[string]time.Duration{
			"billing-svc:v1": 8 * time.Minute,
		},
	})
	if err != nil {
		t.Fatalf("expected startup advisory to be non-blocking: %v", err)
	}
	if log.WarnCount() != 1 {
		t.Fatalf("expected startup check warning for recommendation mismatch, got %d", log.WarnCount())
	}

	events := make([]LegacyInFlightCompatibilityEvent, 0, 2)
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
		InFlightTTL:  10 * time.Minute,
		Clock:        fixedClock{now: now},
		HashFunc: func(*http.Request, []byte) (string, error) {
			return "legacy-hash", nil
		},
		Logger: log,
		OnLegacyInFlightCompatibility: func(_ context.Context, event LegacyInFlightCompatibilityEvent) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", key)
	rec := httptest.NewRecorder()
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"ok": "memory"})
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected recovery via compatibility path, got %d", rec.Code)
	}
	if len(events) != 2 {
		t.Fatalf("expected recommendation-check scenario emitted entered+recovered, got %d", len(events))
	}
	if events[0].Outcome != LegacyInFlightCompatibilityEntered {
		t.Fatalf("expected entered outcome first, got %q", events[0].Outcome)
	}
	if events[1].Outcome != LegacyInFlightCompatibilityRecovered {
		t.Fatalf("expected recovered outcome second, got %q", events[1].Outcome)
	}
}

func TestIdempotencyReplayIgnoresQueryParameterOrder(t *testing.T) {
	mem := newMemoryStore()
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var calls int
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		httpx.WriteJSON(w, http.StatusCreated, map[string]int{"calls": calls})
	}))

	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge?b=2&a=1", strings.NewReader("alpha"))
	req1.Header.Set("Idempotency-Key", "key-query-order")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected first request to succeed, got %d", rec1.Code)
	}

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge?a=1&b=2", strings.NewReader("alpha"))
	req2.Header.Set("Idempotency-Key", "key-query-order")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected replay for equivalent query ordering, got %d", rec2.Code)
	}
	if rec2.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatal("expected replay header on canonicalized query replay")
	}
	if calls != 1 {
		t.Fatalf("expected canonicalized replay to avoid second execution, got %d calls", calls)
	}
}

func TestIdempotencyPreservesMultiValueQueryOrder(t *testing.T) {
	mem := newMemoryStore()
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var calls int
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		httpx.WriteJSON(w, http.StatusCreated, map[string]int{"calls": calls})
	}))

	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge?tag=a&tag=b", strings.NewReader("alpha"))
	req1.Header.Set("Idempotency-Key", "key-query-values")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected first request to succeed, got %d", rec1.Code)
	}

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge?tag=b&tag=a", strings.NewReader("alpha"))
	req2.Header.Set("Idempotency-Key", "key-query-values")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected conflict for reordered multi-value query, got %d", rec2.Code)
	}
	if calls != 1 {
		t.Fatalf("expected reordered multi-value query not to execute twice, got %d calls", calls)
	}
}

func TestIdempotencyReplaysResponseAtExactBufferLimit(t *testing.T) {
	mem := newMemoryStore()
	mw, err := New(Options{
		Store:            mem,
		MaxBodyBytes:     1024,
		MaxResponseBytes: 4,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("four"))
	}))

	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req1.Header.Set("Idempotency-Key", "key-limit-exact")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected first request to succeed at exact limit, got %d", rec1.Code)
	}

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req2.Header.Set("Idempotency-Key", "key-limit-exact")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected replay at exact limit, got %d", rec2.Code)
	}
	if rec2.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("expected replay header at exact limit")
	}
	if rec2.Body.String() != "four" {
		t.Fatalf("expected exact-limit replay body, got %q", rec2.Body.String())
	}
}

func TestIdempotencyUsesCustomReplayHeaderName(t *testing.T) {
	mem := newMemoryStore()
	mw, err := New(Options{
		Store:            mem,
		MaxBodyBytes:     1024,
		MaxResponseBytes: 1024,
		ReplayHeaderName: "X-Idempotent-Replay",
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"body": "alpha"})
	}))

	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req1.Header.Set("Idempotency-Key", "key-custom-replay-header")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected first request to succeed, got %d", rec1.Code)
	}

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req2.Header.Set("Idempotency-Key", "key-custom-replay-header")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected replay to succeed, got %d", rec2.Code)
	}
	if rec2.Header().Get("X-Idempotent-Replay") != "true" {
		t.Fatalf("expected custom replay header to be set")
	}
	if rec2.Header().Get("Idempotency-Replayed") != "" {
		t.Fatalf("expected default replay header to remain unset")
	}
}

func TestIdempotencyReplayMetadataOverridesStoredHeaders(t *testing.T) {
	mem := newMemoryStore()
	mw, err := New(Options{
		Store:            mem,
		MaxBodyBytes:     1024,
		MaxResponseBytes: 1024,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Idempotency-Replayed", "false")
		w.Header().Set("Idempotency-Key", "handler-key")
		w.Header().Set("X-App-Header", "kept")
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"body": "alpha"})
	}))

	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req1.Header.Set("Idempotency-Key", "key-owned")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected first request to succeed, got %d", rec1.Code)
	}

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req2.Header.Set("Idempotency-Key", "key-owned")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected replay to succeed, got %d", rec2.Code)
	}
	if got := rec2.Header().Get("Idempotency-Replayed"); got != "true" {
		t.Fatalf("expected middleware replay header, got %q", got)
	}
	if got := rec2.Header().Get("Idempotency-Key"); got != "key-owned" {
		t.Fatalf("expected middleware key echo, got %q", got)
	}
	if got := rec2.Header().Get("X-App-Header"); got != "kept" {
		t.Fatalf("expected unrelated app header to be preserved, got %q", got)
	}
}

func TestIdempotencyCustomReplayMetadataOverridesStoredHeaders(t *testing.T) {
	mem := newMemoryStore()
	mw, err := New(Options{
		Store:            mem,
		MaxBodyBytes:     1024,
		MaxResponseBytes: 1024,
		ReplayHeaderName: "X-Idempotent-Replay",
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Idempotent-Replay", "false")
		w.Header().Set("Idempotency-Replayed", "false")
		w.Header().Set("Idempotency-Key", "handler-key")
		w.Header().Set("X-App-Header", "kept")
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"body": "alpha"})
	}))

	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req1.Header.Set("Idempotency-Key", "key-custom-owned")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected first request to succeed, got %d", rec1.Code)
	}

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req2.Header.Set("Idempotency-Key", "key-custom-owned")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected replay to succeed, got %d", rec2.Code)
	}
	if got := rec2.Header().Get("X-Idempotent-Replay"); got != "true" {
		t.Fatalf("expected middleware custom replay header, got %q", got)
	}
	if got := rec2.Header().Get("Idempotency-Replayed"); got != "" {
		t.Fatalf("expected default replay header to remain unset, got %q", got)
	}
	if got := rec2.Header().Get("Idempotency-Key"); got != "key-custom-owned" {
		t.Fatalf("expected middleware key echo, got %q", got)
	}
	if got := rec2.Header().Get("X-App-Header"); got != "kept" {
		t.Fatalf("expected unrelated app header to be preserved, got %q", got)
	}
}

func TestIdempotencyRejectsReplayAcrossDifferentActors(t *testing.T) {
	mem := newMemoryStore()
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var calls int
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		httpx.WriteJSON(w, http.StatusCreated, map[string]int{"calls": calls})
	}))

	req1 := httptest.NewRequestWithContext(
		authorization.WithActor(context.Background(), authorization.Actor{UserID: "user-1"}),
		http.MethodPost,
		"/charge",
		strings.NewReader("alpha"),
	)
	req1.Header.Set("Idempotency-Key", "actor-key")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected first request to succeed, got %d", rec1.Code)
	}

	req2 := httptest.NewRequestWithContext(
		authorization.WithActor(context.Background(), authorization.Actor{UserID: "user-2"}),
		http.MethodPost,
		"/charge",
		strings.NewReader("alpha"),
	)
	req2.Header.Set("Idempotency-Key", "actor-key")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected actor-scoped conflict, got %d", rec2.Code)
	}
	if calls != 1 {
		t.Fatalf("expected conflicting actor request not to execute handler, got %d calls", calls)
	}
}

func TestIdempotencyRejectsReplayAcrossDifferentTenants(t *testing.T) {
	mem := newMemoryStore()
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var calls int
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		httpx.WriteJSON(w, http.StatusCreated, map[string]int{"calls": calls})
	}))

	req1 := httptest.NewRequestWithContext(
		authorization.WithScope(context.Background(), authorization.Scope{TenantID: "tenant-a"}),
		http.MethodPost,
		"/charge",
		strings.NewReader("alpha"),
	)
	req1.Header.Set("Idempotency-Key", "tenant-key")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected first request to succeed, got %d", rec1.Code)
	}

	req2 := httptest.NewRequestWithContext(
		authorization.WithScope(context.Background(), authorization.Scope{TenantID: "tenant-b"}),
		http.MethodPost,
		"/charge",
		strings.NewReader("alpha"),
	)
	req2.Header.Set("Idempotency-Key", "tenant-key")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected tenant-scoped conflict, got %d", rec2.Code)
	}
	if calls != 1 {
		t.Fatalf("expected conflicting tenant request not to execute handler, got %d calls", calls)
	}
}

func TestTenantScopedStorageKeyFuncIsolatesTenantsAndPreservesClientReplayKey(t *testing.T) {
	store := &recordingMemoryStore{memoryStore: newMemoryStore()}
	mw, err := New(Options{
		Store:          store,
		MaxBodyBytes:   1024,
		StorageKeyFunc: TenantScopedStorageKeyFunc(),
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var calls int
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		httpx.WriteJSON(w, http.StatusCreated, map[string]int{"calls": calls})
	}))

	makeRequest := func(tenantID string) *httptest.ResponseRecorder {
		ctx := authorization.WithActor(context.Background(), authorization.Actor{UserID: "user-secret-z"})
		ctx = authorization.WithScope(ctx, authorization.Scope{TenantID: tenantID, UserID: "user-secret-z"})
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/charge", strings.NewReader("alpha"))
		req.Header.Set("Idempotency-Key", "customer-visible-key-z")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	rec1 := makeRequest("tenant-secret-a-z")
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first tenant-a status = %d body=%s", rec1.Code, rec1.Body.String())
	}
	rec2 := makeRequest("tenant-secret-b-z")
	if rec2.Code != http.StatusCreated {
		t.Fatalf("tenant-b status = %d body=%s", rec2.Code, rec2.Body.String())
	}
	rec3 := makeRequest("tenant-secret-a-z")
	if rec3.Code != http.StatusCreated {
		t.Fatalf("tenant-a replay status = %d body=%s", rec3.Code, rec3.Body.String())
	}
	if calls != 2 {
		t.Fatalf("expected separate tenant records and one tenant-a replay, got %d handler calls", calls)
	}
	if got := rec3.Header().Get("Idempotency-Replayed"); got != "true" {
		t.Fatalf("replay header = %q", got)
	}
	if got := rec3.Header().Get("Idempotency-Key"); got != "customer-visible-key-z" {
		t.Fatalf("expected replay to preserve client key, got %q", got)
	}

	keys := store.UniqueKeys()
	if len(keys) != 2 {
		t.Fatalf("expected two tenant-scoped storage keys, got %#v", keys)
	}
	for _, key := range keys {
		if !strings.HasPrefix(key, "atk:v1:") {
			t.Fatalf("expected versioned storage key prefix, got %q", key)
		}
		for _, forbidden := range []string{"customer-visible-key-z", "tenant-secret", "user-secret"} {
			if strings.Contains(key, forbidden) {
				t.Fatalf("storage key %q leaked raw material %q", key, forbidden)
			}
		}
	}
}

func TestNewRequiresStore(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("expected error for missing store")
	}
}

func TestNewRejectsStoreWithoutReleaseSemantics(t *testing.T) {
	if _, err := New(Options{Store: &storeWithoutRelease{store: newMemoryStore()}}); err == nil {
		t.Fatal("expected error for store without release semantics")
	}
}

func TestNewRejectsNegativeMaxResponseBytes(t *testing.T) {
	if _, err := New(Options{
		Store:            newMemoryStore(),
		MaxResponseBytes: -1,
	}); err == nil {
		t.Fatal("expected error for negative max response bytes")
	}
}

func TestNewDefaultsClock(t *testing.T) {
	mw, err := New(Options{Store: newMemoryStore()})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	if mw.opts.Clock == nil {
		t.Fatal("expected default clock")
	}
}

func TestMiddlewareBuffersResponsesWithoutOptionalInterfaces(t *testing.T) {
	mem := newMemoryStore()
	mw, err := New(Options{
		Store:            mem,
		MaxBodyBytes:     1024,
		MaxResponseBytes: 1024,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setOptionalInterfaceHeaders(w)
		httpx.WriteJSON(w, http.StatusCreated, map[string]bool{"ok": true})
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", "key-interfaces")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	assertOptionalInterfaceHeadersFalse(t, rec.Header())
}

func TestShouldHandleOptOutPreservesOptionalResponseWriterInterfaces(t *testing.T) {
	mem := newMemoryStore()
	mw, err := New(Options{
		Store:            mem,
		MaxBodyBytes:     1024,
		MaxResponseBytes: 1024,
		ShouldHandle: func(r *http.Request) bool {
			return r.URL.Path != "/events"
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeOptionalResponseWriterEvidence(w)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/events", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", "key-streaming-opt-out")
	rec := newOptionalResponseWriter()
	handler.ServeHTTP(rec, req)

	if rec.Status() != http.StatusOK {
		t.Fatalf("expected opt-out route to pass through, got %d", rec.Status())
	}
	assertOptionalResponseWriterPreserved(t, rec)
	if rec.Body() != "stream-response" {
		t.Fatalf("expected streaming body through original writer, got %q", rec.Body())
	}
	mem.mu.Lock()
	records := len(mem.data)
	mem.mu.Unlock()
	if records != 0 {
		t.Fatalf("expected opt-out route not to create replay records, got %d", records)
	}
}

func TestIdempotencyMarksAmbiguousStateWhenResponseExceedsBufferLimit(t *testing.T) {
	mem := newMemoryStore()
	var events []OutcomeEvent
	mw, err := New(Options{
		Store:            mem,
		MaxBodyBytes:     1024,
		MaxResponseBytes: 4,
		OnOutcome: func(_ context.Context, event OutcomeEvent) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var calls int
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"body": "abcdef"})
	}))

	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req1.Header.Set("Idempotency-Key", "key-response-limit")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for oversized buffered response, got %d", rec1.Code)
	}
	if got := rec1.Body.String(); !strings.Contains(got, `"detail":"previous idempotent attempt may have completed, but its response exceeded the replay buffer limit"`) {
		t.Fatalf("expected buffer limit problem detail, got body %q", got)
	}
	if len(events) != 1 || events[0].Outcome != IdempotencyOutcomeResponseTooLarge || events[0].Status != http.StatusServiceUnavailable {
		t.Fatalf("expected response-too-large outcome after oversized response, got %#v", events)
	}
	record, found, err := mem.Get(context.Background(), "key-response-limit")
	if err != nil {
		t.Fatalf("store get after oversized response: %v", err)
	}
	if !found {
		t.Fatal("expected oversized response path to leave an ambiguous record")
	}
	if record.State != StateAmbiguous {
		t.Fatalf("expected ambiguous state, got %v", record.State)
	}

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req2.Header.Set("Idempotency-Key", "key-response-limit")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected retry to stay blocked after ambiguous outcome, got %d", rec2.Code)
	}
	if got := rec2.Body.String(); !strings.Contains(got, `"detail":"idempotency outcome is ambiguous; previous attempt may have completed"`) {
		t.Fatalf("expected ambiguous outcome detail, got body %q", got)
	}
	if len(events) != 2 || events[1].Outcome != IdempotencyOutcomeAmbiguous || events[1].Status != http.StatusServiceUnavailable {
		t.Fatalf("expected ambiguous outcome after retry, got %#v", events)
	}
	if calls != 1 {
		t.Fatalf("expected ambiguous retry not to execute handler again, got %d calls", calls)
	}
}

func TestIdempotencyAllowsRetryAfterServerError(t *testing.T) {
	mem := newMemoryStore()
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var calls int
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{
			Type:   httpx.DefaultTypeURI(httpx.TypeInternal),
			Title:  http.StatusText(http.StatusInternalServerError),
			Detail: strconv.Itoa(calls),
		})
	}))

	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req1.Header.Set("Idempotency-Key", "key-500")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec1.Code)
	}

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req2.Header.Set("Idempotency-Key", "key-500")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusInternalServerError {
		t.Fatalf("expected retry to execute handler and return 500, got %d", rec2.Code)
	}
	if got := rec2.Body.String(); !strings.Contains(got, `"detail":"2"`) {
		t.Fatalf("expected second request to execute handler again, got body %q", got)
	}
}

func TestIdempotencyAllowsRetryAfterPanic(t *testing.T) {
	mem := newMemoryStore()
	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var calls int
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			panic("boom")
		}
		httpx.WriteJSON(w, http.StatusCreated, map[string]int{"calls": calls})
	}))

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected first request to panic")
			}
		}()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
		req.Header.Set("Idempotency-Key", "key-panic")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}()

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req2.Header.Set("Idempotency-Key", "key-panic")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected retry to succeed after panic, got %d", rec2.Code)
	}
	if got := rec2.Body.String(); !strings.Contains(got, `"calls":2`) {
		t.Fatalf("expected second request to execute handler again, got body %q", got)
	}
}

func TestIdempotencyAllowsRetryAfterCanceledPanic(t *testing.T) {
	store := &contextSensitiveStore{
		memoryStore:    newMemoryStore(),
		requireCleanup: true,
	}
	mw, err := New(Options{
		Store:        store,
		MaxBodyBytes: 1024,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var calls int
	var cancel context.CancelFunc
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			cancel()
			panic("boom")
		}
		httpx.WriteJSON(w, http.StatusCreated, map[string]int{"calls": calls})
	}))

	ctx, cancelFn := context.WithCancel(context.Background())
	cancel = cancelFn
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected first request to panic")
			}
		}()
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/charge", strings.NewReader("alpha"))
		req.Header.Set("Idempotency-Key", "key-canceled-panic")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}()

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req2.Header.Set("Idempotency-Key", "key-canceled-panic")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected retry to succeed after canceled panic, got %d", rec2.Code)
	}
	if calls != 2 {
		t.Fatalf("expected second request to execute after canceled panic cleanup, got %d calls", calls)
	}
}

func TestIdempotencyReturnsServiceUnavailableWhenSaveFails(t *testing.T) {
	store := &saveFailStore{
		memoryStore:       newMemoryStore(),
		saveErr:           errors.New("save failed"),
		remainingFailures: 1,
	}

	var onError []error
	mw, err := New(Options{
		Store:        store,
		MaxBodyBytes: 1024,
		OnError: func(err error) {
			onError = append(onError, err)
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var calls int
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		httpx.WriteJSON(w, http.StatusCreated, map[string]int{"calls": calls})
	}))

	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req1.Header.Set("Idempotency-Key", "key-save-fail")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec1.Code)
	}
	if len(onError) != 1 || onError[0] == nil || onError[0].Error() != "save failed" {
		t.Fatalf("expected save failure to be reported, got %#v", onError)
	}
	if got := rec1.Body.String(); !strings.Contains(got, `"detail":"previous idempotent attempt may have completed, but its response could not be persisted"`) {
		t.Fatalf("expected persistence failure problem detail, got body %q", got)
	}

	record, found, err := store.Get(context.Background(), "key-save-fail")
	if err != nil {
		t.Fatalf("store get after save failure: %v", err)
	}
	if !found {
		t.Fatal("expected failed completion save to leave ambiguous record")
	}
	if record.State != StateAmbiguous {
		t.Fatalf("expected ambiguous state after save failure, got %v", record.State)
	}
}

func TestIdempotencyBlocksRetryAfterTransientSaveFailure(t *testing.T) {
	store := &saveFailStore{
		memoryStore:       newMemoryStore(),
		saveErr:           errors.New("save failed"),
		remainingFailures: 1,
	}

	mw, err := New(Options{
		Store:        store,
		MaxBodyBytes: 1024,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var calls int
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		httpx.WriteJSON(w, http.StatusCreated, map[string]int{"calls": calls})
	}))

	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req1.Header.Set("Idempotency-Key", "key-transient-save-fail")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected first response to fail closed with 503, got %d", rec1.Code)
	}
	record, found, err := store.Get(context.Background(), "key-transient-save-fail")
	if err != nil {
		t.Fatalf("store get after transient save failure: %v", err)
	}
	if !found {
		t.Fatal("expected transient save failure to leave ambiguous record")
	}
	if record.State != StateAmbiguous {
		t.Fatalf("expected ambiguous state after save failure, got %v", record.State)
	}

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req2.Header.Set("Idempotency-Key", "key-transient-save-fail")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected retry to stay blocked after ambiguous save failure, got %d", rec2.Code)
	}
	if got := rec2.Body.String(); !strings.Contains(got, `"detail":"idempotency outcome is ambiguous; previous attempt may have completed"`) {
		t.Fatalf("expected ambiguous outcome detail, got body %q", got)
	}
	if calls != 1 {
		t.Fatalf("expected ambiguous retry not to execute handler again, got %d calls", calls)
	}
}

func TestIdempotencyFailsClosedWhenReservationCollisionCannotBeResolved(t *testing.T) {
	store := &reservationCollisionStore{}

	mw, err := New(Options{
		Store:        store,
		MaxBodyBytes: 1024,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var calls int
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		httpx.WriteJSON(w, http.StatusCreated, map[string]int{"calls": calls})
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", "key-collision")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if got := rec.Body.String(); !strings.Contains(got, `"detail":"idempotency state is temporarily unavailable; retry with the same key later"`) {
		t.Fatalf("expected reservation collision problem detail, got body %q", got)
	}
	if calls != 0 {
		t.Fatalf("expected unresolved collision not to execute handler, got %d calls", calls)
	}
}

func TestIdempotencyReservationCollisionHonorsFailOpen(t *testing.T) {
	store := &reservationCollisionStore{}

	mw, err := New(Options{
		Store:        store,
		MaxBodyBytes: 1024,
		FailOpen:     true,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var calls int
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		httpx.WriteJSON(w, http.StatusCreated, map[string]int{"calls": calls})
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", "key-collision-open")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected fail-open request to succeed, got %d", rec.Code)
	}
	if calls != 1 {
		t.Fatalf("expected fail-open unresolved collision to execute handler once, got %d calls", calls)
	}
}

type memoryStore struct {
	mu   sync.Mutex
	data map[string]memoryEntry
	now  func() time.Time
}

type recordingMemoryStore struct {
	*memoryStore
	keyMu sync.Mutex
	keys  []string
}

type saveFailStore struct {
	*memoryStore
	mu                sync.Mutex
	saveErr           error
	remainingFailures int
}

type contextSensitiveStore struct {
	*memoryStore
	mu             sync.Mutex
	failFirstSave  bool
	saveCalls      int
	releaseCalls   int
	requireCleanup bool
}

type reservationCollisionStore struct{}

type storeWithoutRelease struct {
	store *memoryStore
}

type captureLogger struct {
	mu       sync.Mutex
	messages []string
	warnings []warnEntry
}

type legacyRecoveryErrorStore struct {
	record     Record
	releaseErr error
}

type memoryEntry struct {
	record    Record
	expiresAt time.Time
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		data: make(map[string]memoryEntry),
		now:  time.Now,
	}
}

func (m *recordingMemoryStore) recordKey(key string) {
	m.keyMu.Lock()
	defer m.keyMu.Unlock()
	m.keys = append(m.keys, key)
}

func (m *recordingMemoryStore) UniqueKeys() []string {
	m.keyMu.Lock()
	defer m.keyMu.Unlock()
	seen := make(map[string]struct{}, len(m.keys))
	out := make([]string, 0, len(m.keys))
	for _, key := range m.keys {
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func (m *recordingMemoryStore) Get(ctx context.Context, key string) (Record, bool, error) {
	m.recordKey(key)
	return m.memoryStore.Get(ctx, key)
}

func (m *recordingMemoryStore) TryBegin(ctx context.Context, key string, record Record, ttl time.Duration) (bool, error) {
	m.recordKey(key)
	return m.memoryStore.TryBegin(ctx, key, record, ttl)
}

func (m *recordingMemoryStore) Save(ctx context.Context, key string, record Record, ttl time.Duration) error {
	m.recordKey(key)
	return m.memoryStore.Save(ctx, key, record, ttl)
}

func (m *recordingMemoryStore) Release(ctx context.Context, key string) error {
	m.recordKey(key)
	return m.memoryStore.Release(ctx, key)
}

func (m *recordingMemoryStore) ReleaseReservation(ctx context.Context, key, token string) error {
	m.recordKey(key)
	return m.memoryStore.ReleaseReservation(ctx, key, token)
}

func (m *memoryStore) Get(ctx context.Context, key string) (Record, bool, error) {
	if m == nil {
		return Record{}, false, nil
	}
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.data[key]
	if !ok {
		return Record{}, false, nil
	}
	if m.isExpired(entry) {
		delete(m.data, key)
		return Record{}, false, nil
	}
	return cloneRecord(entry.record), true, nil
}

func (m *memoryStore) TryBegin(ctx context.Context, key string, record Record, ttl time.Duration) (bool, error) {
	if m == nil {
		return false, nil
	}
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.data[key]; ok && !m.isExpired(entry) {
		return false, nil
	}
	m.data[key] = memoryEntry{
		record:    cloneRecord(record),
		expiresAt: m.now().Add(ttl),
	}
	return true, nil
}

func (m *memoryStore) Save(ctx context.Context, key string, record Record, ttl time.Duration) error {
	if m == nil {
		return nil
	}
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = memoryEntry{
		record:    cloneRecord(record),
		expiresAt: m.now().Add(ttl),
	}
	return nil
}

func (m *memoryStore) Release(ctx context.Context, key string) error {
	return m.release(ctx, key, "", false)
}

func (m *memoryStore) ReleaseReservation(ctx context.Context, key, token string) error {
	return m.release(ctx, key, token, true)
}

func (m *memoryStore) release(ctx context.Context, key, token string, requireToken bool) error {
	if m == nil {
		return nil
	}
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.data[key]
	if !ok {
		return nil
	}
	if entry.record.State != StateInFlight {
		return nil
	}
	if !requireToken {
		delete(m.data, key)
		return nil
	}
	if entry.record.ReservationToken == "" {
		if token == "" {
			delete(m.data, key)
			return nil
		}
		return errors.New("idempotency reservation token mismatch")
	}
	if entry.record.ReservationToken != token {
		return errors.New("idempotency reservation token mismatch")
	}
	delete(m.data, key)
	return nil
}

func (s *saveFailStore) Save(ctx context.Context, key string, record Record, ttl time.Duration) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.remainingFailures > 0 {
		s.remainingFailures--
		return s.saveErr
	}
	return s.memoryStore.Save(ctx, key, record, ttl)
}

func (s *contextSensitiveStore) Save(ctx context.Context, key string, record Record, ttl time.Duration) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	s.saveCalls++
	failFirst := s.failFirstSave && s.saveCalls == 1
	s.mu.Unlock()
	if failFirst {
		return errors.New("save failed")
	}
	if s.requireCleanup && ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return s.memoryStore.Save(ctx, key, record, ttl)
}

func (s *contextSensitiveStore) Release(ctx context.Context, key string) error {
	return s.release(ctx, key, "", false)
}

func (s *contextSensitiveStore) ReleaseReservation(ctx context.Context, key, token string) error {
	return s.release(ctx, key, token, true)
}

func (s *contextSensitiveStore) release(ctx context.Context, key, token string, requireToken bool) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	s.releaseCalls++
	s.mu.Unlock()
	if s.requireCleanup && ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if requireToken {
		return s.memoryStore.ReleaseReservation(ctx, key, token)
	}
	return s.memoryStore.Release(ctx, key)
}

func (s *reservationCollisionStore) Get(ctx context.Context, key string) (Record, bool, error) {
	_ = ctx
	_ = key
	return Record{}, false, nil
}

func (s *reservationCollisionStore) TryBegin(ctx context.Context, key string, record Record, ttl time.Duration) (bool, error) {
	_ = ctx
	_ = key
	_ = record
	_ = ttl
	return false, nil
}

func (s *reservationCollisionStore) Save(ctx context.Context, key string, record Record, ttl time.Duration) error {
	_ = ctx
	_ = key
	_ = record
	_ = ttl
	return nil
}

func (s *reservationCollisionStore) Release(ctx context.Context, key string) error {
	_ = ctx
	_ = key
	return nil
}

func (s *reservationCollisionStore) ReleaseReservation(ctx context.Context, key, token string) error {
	_ = ctx
	_ = key
	_ = token
	return nil
}

func (s *storeWithoutRelease) Get(ctx context.Context, key string) (Record, bool, error) {
	if s == nil || s.store == nil {
		return Record{}, false, nil
	}
	return s.store.Get(ctx, key)
}

func (s *storeWithoutRelease) TryBegin(ctx context.Context, key string, record Record, ttl time.Duration) (bool, error) {
	if s == nil || s.store == nil {
		return false, nil
	}
	return s.store.TryBegin(ctx, key, record, ttl)
}

func (s *storeWithoutRelease) Save(ctx context.Context, key string, record Record, ttl time.Duration) error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.Save(ctx, key, record, ttl)
}

func (s *captureLogger) Debug(msg string, _ ...any) {}

func (s *captureLogger) Info(msg string, _ ...any) {}

func (s *captureLogger) Warn(msg string, values ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
	s.warnings = append(s.warnings, warnEntry{
		msg:    msg,
		values: append([]any(nil), values...),
	})
}

func (s *captureLogger) Error(msg string, _ ...any) {}

func (s *captureLogger) WarnCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.warnings)
}

func (s *captureLogger) LastWarnMessage() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.warnings) == 0 {
		return ""
	}
	return s.warnings[len(s.warnings)-1].msg
}

func (s *captureLogger) WarnValues() [][]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	values := make([][]any, 0, len(s.warnings))
	for _, warning := range s.warnings {
		values = append(values, append([]any(nil), warning.values...))
	}
	return values
}

func (s *legacyRecoveryErrorStore) Get(_ context.Context, _ string) (Record, bool, error) {
	if s == nil {
		return Record{}, false, nil
	}
	return s.record, true, nil
}

func (s *legacyRecoveryErrorStore) TryBegin(_ context.Context, _ string, _ Record, _ time.Duration) (bool, error) {
	if s == nil {
		return false, nil
	}
	return false, nil
}

func (s *legacyRecoveryErrorStore) Save(_ context.Context, _ string, _ Record, _ time.Duration) error {
	return nil
}

func (s *legacyRecoveryErrorStore) Release(_ context.Context, key string) error {
	_ = key
	if s == nil {
		return nil
	}
	return s.releaseErr
}

func (s *legacyRecoveryErrorStore) ReleaseReservation(_ context.Context, key, token string) error {
	_ = key
	_ = token
	if s == nil {
		return nil
	}
	return s.releaseErr
}

func (m *memoryStore) isExpired(entry memoryEntry) bool {
	if entry.expiresAt.IsZero() {
		return false
	}
	return m.now().After(entry.expiresAt)
}

func cloneRecord(record Record) Record {
	out := record
	if record.Header != nil {
		out.Header = record.Header.Clone()
	}
	if record.Body != nil {
		out.Body = append([]byte(nil), record.Body...)
	}
	return out
}

func setOptionalInterfaceHeaders(w http.ResponseWriter) {
	_, flusher := w.(http.Flusher)
	_, hijacker := w.(http.Hijacker)
	_, pusher := w.(http.Pusher)
	_, readerFrom := w.(io.ReaderFrom)
	w.Header().Set("X-Has-Flusher", strconv.FormatBool(flusher))
	w.Header().Set("X-Has-Hijacker", strconv.FormatBool(hijacker))
	w.Header().Set("X-Has-Pusher", strconv.FormatBool(pusher))
	w.Header().Set("X-Has-ReaderFrom", strconv.FormatBool(readerFrom))
}

func assertOptionalInterfaceHeadersFalse(t *testing.T, header http.Header) {
	t.Helper()
	if got := header.Get("X-Has-Flusher"); got != "false" {
		t.Fatalf("expected buffered writer without flusher, got %q", got)
	}
	if got := header.Get("X-Has-Hijacker"); got != "false" {
		t.Fatalf("expected buffered writer without hijacker, got %q", got)
	}
	if got := header.Get("X-Has-Pusher"); got != "false" {
		t.Fatalf("expected buffered writer without pusher, got %q", got)
	}
	if got := header.Get("X-Has-ReaderFrom"); got != "false" {
		t.Fatalf("expected buffered writer without readerFrom, got %q", got)
	}
}

type optionalResponseWriter struct {
	header        http.Header
	status        int
	body          strings.Builder
	flushed       bool
	pushedTargets []string
	readFromCalls int
}

func newOptionalResponseWriter() *optionalResponseWriter {
	return &optionalResponseWriter{header: make(http.Header)}
}

func (w *optionalResponseWriter) Header() http.Header {
	return w.header
}

func (w *optionalResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(p)
}

func (w *optionalResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *optionalResponseWriter) Flush() {
	w.flushed = true
}

func (w *optionalResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, net.ErrClosed
}

func (w *optionalResponseWriter) Push(target string, _ *http.PushOptions) error {
	w.pushedTargets = append(w.pushedTargets, target)
	return nil
}

func (w *optionalResponseWriter) ReadFrom(r io.Reader) (int64, error) {
	w.readFromCalls++
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return io.Copy(&w.body, r)
}

func (w *optionalResponseWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *optionalResponseWriter) Body() string {
	return w.body.String()
}

func writeOptionalResponseWriterEvidence(w http.ResponseWriter) {
	flusher, hasFlusher := w.(http.Flusher)
	_, hasHijacker := w.(http.Hijacker)
	pusher, hasPusher := w.(http.Pusher)
	readerFrom, hasReaderFrom := w.(io.ReaderFrom)
	w.Header().Set("X-Has-Flusher", strconv.FormatBool(hasFlusher))
	w.Header().Set("X-Has-Hijacker", strconv.FormatBool(hasHijacker))
	w.Header().Set("X-Has-Pusher", strconv.FormatBool(hasPusher))
	w.Header().Set("X-Has-ReaderFrom", strconv.FormatBool(hasReaderFrom))
	w.WriteHeader(http.StatusOK)
	if hasFlusher {
		flusher.Flush()
	}
	if hasPusher {
		_ = pusher.Push("/events/next", nil)
	}
	if hasReaderFrom {
		_, _ = readerFrom.ReadFrom(strings.NewReader("stream-response"))
		return
	}
	_, _ = w.Write([]byte("stream-response"))
}

func assertOptionalResponseWriterPreserved(t *testing.T, w *optionalResponseWriter) {
	t.Helper()
	for name, value := range map[string]string{
		"X-Has-Flusher":    "true",
		"X-Has-Hijacker":   "true",
		"X-Has-Pusher":     "true",
		"X-Has-ReaderFrom": "true",
	} {
		if got := w.Header().Get(name); got != value {
			t.Fatalf("%s = %q, want %q", name, got, value)
		}
	}
	if !w.flushed {
		t.Fatal("expected opt-out route to call Flush on original writer")
	}
	if len(w.pushedTargets) != 1 || w.pushedTargets[0] != "/events/next" {
		t.Fatalf("pushed targets = %v, want [/events/next]", w.pushedTargets)
	}
	if w.readFromCalls != 1 {
		t.Fatalf("ReadFrom calls = %d, want 1", w.readFromCalls)
	}
}

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

type sequenceClock struct {
	timestamps []time.Time
	pos        int
}

func (c *sequenceClock) Now() time.Time {
	if c == nil || len(c.timestamps) == 0 {
		return time.Time{}
	}
	if c.pos >= len(c.timestamps) {
		return c.timestamps[len(c.timestamps)-1]
	}
	next := c.timestamps[c.pos]
	c.pos++
	return next
}

type warnEntry struct {
	msg    string
	values []any
}

func warnKeyValue(values []any) map[string]any {
	out := make(map[string]any, len(values)/2)
	for i := 0; i+1 < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok || key == "" {
			continue
		}
		out[key] = values[i+1]
	}
	return out
}

func warnValueAsString(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	raw, ok := values[key]
	if !ok {
		return ""
	}
	str, ok := raw.(string)
	if !ok {
		return ""
	}
	return str
}

func hashValue(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:])
}

func TestIdempotencyStoresCompletedResponseAfterRequestCancellation(t *testing.T) {
	store := &contextSensitiveStore{
		memoryStore:    newMemoryStore(),
		requireCleanup: true,
	}
	mw, err := New(Options{
		Store:        store,
		MaxBodyBytes: 1024,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var cancel context.CancelFunc
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancel()
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
	}))

	ctx, cancelFn := context.WithCancel(context.Background())
	cancel = cancelFn
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", "key-canceled-success")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected completed response to be persisted after cancellation, got %d", rec.Code)
	}
	record, found, err := store.Get(context.Background(), "key-canceled-success")
	if err != nil {
		t.Fatalf("store get: %v", err)
	}
	if !found {
		t.Fatal("expected completed idempotency record")
	}
	if record.State != StateCompleted {
		t.Fatalf("expected completed record, got %v", record.State)
	}
}

func TestIdempotencyReleasesReservationAfterServerErrorWhenRequestCanceled(t *testing.T) {
	store := &contextSensitiveStore{
		memoryStore:    newMemoryStore(),
		requireCleanup: true,
	}
	mw, err := New(Options{
		Store:        store,
		MaxBodyBytes: 1024,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var calls int
	var cancel context.CancelFunc
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		cancel()
		httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{
			Type:   httpx.DefaultTypeURI(httpx.TypeInternal),
			Title:  http.StatusText(http.StatusInternalServerError),
			Detail: "boom",
		})
	}))

	ctx, cancelFn := context.WithCancel(context.Background())
	cancel = cancelFn
	req1 := httptest.NewRequestWithContext(ctx, http.MethodPost, "/charge", strings.NewReader("alpha"))
	req1.Header.Set("Idempotency-Key", "key-canceled-500")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec1.Code)
	}

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req2.Header.Set("Idempotency-Key", "key-canceled-500")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusInternalServerError {
		t.Fatalf("expected retry to execute handler again, got %d", rec2.Code)
	}
	if calls != 2 {
		t.Fatalf("expected retry after cleanup release, got %d calls", calls)
	}
}

func TestIdempotencyMarksAmbiguousAfterOversizedResponseWhenRequestCanceled(t *testing.T) {
	store := &contextSensitiveStore{
		memoryStore:    newMemoryStore(),
		requireCleanup: true,
	}
	mw, err := New(Options{
		Store:            store,
		MaxBodyBytes:     1024,
		MaxResponseBytes: 4,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var cancel context.CancelFunc
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancel()
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"body": "abcdef"})
	}))

	ctx, cancelFn := context.WithCancel(context.Background())
	cancel = cancelFn
	req1 := httptest.NewRequestWithContext(ctx, http.MethodPost, "/charge", strings.NewReader("alpha"))
	req1.Header.Set("Idempotency-Key", "key-canceled-oversized")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for oversized response, got %d", rec1.Code)
	}

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req2.Header.Set("Idempotency-Key", "key-canceled-oversized")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected ambiguous retry to stay blocked, got %d", rec2.Code)
	}
	if got := rec2.Body.String(); !strings.Contains(got, `"detail":"idempotency outcome is ambiguous; previous attempt may have completed"`) {
		t.Fatalf("expected ambiguous detail, got %q", got)
	}
}

func TestIdempotencyPersistsAmbiguousStateAfterSaveFailureWhenRequestCanceled(t *testing.T) {
	store := &contextSensitiveStore{
		memoryStore:    newMemoryStore(),
		failFirstSave:  true,
		requireCleanup: true,
	}
	mw, err := New(Options{
		Store:        store,
		MaxBodyBytes: 1024,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var cancel context.CancelFunc
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancel()
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
	}))

	ctx, cancelFn := context.WithCancel(context.Background())
	cancel = cancelFn
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", "key-canceled-save-fail")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 on initial save failure, got %d", rec.Code)
	}

	record, found, err := store.Get(context.Background(), "key-canceled-save-fail")
	if err != nil {
		t.Fatalf("store get: %v", err)
	}
	if !found {
		t.Fatal("expected ambiguous record after failed save")
	}
	if record.State != StateAmbiguous {
		t.Fatalf("expected ambiguous state, got %v", record.State)
	}
}

func TestIdempotencyInFlightResponseIncludesConfiguredRetryAfter(t *testing.T) {
	mem := newMemoryStore()
	record := Record{
		State:       StateInFlight,
		RequestHash: "hash",
		CreatedAt:   time.Now(),
	}
	ok, err := mem.TryBegin(context.Background(), "key-inflight-retry-after", record, time.Minute)
	if err != nil {
		t.Fatalf("seed store: %v", err)
	}
	if !ok {
		t.Fatal("expected to seed in-flight record")
	}

	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
		InFlightTTL:  90 * time.Second,
		HashFunc: func(*http.Request, []byte) (string, error) {
			return "hash", nil
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expected in-flight request not to execute handler")
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", "key-inflight-retry-after")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "90" {
		t.Fatalf("expected Retry-After 90, got %q", got)
	}
}

func TestIdempotencyAmbiguousResponseUsesRemainingRetryWindow(t *testing.T) {
	now := time.Date(2026, time.April, 23, 12, 0, 0, 0, time.UTC)
	mem := newMemoryStore()
	err := mem.Save(context.Background(), "key-ambiguous-retry-after", Record{
		State:       StateAmbiguous,
		RequestHash: "hash",
		CreatedAt:   now.Add(-30 * time.Second),
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("seed store: %v", err)
	}

	mw, err := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
		TTL:          2 * time.Minute,
		Clock:        fixedClock{now: now},
		HashFunc: func(*http.Request, []byte) (string, error) {
			return "hash", nil
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expected ambiguous request not to execute handler")
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
	req.Header.Set("Idempotency-Key", "key-ambiguous-retry-after")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "90" {
		t.Fatalf("expected Retry-After 90, got %q", got)
	}
}
