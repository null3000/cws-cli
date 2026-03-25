package api

import (
	"context"
	"encoding/json"
	"fmt"
)

// CancelSubmission cancels a pending submission.
func (c *Client) CancelSubmission(ctx context.Context, extensionID string) (*CancelResponse, error) {
	path := c.itemPath(extensionID, "cancelSubmission")

	respBody, statusCode, err := c.doJSON(ctx, "POST", path, nil)
	if err != nil {
		return nil, err
	}

	var resp CancelResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse cancel response (HTTP %d): %s", statusCode, truncateBody(respBody, 200))
	}

	if statusCode < 200 || statusCode >= 300 {
		msg := ParseAPIError(respBody)
		if msg == "" {
			msg = "no pending submission to cancel for this extension"
		}
		return &resp, &CWSError{
			Operation:  "cancel",
			HTTPStatus: statusCode,
			Message:    msg,
			Hint:       ResolveHint("", statusCode, msg),
		}
	}

	return &resp, nil
}
