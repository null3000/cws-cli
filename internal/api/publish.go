package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Publish publishes the most recently uploaded version.
func (c *Client) Publish(ctx context.Context, extensionID string, staged bool) (*PublishResponse, error) {
	path := c.itemPath(extensionID, "publish")

	reqBody := &PublishRequest{}
	if staged {
		reqBody.PublishType = "STAGED_PUBLISH"
	}

	respBody, statusCode, err := c.doJSON(ctx, "POST", path, reqBody)
	if err != nil {
		return nil, err
	}

	var resp PublishResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse publish response (HTTP %d): %s", statusCode, truncateBody(respBody, 200))
	}

	if statusCode < 200 || statusCode >= 300 {
		cwsErr := &CWSError{
			Operation:  "publish",
			HTTPStatus: statusCode,
		}

		// Build the message from available fields
		if resp.StatusCode != "" {
			cwsErr.Code = resp.StatusCode
			cwsErr.Hint = HintForPublishStatus(resp.StatusCode)
		}
		if len(resp.Status) > 0 {
			cwsErr.Message = strings.Join(resp.Status, ", ")
		}

		// Fall back to parsing the API error body
		if cwsErr.Message == "" {
			if detail := ParseAPIError(respBody); detail != "" {
				cwsErr.Message = detail
			}
		}

		// Resolve hint if not already set from publish status code
		if cwsErr.Hint == "" {
			cwsErr.Hint = ResolveHint("", statusCode, cwsErr.Message)
		}

		return &resp, cwsErr
	}

	return &resp, nil
}
