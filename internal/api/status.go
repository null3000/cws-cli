package api

import (
	"context"
	"encoding/json"
	"fmt"
)

// FetchStatus retrieves the current status of an extension.
func (c *Client) FetchStatus(ctx context.Context, extensionID string) (*StatusResponse, []byte, error) {
	path := c.itemPath(extensionID, "fetchStatus")

	respBody, statusCode, err := c.doJSON(ctx, "GET", path, nil)
	if err != nil {
		return nil, nil, err
	}

	if statusCode == 404 {
		return nil, nil, &CWSError{
			Operation:  "status check",
			HTTPStatus: 404,
			Message:    fmt.Sprintf("extension not found. Verify the extension ID: %s", extensionID),
			Hint:       "Double-check the extension ID in your cws.toml or --extension-id flag.",
		}
	}

	var resp StatusResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, nil, fmt.Errorf("failed to parse status response (HTTP %d): %s", statusCode, truncateBody(respBody, 200))
	}

	if statusCode < 200 || statusCode >= 300 {
		if len(resp.ItemError) > 0 {
			return &resp, respBody, NewCWSError("status check", statusCode, resp.ItemError, "")
		}
		parsed := ParseAPIErrorDetail(respBody)
		if parsed != nil {
			cwsErr := &CWSError{
				Operation:  "status check",
				HTTPStatus: statusCode,
				Message:    parsed.Description,
				Hint:       ResolveHint("", statusCode, parsed.Description),
			}
			return &resp, respBody, cwsErr
		}
		return &resp, respBody, &CWSError{
			Operation:  "status check",
			HTTPStatus: statusCode,
			Message:    string(respBody),
			Hint:       HintForHTTPStatus(statusCode),
		}
	}

	return &resp, respBody, nil
}
