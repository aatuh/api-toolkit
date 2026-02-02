package ports

import "net/http"

// HTTPClient describes an outbound HTTP client.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}
