package openapi

import (
	"errors"
	"net/http"
)

const defaultResponseMaxBodyBytes int64 = 1 << 20

var errResponseTooLarge = errors.New("response body exceeds validation limit")

type responseValidation struct {
	maxBodyBytes   int64
	shouldValidate func(*http.Request) bool
	errorHandler   ErrorHandler
}

func newResponseValidation(opts ResponseValidationOptions, fallback ErrorHandler) *responseValidation {
	if !opts.Enabled {
		return nil
	}
	maxBytes := opts.MaxBodyBytes
	if maxBytes == 0 {
		maxBytes = defaultResponseMaxBodyBytes
	}
	if maxBytes < 0 {
		maxBytes = 0
	}
	handler := opts.ErrorHandler
	if handler == nil {
		handler = fallback
	}
	shouldValidate := opts.ShouldValidate
	if shouldValidate == nil {
		shouldValidate = func(*http.Request) bool { return true }
	}
	return &responseValidation{
		maxBodyBytes:   maxBytes,
		shouldValidate: shouldValidate,
		errorHandler:   handler,
	}
}
