package async

import (
	"context"
	"errors"
	"testing"
)

func TestHandlerMuxRoutesBySafeJobKind(t *testing.T) {
	mux, err := NewHandlerMux()
	if err != nil {
		t.Fatalf("NewHandlerMux() error = %v", err)
	}
	var handled Job
	if err := mux.Register("widgets.import", HandlerFunc(func(_ context.Context, job Job) error {
		handled = job
		return nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err = mux.Handle(context.Background(), Job{ID: "job_1", Kind: " widgets.import ", TenantID: "org_1"})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if handled.ID != "job_1" || handled.Kind != " widgets.import " {
		t.Fatalf("handled job = %#v", handled)
	}
}

func TestHandlerMuxRejectsUnknownJobKind(t *testing.T) {
	mux, err := NewHandlerMux(HandlerRoute{Kind: "widgets.import", Handler: HandlerFunc(func(context.Context, Job) error { return nil })})
	if err != nil {
		t.Fatalf("NewHandlerMux() error = %v", err)
	}

	err = mux.Handle(context.Background(), Job{ID: "job_1", Kind: "webhook.delivery"})
	if !errors.Is(err, ErrHandlerNotFound) {
		t.Fatalf("Handle() error = %v, want %v", err, ErrHandlerNotFound)
	}
}

func TestHandlerMuxRejectsInvalidRegistration(t *testing.T) {
	mux, err := NewHandlerMux()
	if err != nil {
		t.Fatalf("NewHandlerMux() error = %v", err)
	}
	if err := mux.Register("", HandlerFunc(func(context.Context, Job) error { return nil })); !errors.Is(err, ErrInvalidJobKind) {
		t.Fatalf("empty kind Register() error = %v, want %v", err, ErrInvalidJobKind)
	}
	if err := mux.Register("widgets.import", nil); !errors.Is(err, ErrHandlerNotFound) {
		t.Fatalf("nil handler Register() error = %v, want %v", err, ErrHandlerNotFound)
	}
}

func TestHandlerMuxRoutesInitialHandlers(t *testing.T) {
	calls := 0
	mux, err := NewHandlerMux(
		HandlerRoute{Kind: "widgets.import", Handler: HandlerFunc(func(context.Context, Job) error { return nil })},
		HandlerRoute{Kind: "webhook.delivery", Handler: HandlerFunc(func(context.Context, Job) error {
			calls++
			return nil
		})},
	)
	if err != nil {
		t.Fatalf("NewHandlerMux() error = %v", err)
	}

	if err := mux.Handle(context.Background(), Job{ID: "job_1", Kind: "webhook.delivery"}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("webhook handler calls = %d, want 1", calls)
	}
}

func TestHandlerMuxRejectsInvalidInitialHandlers(t *testing.T) {
	_, err := NewHandlerMux(HandlerRoute{Kind: "", Handler: HandlerFunc(func(context.Context, Job) error { return nil })})
	if !errors.Is(err, ErrInvalidJobKind) {
		t.Fatalf("empty initial route NewHandlerMux() error = %v, want %v", err, ErrInvalidJobKind)
	}
	_, err = NewHandlerMux(HandlerRoute{Kind: "widgets.import", Handler: nil})
	if !errors.Is(err, ErrHandlerNotFound) {
		t.Fatalf("nil initial handler NewHandlerMux() error = %v, want %v", err, ErrHandlerNotFound)
	}
}
