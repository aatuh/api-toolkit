package httpx

import (
	"errors"
	"net/http"
)

// HeaderLimits defines request header size/count limits.
type HeaderLimits struct {
	// MaxBytes caps the approximate header bytes (name + value + separators).
	MaxBytes int
	// MaxCount caps the total number of header values.
	MaxCount int
}

var (
	ErrHeaderBytesExceeded = errors.New("request headers exceed max bytes")
	ErrHeaderCountExceeded = errors.New("request headers exceed max count")
)

var (
	HeaderLimitsStrict   = HeaderLimits{MaxBytes: 32 << 10, MaxCount: 40}
	HeaderLimitsBalanced = HeaderLimits{MaxBytes: 64 << 10, MaxCount: 100}
	HeaderLimitsRelaxed  = HeaderLimits{MaxBytes: 1 << 20, MaxCount: 200}
)

// ApplyServer sets MaxHeaderBytes using the configured limit.
func (l HeaderLimits) ApplyServer(s *http.Server) {
	if s == nil {
		return
	}
	if l.MaxBytes > 0 {
		s.MaxHeaderBytes = l.MaxBytes
	}
}

// Check validates the request headers against configured limits.
func (l HeaderLimits) Check(r *http.Request) error {
	if r == nil {
		return nil
	}
	if l.MaxCount > 0 && HeaderCount(r.Header) > l.MaxCount {
		return ErrHeaderCountExceeded
	}
	if l.MaxBytes > 0 && HeaderBytes(r.Header) > l.MaxBytes {
		return ErrHeaderBytesExceeded
	}
	return nil
}

// HeaderCount returns the total number of header values.
func HeaderCount(h http.Header) int {
	if len(h) == 0 {
		return 0
	}
	count := 0
	for _, values := range h {
		count += len(values)
	}
	return count
}

// HeaderBytes returns an approximate size for header names and values.
func HeaderBytes(h http.Header) int {
	if len(h) == 0 {
		return 0
	}
	total := 0
	for name, values := range h {
		for _, value := range values {
			total += len(name) + len(value) + 2
		}
	}
	return total
}
