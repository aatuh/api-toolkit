// Package docs registers stable API documentation and OpenAPI endpoints.
//
// Without a registered provider, the default manager discovers OpenAPI files
// from fixed relative paths under the service working directory, such as
// ./swagger/openapi.json and ./docs/openapi.json. Production services should
// prefer RegisterProvider when the OpenAPI source is known at wiring time.
//
// Custom docs managers can expose HTML-mode-specific handler behavior by
// implementing ports.DocsHTMLModeProvider in addition to ports.DocsManager.
package docs
