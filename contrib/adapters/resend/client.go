package resend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/v4/email"
)

const defaultBaseURL = "https://api.resend.com"

const defaultHTTPTimeout = 10 * time.Second

// Client implements email.Sender using the Resend API.
type Client struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// Option customizes Client behavior.
type Option func(*Client)

// WithBaseURL overrides the Resend API base URL.
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.BaseURL = strings.TrimRight(url, "/")
	}
}

// WithHTTPClient overrides the HTTP client used for requests.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.HTTPClient = client
	}
}

// New constructs a Resend API client.
func New(apiKey string, opts ...Option) *Client {
	client := &Client{
		APIKey:     strings.TrimSpace(apiKey),
		BaseURL:    defaultBaseURL,
		HTTPClient: &http.Client{Timeout: defaultHTTPTimeout},
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

type sendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text,omitempty"`
	HTML    string   `json:"html,omitempty"`
	ReplyTo string   `json:"reply_to,omitempty"`
}

type sendResponse struct {
	ID string `json:"id"`
}

// Send sends an email via the Resend API.
func (c *Client) Send(ctx context.Context, msg email.Message) (string, error) {
	if c == nil || c.APIKey == "" {
		return "", fmt.Errorf("missing resend api key")
	}
	payload := sendRequest{
		From:    msg.From,
		To:      msg.To,
		Subject: msg.Subject,
		Text:    msg.Text,
		HTML:    msg.HTML,
		ReplyTo: msg.ReplyTo,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("resend error: %s", strings.TrimSpace(string(respBody)))
	}
	if len(respBody) == 0 {
		return "", fmt.Errorf("resend success response missing body")
	}
	var out sendResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("decode resend response: %w", err)
	}
	if strings.TrimSpace(out.ID) == "" {
		return "", fmt.Errorf("resend success response missing id")
	}
	return out.ID, nil
}

var _ email.Sender = (*Client)(nil)
