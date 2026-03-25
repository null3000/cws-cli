package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// Upload uploads a zip file to the Chrome Web Store.
func (c *Client) Upload(ctx context.Context, extensionID string, zipData []byte) (*UploadResponse, error) {
	path := c.uploadPath(extensionID)
	respBody, statusCode, err := c.doRequest(ctx, "POST", path, bytes.NewReader(zipData), "application/zip")
	if err != nil {
		return nil, err
	}

	var resp UploadResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse upload response (HTTP %d): %s", statusCode, truncateBody(respBody, 200))
	}

	if statusCode < 200 || statusCode >= 300 {
		if len(resp.ItemError) > 0 {
			return &resp, NewCWSError("upload", statusCode, resp.ItemError, "")
		}
		parsed := ParseAPIErrorDetail(respBody)
		if parsed != nil {
			// Use reason codes from field violations as item error codes for hint resolution
			cwsErr := &CWSError{
				Operation:  "upload",
				HTTPStatus: statusCode,
				Message:    parsed.Description,
			}
			if len(parsed.Reasons) > 0 {
				cwsErr.Code = parsed.Reasons[0]
				cwsErr.Hint = ResolveHint(parsed.Reasons[0], statusCode, parsed.Description)
			} else {
				cwsErr.Hint = ResolveHint("", statusCode, parsed.Description)
			}
			return &resp, cwsErr
		}
		return &resp, &CWSError{
			Operation:  "upload",
			HTTPStatus: statusCode,
			Message:    string(respBody),
			Hint:       HintForHTTPStatus(statusCode),
		}
	}

	return &resp, nil
}
