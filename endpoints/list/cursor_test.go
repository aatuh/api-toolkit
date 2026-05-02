package list

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v2/fielderrors"
)

func TestHMACCursorCodecRoundTripAndTamperDetection(t *testing.T) {
	codec := NewHMACCursorCodec([]byte("secret"))
	cursor, err := codec.Encode(map[string]string{"after_id": "item_123"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	values, err := codec.Decode(cursor)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if values["after_id"] != "item_123" {
		t.Fatalf("values = %#v", values)
	}
	if _, err := codec.Decode(cursor + "x"); err == nil {
		t.Fatal("expected tampered cursor to fail")
	}
}

func TestHMACCursorCodecRejectsExpiredCursor(t *testing.T) {
	codec := NewHMACCursorCodec([]byte("secret"))
	cursor, err := codec.Encode(map[string]string{"after_id": "item_123"}, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if _, err := codec.Decode(cursor); err == nil {
		t.Fatal("expected expired cursor error")
	}
}

func TestParseCursorQueryChecked(t *testing.T) {
	codec := NewHMACCursorCodec([]byte("secret"))
	cursor, err := codec.Encode(map[string]string{"after_id": "item_123"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/items?limit=5&cursor="+cursor, nil)
	query, err := ParseCursorQueryChecked(req, CursorQueryConfig{DefaultLimit: 10, MaxLimit: 20, Codec: codec})
	if err != nil {
		t.Fatalf("ParseCursorQueryChecked() error = %v", err)
	}
	if query.Limit != 5 || query.Values["after_id"] != "item_123" {
		t.Fatalf("query = %#v", query)
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/items?limit=nope&cursor=bad", nil)
	_, err = ParseCursorQueryChecked(req, CursorQueryConfig{DefaultLimit: 10, MaxLimit: 20, Codec: codec})
	if !hasListFieldError(err, "limit") || !hasListFieldError(err, "cursor") {
		t.Fatalf("expected limit and cursor errors, got %v", err)
	}
}

func TestNewCursorResponse(t *testing.T) {
	resp := NewCursorResponse([]string{"a", "b"}, "next", CursorQuery{Limit: 2, Cursor: "current"})
	if len(resp.Data) != 2 || resp.Meta.Count != 2 || resp.Meta.NextCursor != "next" || resp.Meta.Cursor != "current" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestHMACCursorCodecOutputIsDeterministic(t *testing.T) {
	codec := NewHMACCursorCodec([]byte("secret"))
	expires := time.Unix(123, 0)
	first, err := codec.Encode(map[string]string{"b": "2", "a": "1"}, expires)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	second, err := codec.Encode(map[string]string{"a": "1", "b": "2"}, expires)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if first != second {
		t.Fatalf("cursor output differs\nfirst=%s\nsecond=%s", first, second)
	}
	if strings.Contains(first, "+") || strings.Contains(first, "/") || strings.Contains(first, "=") {
		t.Fatalf("cursor is not raw URL-safe base64: %s", first)
	}
}

func hasListFieldError(err error, field string) bool {
	provider, ok := err.(fielderrors.Provider)
	if !ok {
		return false
	}
	for _, entry := range provider.FieldErrors() {
		if entry.Field == field {
			return true
		}
	}
	return false
}
