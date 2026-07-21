package binding

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type fuzzBindingJSON struct {
	Name     string `json:"name" required:"true"`
	Quantity int    `json:"quantity" required:"true"`
	Active   bool   `json:"active"`
}

type fuzzBindingQuery struct {
	Limit  int      `query:"limit" required:"true"`
	Tags   []string `query:"tag"`
	Active bool     `query:"active"`
}

func FuzzDecodeJSONAndQuery(f *testing.F) {
	for _, seed := range []struct {
		body  string
		query string
	}{
		{`{"name":"starter","quantity":2,"active":true}`, "limit=10&tag=a&tag=b&active=true"},
		{`{"name":"","quantity":0}`, "limit=nope&active=maybe"},
		{`{"name":"","quantity":0}`, "limit=0&tag=&tag=x"},
		{`{"name":null,"quantity":null}`, "limit=&tag="},
		{`{"name":"first","name":"second","quantity":0}`, "limit=0&limit=1"},
		{`{"name":"starter"} trailing`, "limit=0"},
		{`["not-object"]`, "tag=&tag=x"},
	} {
		f.Add(seed.body, seed.query)
	}
	f.Fuzz(func(t *testing.T, body, rawQuery string) {
		body = limitFuzzString(body, 4096)
		rawQuery = limitFuzzString(rawQuery, 4096)

		req := &http.Request{Body: http.NoBody}
		req.Body = nopCloser{strings.NewReader(body)}
		decoded, err := DecodeJSON[fuzzBindingJSON](req, JSONConfig{
			MaxBytes:           4096,
			AllowUnknownFields: true,
		})
		if err == nil {
			if decoded.Name == "" {
				t.Fatalf("DecodeJSON accepted empty required name for %q", body)
			}
			if decoded.Quantity == 0 {
				t.Fatalf("DecodeJSON accepted zero required quantity for %q", body)
			}
		}

		presenceReq := &http.Request{Body: nopCloser{strings.NewReader(body)}}
		_, _ = DecodeJSON[fuzzBindingJSON](presenceReq, JSONConfig{
			MaxBytes:           4096,
			AllowUnknownFields: true,
			RequiredMode:       RequiredModePresent,
		})

		queryReq := &http.Request{URL: &url.URL{Path: "/", RawQuery: rawQuery}}
		query, err := DecodeQuery[fuzzBindingQuery](queryReq, QueryConfig{})
		if err == nil {
			for _, tag := range query.Tags {
				if strings.TrimSpace(tag) == "" {
					t.Fatalf("DecodeQuery returned an empty tag from %q: %#v", rawQuery, query.Tags)
				}
			}
		}
		_, _ = DecodeQuery[fuzzBindingQuery](queryReq, QueryConfig{RequiredMode: RequiredModePresent})
	})
}

type nopCloser struct {
	*strings.Reader
}

func (n nopCloser) Close() error { return nil }

func limitFuzzString(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
