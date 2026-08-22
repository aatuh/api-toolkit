package timeout

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHardTimeoutWrapRouteRejectsUnsupportedCapabilities(t *testing.T) {
	middleware, err := NewHard(Options{Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewHard() error = %v", err)
	}

	cases := []struct {
		name         string
		capabilities RouteCapabilities
	}{
		{name: "streaming", capabilities: RouteCapabilities{Streaming: true}},
		{name: "server-sent events", capabilities: RouteCapabilities{ServerSentEvents: true}},
		{name: "websocket upgrade", capabilities: RouteCapabilities{WebSocketUpgrade: true}},
		{name: "large download", capabilities: RouteCapabilities{LargeDownload: true}},
		{name: "flusher", capabilities: RouteCapabilities{Flusher: true}},
		{name: "hijacker", capabilities: RouteCapabilities{Hijacker: true}},
		{name: "pusher", capabilities: RouteCapabilities{Pusher: true}},
		{name: "reader from", capabilities: RouteCapabilities{ReaderFrom: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := middleware.WrapRoute(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), tc.capabilities)
			if err == nil {
				t.Fatalf("WrapRoute(%#v) error = nil, want incompatible capability error", tc.capabilities)
			}
		})
	}
}

func TestHardTimeoutWrapRouteAllowsFiniteResponse(t *testing.T) {
	middleware, err := NewHard(Options{Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewHard() error = %v", err)
	}
	handler, err := middleware.WrapRoute(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}), RouteCapabilities{})
	if err != nil {
		t.Fatalf("WrapRoute() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/widgets", nil))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
}

func TestCooperativeTimeoutPreservesStreamingFlusherHijackerPusherAndReaderFrom(t *testing.T) {
	middleware, err := NewPropagator(Options{Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewPropagator() error = %v", err)
	}
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			t.Error("response writer lost http.Flusher")
		}
		if _, ok := w.(http.Hijacker); !ok {
			t.Error("response writer lost http.Hijacker")
		}
		if _, ok := w.(http.Pusher); !ok {
			t.Error("response writer lost http.Pusher")
		}
		if _, ok := w.(io.ReaderFrom); !ok {
			t.Error("response writer lost io.ReaderFrom")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	writer := &capabilityResponseWriter{}
	handler.ServeHTTP(writer, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/events", nil))
	if writer.status != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", writer.status, http.StatusNoContent)
	}
}

func TestHardTimeoutReportsHandlerThatContinuesAfterTimeout(t *testing.T) {
	events := make(chan HardTimeoutEvent, 1)
	release := make(chan struct{})
	finished := make(chan struct{})
	middleware, err := NewHard(Options{
		Timeout: 25 * time.Millisecond,
		EventHooks: &HardTimeoutEventHooks{
			OnHandlerContinuesAfterTimeout: func(event HardTimeoutEvent) {
				events <- event
			},
		},
	})
	if err != nil {
		t.Fatalf("NewHard() error = %v", err)
	}
	handler := middleware.Handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		defer close(finished)
		<-r.Context().Done()
		<-release
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/slow", nil))
	if recorder.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusGatewayTimeout)
	}
	select {
	case event := <-events:
		if event.Outcome != HardTimeoutOutcomeTimeout || event.Method != http.MethodGet || !event.TimedOut {
			t.Fatalf("event = %#v, want bounded timeout event", event)
		}
	case <-time.After(time.Second):
		t.Fatal("expected handler-continues-after-timeout event")
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("handler did not finish after release")
	}
}

type capabilityResponseWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *capabilityResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *capabilityResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *capabilityResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(body)
}

func (*capabilityResponseWriter) Flush() {}

func (*capabilityResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, errors.New("hijack is not available in this test writer")
}

func (*capabilityResponseWriter) Push(string, *http.PushOptions) error { return nil }

func (w *capabilityResponseWriter) ReadFrom(reader io.Reader) (int64, error) {
	return w.body.ReadFrom(reader)
}
