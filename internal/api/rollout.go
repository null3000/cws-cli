package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SetDeployPercentage sets the deploy percentage for a published extension.
func (c *Client) SetDeployPercentage(ctx context.Context, extensionID string, percentage int) (*DeployPercentageResponse, error) {
	path := c.itemPath(extensionID, "setPublishedDeployPercentage")

	reqBody := &DeployPercentageRequest{
		DeployPercentage: percentage,
	}

	respBody, statusCode, err := c.doJSON(ctx, "POST", path, reqBody)
	if err != nil {
		return nil, err
	}

	var resp DeployPercentageResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse rollout response (HTTP %d): %s", statusCode, truncateBody(respBody, 200))
	}

	if statusCode < 200 || statusCode >= 300 {
		cwsErr := &CWSError{
			Operation:  "rollout",
			HTTPStatus: statusCode,
		}
		if resp.Status != "" {
			cwsErr.Message = resp.Status
		} else if detail := ParseAPIError(respBody); detail != "" {
			cwsErr.Message = detail
		}

		// Add rollout-specific hint for the common "does not meet requirements" error
		if strings.Contains(strings.ToLower(cwsErr.Message), "does not meet requirements") ||
			strings.Contains(strings.ToLower(cwsErr.Message), "not eligible") {
			cwsErr.Hint = "Partial rollouts require your extension to have at least 10,000 weekly active users."
		} else {
			cwsErr.Hint = ResolveHint("", statusCode, cwsErr.Message)
		}

		return &resp, cwsErr
	}

	return &resp, nil
}
