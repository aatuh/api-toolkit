package idempotency

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v2/authorization"
	"github.com/aatuh/api-toolkit/v2/httpx"
	"github.com/aatuh/api-toolkit/v2/ports"
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

func TestNewRequiresStore(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("expected error for missing store")
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

func TestIdempotencyMarksAmbiguousStateWhenResponseExceedsBufferLimit(t *testing.T) {
	mem := newMemoryStore()
	mw, err := New(Options{
		Store:            mem,
		MaxBodyBytes:     1024,
		MaxResponseBytes: 4,
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
	record, found, err := mem.Get(context.Background(), "key-response-limit")
	if err != nil {
		t.Fatalf("store get after oversized response: %v", err)
	}
	if !found {
		t.Fatal("expected oversized response path to leave an ambiguous record")
	}
	if record.State != ports.IdempotencyStateAmbiguous {
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
	if record.State != ports.IdempotencyStateAmbiguous {
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
	if record.State != ports.IdempotencyStateAmbiguous {
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

type memoryStore struct {
	mu   sync.Mutex
	data map[string]memoryEntry
	now  func() time.Time
}

type saveFailStore struct {
	*memoryStore
	mu                sync.Mutex
	saveErr           error
	remainingFailures int
}

type memoryEntry struct {
	record    ports.IdempotencyRecord
	expiresAt time.Time
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		data: make(map[string]memoryEntry),
		now:  time.Now,
	}
}

func (m *memoryStore) Get(ctx context.Context, key string) (ports.IdempotencyRecord, bool, error) {
	if m == nil {
		return ports.IdempotencyRecord{}, false, nil
	}
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.data[key]
	if !ok {
		return ports.IdempotencyRecord{}, false, nil
	}
	if m.isExpired(entry) {
		delete(m.data, key)
		return ports.IdempotencyRecord{}, false, nil
	}
	return cloneRecord(entry.record), true, nil
}

func (m *memoryStore) TryBegin(ctx context.Context, key string, record ports.IdempotencyRecord, ttl time.Duration) (bool, error) {
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

func (m *memoryStore) Save(ctx context.Context, key string, record ports.IdempotencyRecord, ttl time.Duration) error {
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
	if m == nil {
		return nil
	}
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (s *saveFailStore) Save(ctx context.Context, key string, record ports.IdempotencyRecord, ttl time.Duration) error {
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

func (m *memoryStore) isExpired(entry memoryEntry) bool {
	if entry.expiresAt.IsZero() {
		return false
	}
	return m.now().After(entry.expiresAt)
}

func cloneRecord(record ports.IdempotencyRecord) ports.IdempotencyRecord {
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
