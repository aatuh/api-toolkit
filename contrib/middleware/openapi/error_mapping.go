package openapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"

	"github.com/aatuh/api-toolkit/v4/fielderrors"
	"github.com/aatuh/api-toolkit/v4/httpx"
)

func statusFromOpenAPIError(err error) int {
	if err == nil {
		return http.StatusBadRequest
	}
	var respErr *openapi3filter.ResponseError
	if errors.As(err, &respErr) {
		return http.StatusInternalServerError
	}
	if vErr := validationErrorFrom(err); vErr != nil && vErr.Status > 0 {
		return vErr.Status
	}
	var secErr *openapi3filter.SecurityRequirementsError
	if errors.As(err, &secErr) {
		return http.StatusUnauthorized
	}
	if errors.Is(err, routers.ErrMethodNotAllowed) {
		return http.StatusMethodNotAllowed
	}
	var routeErr *routers.RouteError
	if errors.As(err, &routeErr) {
		return http.StatusNotFound
	}
	return http.StatusBadRequest
}

func defaultErrorHandler(w http.ResponseWriter, _ *http.Request, status int, err error) {
	if status <= 0 {
		status = statusFromOpenAPIError(err)
	}
	p := problemForOpenAPIError(status, err)
	httpx.WriteProblem(w, status, p)
}

func problemForOpenAPIError(status int, err error) httpx.Problem {
	if status <= 0 {
		status = statusFromOpenAPIError(err)
	}
	if status >= http.StatusInternalServerError || isResponseError(err) {
		detail := "response validation failed"
		if errors.Is(err, errResponseTooLarge) {
			detail = "response body exceeds validation limit"
		}
		return httpx.Problem{
			Type:   httpx.DefaultTypeURI(httpx.TypeInternal),
			Title:  http.StatusText(http.StatusInternalServerError),
			Detail: detail,
		}
	}
	if status == http.StatusUnauthorized || isSecurityError(err) {
		return httpx.Problem{
			Type:   httpx.DefaultTypeURI(httpx.TypeUnauthorized),
			Title:  http.StatusText(http.StatusUnauthorized),
			Detail: "authentication required",
		}
	}
	if status == http.StatusForbidden {
		return httpx.Problem{
			Type:   httpx.DefaultTypeURI(httpx.TypeForbidden),
			Title:  http.StatusText(http.StatusForbidden),
			Detail: "forbidden",
		}
	}
	if status == http.StatusNotFound && isRouteError(err) {
		return httpx.Problem{
			Type:   httpx.DefaultTypeURI(httpx.TypeNotFound),
			Title:  http.StatusText(http.StatusNotFound),
			Detail: "route not found",
		}
	}
	if status == http.StatusMethodNotAllowed {
		return httpx.Problem{
			Type:   httpx.DefaultTypeURI(httpx.TypeValidation),
			Title:  http.StatusText(http.StatusMethodNotAllowed),
			Detail: "method not allowed",
		}
	}
	detail := validationDetail(err)
	if detail == "" {
		detail = "request validation failed"
	}
	p := httpx.Problem{
		Type:   httpx.DefaultTypeURI(httpx.TypeValidation),
		Title:  http.StatusText(status),
		Detail: detail,
	}
	if fieldErrs := fieldErrorsFromOpenAPI(err); len(fieldErrs) > 0 {
		p = httpx.WithFieldErrors(p, fieldErrs)
	}
	return p
}

func validationDetail(err error) string {
	if err == nil {
		return ""
	}
	if vErr := validationErrorFrom(err); vErr != nil {
		if vErr.Detail != "" {
			return vErr.Detail
		}
		if vErr.Title != "" {
			return vErr.Title
		}
	}
	var reqErr *openapi3filter.RequestError
	if errors.As(err, &reqErr) && reqErr.Reason != "" {
		return reqErr.Reason
	}
	return ""
}

func fieldErrorsFromOpenAPI(err error) fielderrors.FieldErrors {
	if err == nil {
		return nil
	}
	if vErr := validationErrorFrom(err); vErr != nil {
		if field := fieldFromValidationSource(vErr.Source); field != "" {
			return fielderrors.FieldErrors{{
				Field:   field,
				Code:    codeFromOpenAPIError(err),
				Message: messageFromOpenAPIError(err),
			}}
		}
	}
	var reqErr *openapi3filter.RequestError
	if errors.As(err, &reqErr) {
		field := fieldFromRequestError(reqErr)
		if field != "" {
			return fielderrors.FieldErrors{{
				Field:   field,
				Code:    codeFromOpenAPIError(reqErr.Err),
				Message: messageFromOpenAPIError(reqErr),
			}}
		}
	}
	return nil
}

func fieldFromValidationSource(src *openapi3filter.ValidationErrorSource) string {
	if src == nil {
		return ""
	}
	if src.Parameter != "" {
		return strings.TrimSpace(src.Parameter)
	}
	if src.Pointer != "" {
		return pointerToField("body", src.Pointer)
	}
	return ""
}

func fieldFromRequestError(err *openapi3filter.RequestError) string {
	if err == nil {
		return ""
	}
	if err.Parameter != nil {
		return fieldFromParameter(err.Parameter)
	}
	if err.RequestBody != nil {
		if pointer := schemaPointerFromError(err.Err); pointer != "" {
			return pointerToField("body", pointer)
		}
		return "body"
	}
	return ""
}

func fieldFromParameter(param *openapi3.Parameter) string {
	if param == nil {
		return ""
	}
	name := strings.TrimSpace(param.Name)
	if name == "" {
		return ""
	}
	loc := strings.TrimSpace(param.In)
	if loc == "" {
		return name
	}
	return loc + "." + name
}

func schemaPointerFromError(err error) string {
	var schemaErr *openapi3.SchemaError
	if errors.As(err, &schemaErr) {
		pointer := schemaErr.JSONPointer()
		if len(pointer) == 0 {
			return ""
		}
		return "/" + strings.Join(pointer, "/")
	}
	return ""
}

func pointerToField(prefix, pointer string) string {
	pointer = strings.TrimSpace(pointer)
	pointer = strings.TrimPrefix(pointer, "#")
	pointer = strings.TrimPrefix(pointer, "/")
	if pointer == "" {
		return prefix
	}
	parts := strings.Split(pointer, "/")
	for i, part := range parts {
		parts[i] = decodePointerSegment(part)
	}
	if prefix == "" {
		return strings.Join(parts, ".")
	}
	return prefix + "." + strings.Join(parts, ".")
}

func decodePointerSegment(segment string) string {
	segment = strings.ReplaceAll(segment, "~1", "/")
	segment = strings.ReplaceAll(segment, "~0", "~")
	return segment
}

func validationErrorFrom(err error) *openapi3filter.ValidationError {
	if err == nil {
		return nil
	}
	var vErr *openapi3filter.ValidationError
	if conv := openapi3filter.ConvertErrors(err); errors.As(conv, &vErr) {
		return vErr
	}
	if errors.As(err, &vErr) {
		return vErr
	}
	return nil
}

func codeFromOpenAPIError(err error) string {
	if err == nil {
		return "invalid"
	}
	if errors.Is(err, openapi3filter.ErrInvalidRequired) {
		return "required"
	}
	if errors.Is(err, openapi3filter.ErrInvalidEmptyValue) {
		return "empty"
	}
	var parseErr *openapi3filter.ParseError
	if errors.As(err, &parseErr) {
		switch parseErr.Kind {
		case openapi3filter.KindOther:
			return "invalid"
		case openapi3filter.KindUnsupportedFormat:
			return "unsupported_format"
		case openapi3filter.KindInvalidFormat:
			return "invalid_format"
		}
	}
	var schemaErr *openapi3.SchemaError
	if errors.As(err, &schemaErr) && schemaErr.SchemaField != "" {
		return normalizeCode(schemaErr.SchemaField)
	}
	return "invalid"
}

func messageFromOpenAPIError(err error) string {
	if err == nil {
		return "invalid"
	}
	if vErr := validationErrorFrom(err); vErr != nil {
		if vErr.Detail != "" {
			return vErr.Detail
		}
		if vErr.Title != "" {
			return vErr.Title
		}
	}
	var schemaErr *openapi3.SchemaError
	if errors.As(err, &schemaErr) && schemaErr.Reason != "" {
		return schemaErr.Reason
	}
	var parseErr *openapi3filter.ParseError
	if errors.As(err, &parseErr) && parseErr.Reason != "" {
		return parseErr.Reason
	}
	var reqErr *openapi3filter.RequestError
	if errors.As(err, &reqErr) && reqErr.Reason != "" {
		return reqErr.Reason
	}
	return err.Error()
}

func normalizeCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return "invalid"
	}
	code = strings.ToLower(code)
	code = strings.ReplaceAll(code, " ", "_")
	code = strings.ReplaceAll(code, "-", "_")
	if _, err := strconv.Atoi(code); err == nil {
		return "invalid"
	}
	return code
}

func isResponseError(err error) bool {
	var respErr *openapi3filter.ResponseError
	return errors.As(err, &respErr)
}

func isSecurityError(err error) bool {
	var secErr *openapi3filter.SecurityRequirementsError
	return errors.As(err, &secErr)
}

func isRouteError(err error) bool {
	if errors.Is(err, routers.ErrPathNotFound) || errors.Is(err, routers.ErrMethodNotAllowed) {
		return true
	}
	var routeErr *routers.RouteError
	return errors.As(err, &routeErr)
}
