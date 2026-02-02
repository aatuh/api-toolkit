package openapi

import "errors"

const defaultResponseMaxBodyBytes int64 = 1 << 20

var errResponseTooLarge = errors.New("response body exceeds validation limit")

type responseValidation struct {
	maxBodyBytes int64
	errorHandler ErrorHandler
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
	return &responseValidation{
		maxBodyBytes: maxBytes,
		errorHandler: handler,
	}
}
