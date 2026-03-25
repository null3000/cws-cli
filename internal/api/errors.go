package api

import (
	"fmt"
	"strings"
)

// CWSError is a structured error returned by Chrome Web Store API operations.
// It preserves the original API message and attaches an actionable hint when possible.
type CWSError struct {
	Operation  string        // "upload", "publish", "status", "rollout", "cancel"
	HTTPStatus int           // HTTP status code from the API response
	Message    string        // original API error message, preserved verbatim
	Code       string        // primary error code (e.g., "PKG_INVALID_VERSION_NUMBER")
	Hint       string        // actionable suggestion for the user
	Details    []ErrorDetail // all errors when multiple ItemErrors are present
}

// ErrorDetail represents a single error entry from an ItemError array.
type ErrorDetail struct {
	Code   string
	Detail string
	Hint   string
}

func (e *CWSError) Error() string {
	if len(e.Details) > 1 {
		var b strings.Builder
		fmt.Fprintf(&b, "%s failed", e.Operation)
		if e.HTTPStatus > 0 {
			fmt.Fprintf(&b, " (HTTP %d)", e.HTTPStatus)
		}
		for _, d := range e.Details {
			if d.Code != "" {
				fmt.Fprintf(&b, "\n  - [%s] %s", d.Code, d.Detail)
			} else {
				fmt.Fprintf(&b, "\n  - %s", d.Detail)
			}
			if d.Hint != "" {
				fmt.Fprintf(&b, "\n    Hint: %s", d.Hint)
			}
		}
		return b.String()
	}

	msg := fmt.Sprintf("%s failed", e.Operation)
	if e.HTTPStatus > 0 {
		msg = fmt.Sprintf("%s failed (HTTP %d)", e.Operation, e.HTTPStatus)
	}
	if e.Message != "" {
		msg += ": " + e.Message
	}
	return msg
}

// itemErrorHints maps Chrome Web Store ItemError error_code values to actionable hints.
var itemErrorHints = map[string]string{
	"PKG_INVALID_VERSION_NUMBER":   `Bump the "version" field in manifest.json to be higher than the currently published version.`,
	"PKG_MANIFEST_PARSE_ERROR":     "Check that manifest.json is valid JSON and conforms to the Chrome extension manifest format.",
	"PKG_INTERNAL_ERROR_PACKAGING": "This is a Chrome Web Store system error. Try reducing your package size or re-uploading. If it persists, try again later.",
	"PKG_INVALID_ZIP":              "The uploaded file is not a valid ZIP archive. Re-zip your extension directory and try again.",
	"PKG_MISSING_MANIFEST":         "The uploaded package does not contain a manifest.json at the root level.",
}

// publishStatusHints maps Chrome Web Store publish status codes to actionable hints.
var publishStatusHints = map[string]string{
	"NOT_AUTHORIZED":        "You don't have permission to publish this extension. Verify your OAuth credentials belong to the extension owner.",
	"INVALID_DEVELOPER":     "The developer account is invalid. Verify your Chrome Web Store developer registration at https://chrome.google.com/webstore/devconsole",
	"DEVELOPER_NO_OWNERSHIP": "Your account does not own this extension. Verify the extension ID and publisher ID are correct.",
	"DEVELOPER_SUSPENDED":   "Your developer account has been suspended. Check the Chrome Web Store developer dashboard for details.",
	"ITEM_NOT_FOUND":        "The extension was not found. Verify the extension ID is correct.",
	"ITEM_PENDING_REVIEW":   "The extension is already pending review. Wait for the review to complete, or use 'cws cancel' to cancel the pending submission first.",
	"ITEM_TAKEN_DOWN":       "The extension has been taken down. Check the Chrome Web Store developer dashboard for details.",
	"PUBLISHER_SUSPENDED":   "The publisher account has been suspended. Check the Chrome Web Store developer dashboard for details.",
}

// httpStatusHints maps HTTP status codes to actionable hints.
var httpStatusHints = map[int]string{
	401: "Your OAuth token is invalid or expired. Run 'cws init' to reconfigure credentials.",
	403: "Permission denied. Verify your OAuth credentials have access to this extension, or run 'cws init' to reconfigure.",
	429: "Rate limited by the Chrome Web Store API. Wait a few minutes and try again.",
}

// HintForItemError returns an actionable hint for a known ItemError error_code.
func HintForItemError(code string) string {
	return itemErrorHints[code]
}

// HintForHTTPStatus returns an actionable hint for a known HTTP status code.
func HintForHTTPStatus(statusCode int) string {
	return httpStatusHints[statusCode]
}

// HintForPublishStatus returns an actionable hint for a known publish status code.
func HintForPublishStatus(statusCode string) string {
	return publishStatusHints[statusCode]
}

// HintForMessage returns an actionable hint by pattern-matching against the error message.
// This is used as a fallback for vague API errors that lack structured error codes.
func HintForMessage(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "publish condition not met"):
		return "The API did not provide a specific reason. Check the Chrome Web Store developer dashboard for details: https://chrome.google.com/webstore/devconsole"
	case strings.Contains(lower, "version number"):
		return `Bump the "version" field in manifest.json to be higher than the currently published version.`
	case strings.Contains(lower, "already in a pending"):
		return "A previous version is still being reviewed. Wait for it to complete, or use 'cws cancel' to cancel it first."
	default:
		return ""
	}
}

// ResolveHint tries all hint sources in priority order and returns the first match.
func ResolveHint(errorCode string, httpStatus int, message string) string {
	if h := HintForItemError(errorCode); h != "" {
		return h
	}
	if h := HintForHTTPStatus(httpStatus); h != "" {
		return h
	}
	if h := HintForMessage(message); h != "" {
		return h
	}
	return ""
}

// NewCWSError creates a CWSError from an operation, HTTP status, and ItemError list.
func NewCWSError(operation string, httpStatus int, itemErrors []ItemError, fallbackMsg string) *CWSError {
	cwsErr := &CWSError{
		Operation:  operation,
		HTTPStatus: httpStatus,
	}

	if len(itemErrors) > 0 {
		for _, ie := range itemErrors {
			cwsErr.Details = append(cwsErr.Details, ErrorDetail{
				Code:   ie.ErrorCode,
				Detail: ie.ErrorDetail,
				Hint:   HintForItemError(ie.ErrorCode),
			})
		}
		// Use first error as the primary message/code/hint
		cwsErr.Code = itemErrors[0].ErrorCode
		cwsErr.Message = itemErrors[0].ErrorDetail
		cwsErr.Hint = ResolveHint(itemErrors[0].ErrorCode, httpStatus, itemErrors[0].ErrorDetail)
	} else if fallbackMsg != "" {
		cwsErr.Message = fallbackMsg
		cwsErr.Hint = ResolveHint("", httpStatus, fallbackMsg)
	} else {
		cwsErr.Hint = HintForHTTPStatus(httpStatus)
	}

	return cwsErr
}
