package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/null3000/cws-cli/internal/auth"
)

const defaultBaseURL = "https://chromewebstore.googleapis.com"

// Client is the Chrome Web Store API V2 client.
type Client struct {
	httpClient  *http.Client
	auth        auth.Authenticator
	publisherID string
	BaseURL     string // override for testing; empty uses default
}

func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return defaultBaseURL
}

// NewClient creates a new API client.
func NewClient(authenticator auth.Authenticator, publisherID string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		auth:        authenticator,
		publisherID: publisherID,
	}
}

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, contentType string) ([]byte, int, error) {
	token, err := c.auth.AccessToken(ctx)
	if err != nil {
		return nil, 0, err
	}

	url := c.baseURL() + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w. Check your network connection and try again", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, reqBody any) ([]byte, int, error) {
	var body io.Reader
	var contentType string
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewReader(data)
		contentType = "application/json"
	}
	return c.doRequest(ctx, method, path, body, contentType)
}

// ParsedAPIError holds structured information extracted from a Google API error response.
type ParsedAPIError struct {
	Message     string   // top-level error message
	StatusCode  int      // HTTP-like code from the error body
	Status      string   // status string (e.g., "INVALID_ARGUMENT")
	Reasons     []string // reason codes from field violations (e.g., "PKG_INVALID_VERSION_NUMBER")
	Description string   // most specific description found
}

// ParseAPIErrorDetail extracts structured error information from a Google API error response.
// Returns nil if the body is not a recognized error format.
func ParseAPIErrorDetail(body []byte) *ParsedAPIError {
	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil || apiErr.Error == nil {
		return nil
	}

	parsed := &ParsedAPIError{
		Message:    apiErr.Error.Message,
		StatusCode: apiErr.Error.Code,
		Status:     apiErr.Error.Status,
	}

	for _, d := range apiErr.Error.Details {
		for _, v := range d.FieldViolations {
			if v.Reason != "" {
				parsed.Reasons = append(parsed.Reasons, v.Reason)
			}
			if v.Description != "" {
				parsed.Description = v.Description
			}
		}
	}

	if parsed.Description == "" {
		parsed.Description = parsed.Message
	}

	return parsed
}

// ParseAPIError attempts to extract a human-readable error from a Google API error response.
// Returns an empty string if the body is not a recognized error format.
func ParseAPIError(body []byte) string {
	parsed := ParseAPIErrorDetail(body)
	if parsed == nil {
		return ""
	}
	return parsed.Description
}

// truncateBody returns the response body as a string, truncated to maxLen characters.
func truncateBody(body []byte, maxLen int) string {
	s := string(body)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

func (c *Client) itemPath(extensionID, action string) string {
	return fmt.Sprintf("/v2/publishers/%s/items/%s:%s", c.publisherID, extensionID, action)
}

func (c *Client) uploadPath(extensionID string) string {
	return fmt.Sprintf("/upload/v2/publishers/%s/items/%s:upload", c.publisherID, extensionID)
}
