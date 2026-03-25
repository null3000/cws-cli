package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/null3000/cws-cli/internal/api"
)

// mockAuth implements auth.Authenticator for testing.
type mockAuth struct {
	token string
	err   error
}

func (m *mockAuth) AccessToken(ctx context.Context) (string, error) {
	return m.token, m.err
}

// newTestClient creates an api.Client pointed at the given httptest server.
func newTestClient(serverURL string) *api.Client {
	client := api.NewClient(&mockAuth{token: "test-token"}, "test-publisher")
	client.BaseURL = serverURL
	return client
}

// --- ParseAPIError tests ---

func TestParseAPIError_FieldViolation(t *testing.T) {
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

	got := api.ParseAPIError(body)
	want := "Invalid version number in manifest."
	if got != want {
		t.Errorf("ParseAPIError() = %q, want %q", got, want)
	}
}

func TestParseAPIError_MessageOnly(t *testing.T) {
	body := []byte(`{
		"error": {
			"code": 403,
			"message": "Forbidden",
			"status": "PERMISSION_DENIED"
		}
	}`)

	got := api.ParseAPIError(body)
	want := "Forbidden"
	if got != want {
		t.Errorf("ParseAPIError() = %q, want %q", got, want)
	}
}

func TestParseAPIError_InvalidJSON(t *testing.T) {
	got := api.ParseAPIError([]byte(`not json`))
	if got != "" {
		t.Errorf("ParseAPIError() = %q, want empty string", got)
	}
}

func TestParseAPIError_EmptyBody(t *testing.T) {
	got := api.ParseAPIError([]byte{})
	if got != "" {
		t.Errorf("ParseAPIError() = %q, want empty string", got)
	}
}

func TestParseAPIError_NullError(t *testing.T) {
	got := api.ParseAPIError([]byte(`{"error": null}`))
	if got != "" {
		t.Errorf("ParseAPIError() = %q, want empty string", got)
	}
}

// --- FetchStatus tests ---

func TestFetchStatus_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Authorization header 'Bearer test-token', got %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"name":   "publishers/test-publisher/items/ext123",
			"itemId": "ext123",
			"publishedItemRevisionStatus": map[string]any{
				"state": "PUBLISHED",
				"distributionChannels": []map[string]any{
					{"deployPercentage": 100, "crxVersion": "1.0.0"},
				},
			},
			"lastAsyncUploadState": "SUCCEEDED",
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	resp, _, err := client.FetchStatus(context.Background(), "ext123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ItemID != "ext123" {
		t.Errorf("ItemID = %q, want %q", resp.ItemID, "ext123")
	}
	if resp.PublishedItemRevisionStatus == nil {
		t.Fatal("PublishedItemRevisionStatus is nil")
	}
	if resp.PublishedItemRevisionStatus.State != "PUBLISHED" {
		t.Errorf("State = %q, want %q", resp.PublishedItemRevisionStatus.State, "PUBLISHED")
	}
	if len(resp.PublishedItemRevisionStatus.DistributionChannels) != 1 {
		t.Fatalf("expected 1 distribution channel, got %d", len(resp.PublishedItemRevisionStatus.DistributionChannels))
	}
	ch := resp.PublishedItemRevisionStatus.DistributionChannels[0]
	if ch.CrxVersion != "1.0.0" {
		t.Errorf("CrxVersion = %q, want %q", ch.CrxVersion, "1.0.0")
	}
	if ch.DeployPercentage != 100 {
		t.Errorf("DeployPercentage = %d, want 100", ch.DeployPercentage)
	}
	if resp.LastAsyncUploadState != "SUCCEEDED" {
		t.Errorf("LastAsyncUploadState = %q, want %q", resp.LastAsyncUploadState, "SUCCEEDED")
	}
}

func TestFetchStatus_404(t *testing.T) {
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
	if got := err.Error(); got != "status check failed (HTTP 404): extension not found. Verify the extension ID: badext" {
		t.Errorf("error = %q, want status check failed with extension not found message", got)
	}
}

func TestFetchStatus_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    400,
				"message": "Bad request",
				"status":  "INVALID_ARGUMENT",
			},
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, _, err := client.FetchStatus(context.Background(), "ext123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "status check failed (HTTP 400): Bad request" {
		t.Errorf("error = %q, want 'status check failed (HTTP 400): Bad request'", got)
	}
}

// --- Upload tests ---

func TestUpload_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != "zipdata" {
			t.Errorf("request body = %q, want %q", string(body), "zipdata")
		}
		if r.Header.Get("Content-Type") != "application/zip" {
			t.Errorf("Content-Type = %q, want application/zip", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"id":          "ext123",
			"uploadState": "SUCCESS",
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.Upload(context.Background(), "ext123", []byte("zipdata"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UploadState != "SUCCESS" {
		t.Errorf("UploadState = %q, want %q", resp.UploadState, "SUCCESS")
	}
}

func TestUpload_400_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    400,
				"message": "Invalid package",
				"details": []map[string]any{
					{
						"@type": "type.googleapis.com/google.rpc.BadRequest",
						"fieldViolations": []map[string]any{
							{"field": "media", "description": "Version too low"},
						},
					},
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
	if got := err.Error(); got != "upload failed (HTTP 400): Version too low" {
		t.Errorf("error = %q, want 'upload failed (HTTP 400): Version too low'", got)
	}
}

func TestUpload_AuthError(t *testing.T) {
	client := api.NewClient(&mockAuth{err: fmt.Errorf("auth failed")}, "test-publisher")
	client.BaseURL = "http://localhost"

	_, err := client.Upload(context.Background(), "ext123", []byte("zipdata"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "auth failed" {
		t.Errorf("error = %q, want %q", got, "auth failed")
	}
}

// --- Publish tests ---

func TestPublish_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"status": []string{"OK"},
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.Publish(context.Background(), "ext123", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Status) != 1 || resp.Status[0] != "OK" {
		t.Errorf("Status = %v, want [OK]", resp.Status)
	}
}

func TestPublish_Staged(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req api.PublishRequest
		json.Unmarshal(body, &req)
		if req.PublishType != "STAGED_PUBLISH" {
			t.Errorf("PublishType = %q, want %q", req.PublishType, "STAGED_PUBLISH")
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"status": []string{"OK"},
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.Publish(context.Background(), "ext123", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPublish_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    403,
				"message": "Permission denied",
			},
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.Publish(context.Background(), "ext123", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "publish failed (HTTP 403): Permission denied" {
		t.Errorf("error = %q, want 'publish failed (HTTP 403): Permission denied'", got)
	}
}

// --- SetDeployPercentage tests ---

func TestSetDeployPercentage_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req api.DeployPercentageRequest
		json.Unmarshal(body, &req)
		if req.DeployPercentage != 50 {
			t.Errorf("DeployPercentage = %d, want 50", req.DeployPercentage)
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"deployPercentage": 50,
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.SetDeployPercentage(context.Background(), "ext123", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.DeployPercentage != 50 {
		t.Errorf("DeployPercentage = %d, want 50", resp.DeployPercentage)
	}
}

func TestSetDeployPercentage_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    400,
				"message": "Extension does not meet requirements",
			},
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.SetDeployPercentage(context.Background(), "ext123", 50)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "rollout failed (HTTP 400): Extension does not meet requirements" {
		t.Errorf("error = %q, want 'rollout failed (HTTP 400): Extension does not meet requirements'", got)
	}
}

// --- CancelSubmission tests ---

func TestCancelSubmission_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"status": "OK",
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.CancelSubmission(context.Background(), "ext123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "OK" {
		t.Errorf("Status = %q, want %q", resp.Status, "OK")
	}
}

func TestCancelSubmission_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    400,
				"message": "No pending submission",
			},
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.CancelSubmission(context.Background(), "ext123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "cancel failed (HTTP 400): No pending submission" {
		t.Errorf("error = %q, want 'cancel failed (HTTP 400): No pending submission'", got)
	}
}
