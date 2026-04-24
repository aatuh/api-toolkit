package telemetry

import (
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const defaultHTTPClientTimeout = 10 * time.Second

// WrapHTTPTransport returns a RoundTripper that propagates trace context.
func WrapHTTPTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return otelhttp.NewTransport(base)
}

// WrapHTTPClient returns a shallow copy of the client using an OTel transport.
func WrapHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPClientTimeout}
	}
	next := *client
	next.Transport = WrapHTTPTransport(client.Transport)
	return &next
}
