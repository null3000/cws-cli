package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/null3000/cws-cli/internal/api"
)

// --- CWSError formatting tests ---

func TestCWSError_SingleError(t *testing.T) {
	err := &api.CWSError{
		Operation:  "upload",
		HTTPStatus: 400,
		Message:    "Invalid version number in manifest.",
		Hint:       `Bump the "version" field in manifest.json to be higher than the currently published version.`,
	}
	got := err.Error()
	want := "upload failed (HTTP 400): Invalid version number in manifest."
	if got != want {
		t.Errorf("CWSError.Error() = %q, want %q", got, want)
	}
}

func TestCWSError_NoHTTPStatus(t *testing.T) {
	err := &api.CWSError{
		Operation: "upload",
		Message:   "upload processing failed",
	}
	got := err.Error()
	want := "upload failed: upload processing failed"
	if got != want {
		t.Errorf("CWSError.Error() = %q, want %q", got, want)
	}
}

func TestCWSError_MultipleDetails(t *testing.T) {
	err := &api.CWSError{
		Operation:  "upload",
		HTTPStatus: 400,
		Details: []api.ErrorDetail{
			{Code: "PKG_INVALID_VERSION_NUMBER", Detail: "Invalid version number.", Hint: "Bump the version."},
			{Code: "PKG_MANIFEST_PARSE_ERROR", Detail: "Bad manifest.", Hint: "Fix the manifest."},
		},
	}
	got := err.Error()
	if !strings.Contains(got, "[PKG_INVALID_VERSION_NUMBER]") {
		t.Errorf("expected error to contain first error code, got: %s", got)
	}
	if !strings.Contains(got, "[PKG_MANIFEST_PARSE_ERROR]") {
		t.Errorf("expected error to contain second error code, got: %s", got)
	}
	if !strings.Contains(got, "Hint: Bump the version.") {
		t.Errorf("expected error to contain first hint, got: %s", got)
	}
	if !strings.Contains(got, "Hint: Fix the manifest.") {
		t.Errorf("expected error to contain second hint, got: %s", got)
	}
}

func TestCWSError_MultipleDetails_HintsInline(t *testing.T) {
	// When there are multiple details, hints should be inline in Error() output
	// and root.go should NOT add a duplicate hint (it checks len(Details) <= 1)
	cwsErr := &api.CWSError{
		Operation:  "upload",
		HTTPStatus: 400,
		Hint:       "Bump the version.", // This would be set by NewCWSError
		Details: []api.ErrorDetail{
			{Code: "PKG_INVALID_VERSION_NUMBER", Detail: "Bad version", Hint: "Bump the version."},
			{Code: "PKG_MANIFEST_PARSE_ERROR", Detail: "Bad manifest", Hint: "Fix the manifest."},
		},
	}

	// Verify hints are inline in Error()
	got := cwsErr.Error()
	if strings.Count(got, "Hint: Bump the version.") != 1 {
		t.Errorf("expected exactly one occurrence of first hint in Error(), got:\n%s", got)
	}

	// Verify that root.go's condition would skip the catch-all hint
	if len(cwsErr.Details) <= 1 {
		t.Error("expected Details > 1 so root.go skips duplicate hint")
	}
}

func TestCWSError_ImplementsError(t *testing.T) {
	var err error = &api.CWSError{Operation: "test", Message: "fail"}
	if err.Error() != "test failed: fail" {
		t.Errorf("unexpected: %s", err.Error())
	}
}

func TestCWSError_ErrorsAs(t *testing.T) {
	inner := &api.CWSError{Operation: "upload", HTTPStatus: 400, Message: "bad", Hint: "fix it"}
	wrapped := errors.New("upload succeeded but publish failed: " + inner.Error())
	// Test that a directly returned CWSError can be extracted
	var cwsErr *api.CWSError
	if !errors.As(inner, &cwsErr) {
		t.Fatal("errors.As should find CWSError directly")
	}
	if cwsErr.Hint != "fix it" {
		t.Errorf("Hint = %q, want %q", cwsErr.Hint, "fix it")
	}
	// Wrapped with fmt.Errorf("%w") would work too, but plain errors.New doesn't
	if errors.As(wrapped, &cwsErr) {
		t.Fatal("errors.As should not find CWSError in non-%w wrapped error")
	}
}

// --- Hint lookup tests ---

func TestHintForItemError_KnownCode(t *testing.T) {
	hint := api.HintForItemError("PKG_INVALID_VERSION_NUMBER")
	if hint == "" {
		t.Fatal("expected hint for PKG_INVALID_VERSION_NUMBER, got empty")
	}
	if !strings.Contains(hint, "version") {
		t.Errorf("hint should mention version, got: %s", hint)
	}
}

func TestHintForItemError_UnknownCode(t *testing.T) {
	hint := api.HintForItemError("TOTALLY_UNKNOWN_CODE")
	if hint != "" {
		t.Errorf("expected empty hint for unknown code, got: %s", hint)
	}
}

func TestHintForHTTPStatus_401(t *testing.T) {
	hint := api.HintForHTTPStatus(401)
	if hint == "" {
		t.Fatal("expected hint for 401")
	}
	if !strings.Contains(hint, "cws init") {
		t.Errorf("hint should suggest cws init, got: %s", hint)
	}
}

func TestHintForHTTPStatus_403(t *testing.T) {
	hint := api.HintForHTTPStatus(403)
	if hint == "" {
		t.Fatal("expected hint for 403")
	}
}

func TestHintForHTTPStatus_429(t *testing.T) {
	hint := api.HintForHTTPStatus(429)
	if hint == "" {
		t.Fatal("expected hint for 429")
	}
	if !strings.Contains(strings.ToLower(hint), "rate") {
		t.Errorf("hint should mention rate limiting, got: %s", hint)
	}
}

func TestHintForHTTPStatus_Unknown(t *testing.T) {
	hint := api.HintForHTTPStatus(418)
	if hint != "" {
		t.Errorf("expected empty hint for unknown status, got: %s", hint)
	}
}

func TestHintForPublishStatus_KnownCodes(t *testing.T) {
	codes := []string{
		"NOT_AUTHORIZED", "INVALID_DEVELOPER", "DEVELOPER_NO_OWNERSHIP",
		"DEVELOPER_SUSPENDED", "ITEM_NOT_FOUND", "ITEM_PENDING_REVIEW",
		"ITEM_TAKEN_DOWN", "PUBLISHER_SUSPENDED",
	}
	for _, code := range codes {
		hint := api.HintForPublishStatus(code)
		if hint == "" {
			t.Errorf("expected hint for publish status %q, got empty", code)
		}
	}
}

func TestHintForPublishStatus_Unknown(t *testing.T) {
	hint := api.HintForPublishStatus("OK")
	if hint != "" {
		t.Errorf("expected empty hint for OK status, got: %s", hint)
	}
}

func TestHintForMessage_PublishConditionNotMet(t *testing.T) {
	hint := api.HintForMessage("Publish condition not met:")
	if hint == "" {
		t.Fatal("expected hint for 'Publish condition not met'")
	}
	if !strings.Contains(hint, "dashboard") {
		t.Errorf("hint should point to dashboard, got: %s", hint)
	}
}

func TestHintForMessage_VersionNumber(t *testing.T) {
	hint := api.HintForMessage("Invalid version number in manifest")
	if hint == "" {
		t.Fatal("expected hint for version number message")
	}
}

func TestHintForMessage_NoMatch(t *testing.T) {
	hint := api.HintForMessage("something completely unrelated")
	if hint != "" {
		t.Errorf("expected empty hint for unrecognized message, got: %s", hint)
	}
}

func TestResolveHint_PriorityOrder(t *testing.T) {
	// ItemError code should take precedence over HTTP status
	hint := api.ResolveHint("PKG_INVALID_VERSION_NUMBER", 401, "some message")
	itemHint := api.HintForItemError("PKG_INVALID_VERSION_NUMBER")
	if hint != itemHint {
		t.Errorf("ResolveHint should prefer item error hint, got: %s", hint)
	}

	// HTTP status should be used when no item error code
	hint = api.ResolveHint("", 401, "some message")
	httpHint := api.HintForHTTPStatus(401)
	if hint != httpHint {
		t.Errorf("ResolveHint should fall back to HTTP hint, got: %s", hint)
	}

	// Message pattern should be used as last resort
	hint = api.ResolveHint("", 0, "Publish condition not met: something")
	if hint == "" {
		t.Fatal("ResolveHint should fall back to message pattern hint")
	}
}

// --- ParseAPIErrorDetail tests ---

func TestParseAPIErrorDetail_ExtractsReasons(t *testing.T) {
	body := []byte(`{
		"error": {
			"code": 400,
			"message": "The uploaded package was invalid.",
			"status": "INVALID_ARGUMENT",
			"details": [
				{
					"@type": "type.googleapis.com/google.rpc.BadRequest",
					"fieldViolations": [
						{
							"field": "media",
							"description": "Invalid version number in manifest.",
							"reason": "PKG_INVALID_VERSION_NUMBER"
						}
					]
				}
			]
		}
	}`)

	parsed := api.ParseAPIErrorDetail(body)
	if parsed == nil {
		t.Fatal("ParseAPIErrorDetail returned nil")
	}
	if parsed.Status != "INVALID_ARGUMENT" {
		t.Errorf("Status = %q, want INVALID_ARGUMENT", parsed.Status)
	}
	if len(parsed.Reasons) != 1 || parsed.Reasons[0] != "PKG_INVALID_VERSION_NUMBER" {
		t.Errorf("Reasons = %v, want [PKG_INVALID_VERSION_NUMBER]", parsed.Reasons)
	}
	if parsed.Description != "Invalid version number in manifest." {
		t.Errorf("Description = %q, want 'Invalid version number in manifest.'", parsed.Description)
	}
}

func TestParseAPIErrorDetail_MultipleViolations(t *testing.T) {
	body := []byte(`{
		"error": {
			"code": 400,
			"message": "Multiple errors",
			"details": [
				{
					"@type": "type.googleapis.com/google.rpc.BadRequest",
					"fieldViolations": [
						{"field": "a", "description": "first", "reason": "REASON_A"},
						{"field": "b", "description": "second", "reason": "REASON_B"}
					]
				}
			]
		}
	}`)

	parsed := api.ParseAPIErrorDetail(body)
	if parsed == nil {
		t.Fatal("ParseAPIErrorDetail returned nil")
	}
	if len(parsed.Reasons) != 2 {
		t.Fatalf("expected 2 reasons, got %d", len(parsed.Reasons))
	}
	if parsed.Reasons[0] != "REASON_A" || parsed.Reasons[1] != "REASON_B" {
		t.Errorf("Reasons = %v, want [REASON_A, REASON_B]", parsed.Reasons)
	}
	// Description should be the last violation's description
	if parsed.Description != "second" {
		t.Errorf("Description = %q, want 'second'", parsed.Description)
	}
}

func TestParseAPIErrorDetail_NoViolations(t *testing.T) {
	body := []byte(`{
		"error": {
			"code": 403,
			"message": "Forbidden",
			"status": "PERMISSION_DENIED"
		}
	}`)

	parsed := api.ParseAPIErrorDetail(body)
	if parsed == nil {
		t.Fatal("ParseAPIErrorDetail returned nil")
	}
	if len(parsed.Reasons) != 0 {
		t.Errorf("expected no reasons, got %v", parsed.Reasons)
	}
	// Description should fall back to message
	if parsed.Description != "Forbidden" {
		t.Errorf("Description = %q, want 'Forbidden'", parsed.Description)
	}
}

func TestParseAPIErrorDetail_InvalidJSON(t *testing.T) {
	parsed := api.ParseAPIErrorDetail([]byte(`not json`))
	if parsed != nil {
		t.Error("expected nil for invalid JSON")
	}
}

// --- Integration tests: API endpoints return CWSError with hints ---

func TestUpload_400_ItemError_WithHint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]any{
			"uploadState": "FAILURE",
			"itemError": []map[string]any{
				{
					"error_code":   "PKG_INVALID_VERSION_NUMBER",
					"error_detail": "Invalid version number in manifest: 1.0.0",
				},
			},
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.Upload(context.Background(), "ext123", []byte("zipdata"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var cwsErr *api.CWSError
	if !errors.As(err, &cwsErr) {
		t.Fatalf("expected CWSError, got %T: %v", err, err)
	}
	if cwsErr.Hint == "" {
		t.Error("expected non-empty hint for PKG_INVALID_VERSION_NUMBER")
	}
	if !strings.Contains(cwsErr.Hint, "version") {
		t.Errorf("hint should mention version, got: %s", cwsErr.Hint)
	}
}

func TestUpload_400_MultipleItemErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]any{
			"uploadState": "FAILURE",
			"itemError": []map[string]any{
				{"error_code": "PKG_INVALID_VERSION_NUMBER", "error_detail": "Bad version"},
				{"error_code": "PKG_MANIFEST_PARSE_ERROR", "error_detail": "Bad manifest"},
			},
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.Upload(context.Background(), "ext123", []byte("zipdata"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	got := err.Error()
	if !strings.Contains(got, "PKG_INVALID_VERSION_NUMBER") {
		t.Errorf("error should contain first error code, got: %s", got)
	}
	if !strings.Contains(got, "PKG_MANIFEST_PARSE_ERROR") {
		t.Errorf("error should contain second error code, got: %s", got)
	}
}

func TestUpload_401_AuthHint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    401,
				"message": "Request had invalid authentication credentials.",
			},
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.Upload(context.Background(), "ext123", []byte("zipdata"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var cwsErr *api.CWSError
	if !errors.As(err, &cwsErr) {
		t.Fatalf("expected CWSError, got %T: %v", err, err)
	}
	if !strings.Contains(cwsErr.Hint, "cws init") {
		t.Errorf("hint should suggest cws init for 401, got: %s", cwsErr.Hint)
	}
}

func TestPublish_VagueConditionNotMet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    400,
				"message": "Publish condition not met:",
			},
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.Publish(context.Background(), "ext123", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var cwsErr *api.CWSError
	if !errors.As(err, &cwsErr) {
		t.Fatalf("expected CWSError, got %T: %v", err, err)
	}
	if !strings.Contains(cwsErr.Hint, "dashboard") {
		t.Errorf("hint should point to dashboard for vague publish error, got: %s", cwsErr.Hint)
	}
}

func TestPublish_StatusCode_NotAuthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(map[string]any{
			"statusCode": "NOT_AUTHORIZED",
			"status":     []string{"NOT_AUTHORIZED"},
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.Publish(context.Background(), "ext123", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var cwsErr *api.CWSError
	if !errors.As(err, &cwsErr) {
		t.Fatalf("expected CWSError, got %T: %v", err, err)
	}
	if cwsErr.Code != "NOT_AUTHORIZED" {
		t.Errorf("Code = %q, want NOT_AUTHORIZED", cwsErr.Code)
	}
	if cwsErr.Hint == "" {
		t.Error("expected non-empty hint for NOT_AUTHORIZED")
	}
}

func TestFetchStatus_404_HasHint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"error": {"code": 404, "message": "not found"}}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, _, err := client.FetchStatus(context.Background(), "badext")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var cwsErr *api.CWSError
	if !errors.As(err, &cwsErr) {
		t.Fatalf("expected CWSError, got %T: %v", err, err)
	}
	if cwsErr.Hint == "" {
		t.Error("expected non-empty hint for 404")
	}
}

func TestNewCWSError_FromItemErrors(t *testing.T) {
	itemErrors := []api.ItemError{
		{ErrorCode: "PKG_INVALID_VERSION_NUMBER", ErrorDetail: "Bad version"},
	}
	cwsErr := api.NewCWSError("upload", 400, itemErrors, "")
	if cwsErr.Code != "PKG_INVALID_VERSION_NUMBER" {
		t.Errorf("Code = %q, want PKG_INVALID_VERSION_NUMBER", cwsErr.Code)
	}
	if cwsErr.Hint == "" {
		t.Error("expected non-empty hint")
	}
}

func TestNewCWSError_FallbackMessage(t *testing.T) {
	cwsErr := api.NewCWSError("upload", 0, nil, "upload processing failed")
	if cwsErr.Message != "upload processing failed" {
		t.Errorf("Message = %q, want 'upload processing failed'", cwsErr.Message)
	}
}
