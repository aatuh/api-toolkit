package opa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/ports"
)

const defaultTimeout = 5 * time.Second

// Config configures the OPA adapter.
type Config struct {
	DecisionURL  string
	BaseURL      string
	DecisionPath string
	Headers      map[string]string
	Client       *http.Client
	Timeout      time.Duration
	ResultKey    string
}

// Client evaluates policies through OPA's REST API.
type Client struct {
	url       string
	headers   map[string]string
	client    *http.Client
	resultKey string
}

// New creates a new OPA adapter.
func New(cfg Config) (*Client, error) {
	decisionURL, err := resolveDecisionURL(cfg)
	if err != nil {
		return nil, err
	}
	client := cfg.Client
	if client == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = defaultTimeout
		}
		client = &http.Client{Timeout: timeout}
	}
	headers := make(map[string]string, len(cfg.Headers))
	for k, v := range cfg.Headers {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		headers[key] = strings.TrimSpace(v)
	}
	return &Client{
		url:       decisionURL,
		headers:   headers,
		client:    client,
		resultKey: strings.TrimSpace(cfg.ResultKey),
	}, nil
}

// Evaluate calls OPA with the given policy request.
func (c *Client) Evaluate(ctx context.Context, req ports.PolicyRequest) (ports.PolicyDecision, error) {
	if c == nil {
		return ports.PolicyDecision{}, errors.New("opa client is nil")
	}
	if ctx == nil {
		return ports.PolicyDecision{}, errors.New("context is nil")
	}
	input := map[string]any{}
	if req.Subject != nil {
		input["subject"] = req.Subject
	}
	if req.Action != "" {
		input["action"] = req.Action
	}
	if req.Resource != nil {
		input["resource"] = req.Resource
	}
	if req.Context != nil {
		input["context"] = req.Context
	}
	body, err := json.Marshal(map[string]any{"input": input})
	if err != nil {
		return ports.PolicyDecision{}, fmt.Errorf("opa request marshal: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return ports.PolicyDecision{}, fmt.Errorf("opa request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return ports.PolicyDecision{}, fmt.Errorf("opa request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return ports.PolicyDecision{}, fmt.Errorf("opa decision failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(detail)))
	}
	var payload struct {
		Result any `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ports.PolicyDecision{}, fmt.Errorf("opa response decode: %w", err)
	}
	allow, err := parseResult(payload.Result, c.resultKey)
	if err != nil {
		return ports.PolicyDecision{}, err
	}
	return ports.PolicyDecision{Allow: allow, Data: payload.Result}, nil
}

func resolveDecisionURL(cfg Config) (string, error) {
	if cfg.DecisionURL != "" {
		return strings.TrimSpace(cfg.DecisionURL), nil
	}
	base := strings.TrimSpace(cfg.BaseURL)
	path := strings.TrimSpace(cfg.DecisionPath)
	if base == "" || path == "" {
		return "", errors.New("opa decision url not configured")
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/"), nil
}

func parseResult(result any, key string) (bool, error) {
	if result == nil {
		return false, errors.New("opa result is empty")
	}
	if allow, ok := result.(bool); ok {
		return allow, nil
	}
	if resultMap, ok := result.(map[string]any); ok {
		if key != "" {
			val, ok := resultMap[key]
			if ok {
				allow, ok := val.(bool)
				if ok {
					return allow, nil
				}
				return false, errors.New("opa result key is not boolean")
			}
			return false, errors.New("opa result key not found")
		}
		if allow, ok := resultMap["allow"].(bool); ok {
			return allow, nil
		}
	}
	return false, errors.New("opa result is not boolean")
}
