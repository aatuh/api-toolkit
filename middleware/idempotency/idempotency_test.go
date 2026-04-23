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

func (s *contextSensitiveStore) Save(ctx context.Context, key string, record ports.IdempotencyRecord, ttl time.Duration) error {
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
	if s == nil {
		return nil
	}
	s.mu.Lock()
	s.releaseCalls++
	s.mu.Unlock()
	if s.requireCleanup && ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return s.memoryStore.Release(ctx, key)
}

func (s *reservationCollisionStore) Get(ctx context.Context, key string) (ports.IdempotencyRecord, bool, error) {
	_ = ctx
	_ = key
	return ports.IdempotencyRecord{}, false, nil
}

func (s *reservationCollisionStore) TryBegin(ctx context.Context, key string, record ports.IdempotencyRecord, ttl time.Duration) (bool, error) {
	_ = ctx
	_ = key
	_ = record
	_ = ttl
	return false, nil
}

func (s *reservationCollisionStore) Save(ctx context.Context, key string, record ports.IdempotencyRecord, ttl time.Duration) error {
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

func (s *storeWithoutRelease) Get(ctx context.Context, key string) (ports.IdempotencyRecord, bool, error) {
	if s == nil || s.store == nil {
		return ports.IdempotencyRecord{}, false, nil
	}
	return s.store.Get(ctx, key)
}

func (s *storeWithoutRelease) TryBegin(ctx context.Context, key string, record ports.IdempotencyRecord, ttl time.Duration) (bool, error) {
	if s == nil || s.store == nil {
		return false, nil
	}
	return s.store.TryBegin(ctx, key, record, ttl)
}

func (s *storeWithoutRelease) Save(ctx context.Context, key string, record ports.IdempotencyRecord, ttl time.Duration) error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.Save(ctx, key, record, ttl)
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
	if record.State != ports.IdempotencyStateCompleted {
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
	if record.State != ports.IdempotencyStateAmbiguous {
		t.Fatalf("expected ambiguous state, got %v", record.State)
	}
}
