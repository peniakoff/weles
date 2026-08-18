package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/peniakoff/weles/internal/config"
	"github.com/peniakoff/weles/internal/feedback"
	"github.com/peniakoff/weles/internal/httpapi"
)

type fakeVerifier struct {
	err error
}

func (f fakeVerifier) Verify(context.Context, string, string) error { return f.err }

type recordingPublisher struct {
	mu      sync.Mutex
	reports []*feedback.Report
	err     error
}

func (p *recordingPublisher) Publish(_ context.Context, report *feedback.Report) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return p.err
	}
	p.reports = append(p.reports, report)
	return nil
}

func testServerWith(t *testing.T, pub *recordingPublisher, verifier fakeVerifier) http.Handler {
	t.Helper()
	reg, err := config.ParseAppsYAML([]byte(`
apps:
  - id: example-app
    origins: ["https://app.example.com"]
    hosts: ["app.example.com"]
`))
	if err != nil {
		t.Fatal(err)
	}
	if pub == nil {
		pub = &recordingPublisher{}
	}
	s := &httpapi.Server{
		Registry:  reg,
		Verifier:  verifier,
		Publisher: pub,
		MaxBody:   8 * 1024,
	}
	return s.NewMux()
}

func testServer(t *testing.T, pub *recordingPublisher) http.Handler {
	return testServerWith(t, pub, fakeVerifier{})
}

func validBody() map[string]any {
	return map[string]any{
		"appId":    "example-app",
		"category": "bug",
		"message":  "The calculated value is wrong after switching units.",
		"email":    "user@example.com",
		"context": map[string]any{
			"locale":  "en",
			"pageUrl": "https://app.example.com/en/tools/roof",
			"metadata": map[string]string{
				"toolId": "roof",
			},
		},
		"turnstileToken": "tok",
		"submittedAt":    "2026-08-17T18:00:00.000Z",
		"unexpected":     "ignored",
	}
}

func postJSON(t *testing.T, h http.Handler, origin string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/feedback", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestFeedback_Accepted(t *testing.T) {
	pub := &recordingPublisher{}
	h := testServer(t, pub)
	rec := postJSON(t, h, "https://app.example.com", validBody())

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("CORS origin: %q", got)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing security header")
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["reportId"] == "" {
		t.Fatal("missing reportId")
	}
	if len(pub.reports) != 1 {
		t.Fatalf("published=%d", len(pub.reports))
	}
}

func TestFeedback_Table(t *testing.T) {
	tests := []struct {
		name     string
		origin   string
		body     func() any
		verifier fakeVerifier
		pubErr   error
		wantCode int
		wantErr  string
	}{
		{
			name:   "cors reject unknown origin on POST",
			origin: "https://evil.example.net",
			body: func() any {
				return validBody()
			},
			wantCode: http.StatusForbidden,
			wantErr:  "forbidden",
		},
		{
			name:   "host mismatch",
			origin: "https://app.example.com",
			body: func() any {
				b := validBody()
				b["context"].(map[string]any)["pageUrl"] = "https://evil.example.net/x"
				return b
			},
			wantCode: http.StatusForbidden,
			wantErr:  "forbidden",
		},
		{
			name:   "validation bad category",
			origin: "https://app.example.com",
			body: func() any {
				b := validBody()
				b["category"] = "spam"
				return b
			},
			wantCode: http.StatusBadRequest,
			wantErr:  "bad request",
		},
		{
			name:   "email injection",
			origin: "https://app.example.com",
			body: func() any {
				b := validBody()
				b["email"] = "user@example.com\r\nBcc: evil@example.com"
				return b
			},
			wantCode: http.StatusBadRequest,
			wantErr:  "bad request",
		},
		{
			name:   "invalid json",
			origin: "https://app.example.com",
			body: func() any {
				return nil // special-cased below
			},
			wantCode: http.StatusBadRequest,
			wantErr:  "bad request",
		},
		{
			name:   "wrong content-type",
			origin: "https://app.example.com",
			body: func() any {
				return validBody()
			},
			wantCode: http.StatusBadRequest,
			wantErr:  "bad request",
		},
		{
			name:     "turnstile failure",
			origin:   "https://app.example.com",
			body:     func() any { return validBody() },
			verifier: fakeVerifier{err: errors.New("nope")},
			wantCode: http.StatusForbidden,
			wantErr:  "forbidden",
		},
		{
			name:     "publish failure",
			origin:   "https://app.example.com",
			body:     func() any { return validBody() },
			pubErr:   io.ErrUnexpectedEOF,
			wantCode: http.StatusServiceUnavailable,
			wantErr:  "unavailable",
		},
		{
			name:   "body too large",
			origin: "https://app.example.com",
			body: func() any {
				b := validBody()
				b["message"] = strings.Repeat("x", 9000)
				return b
			},
			wantCode: http.StatusBadRequest,
			wantErr:  "bad request",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pub := &recordingPublisher{err: tc.pubErr}
			h := testServerWith(t, pub, tc.verifier)

			var rec *httptest.ResponseRecorder
			switch tc.name {
			case "invalid json":
				req := httptest.NewRequest(http.MethodPost, "/v1/feedback", bytes.NewReader([]byte("{")))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Origin", tc.origin)
				rec = httptest.NewRecorder()
				h.ServeHTTP(rec, req)
			case "wrong content-type":
				raw, _ := json.Marshal(validBody())
				req := httptest.NewRequest(http.MethodPost, "/v1/feedback", bytes.NewReader(raw))
				req.Header.Set("Content-Type", "text/plain")
				req.Header.Set("Origin", tc.origin)
				rec = httptest.NewRecorder()
				h.ServeHTTP(rec, req)
			default:
				rec = postJSON(t, h, tc.origin, tc.body())
			}

			if rec.Code != tc.wantCode {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			var resp map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v body=%s", err, rec.Body.String())
			}
			if resp["error"] != tc.wantErr {
				t.Fatalf("error=%q", resp["error"])
			}
			// Generic errors must not echo user input.
			if strings.Contains(rec.Body.String(), "user@example.com") ||
				strings.Contains(rec.Body.String(), "Bcc:") ||
				strings.Contains(rec.Body.String(), "evil.example.net") {
				t.Fatalf("response leaked input: %s", rec.Body.String())
			}
		})
	}
}

func TestFeedback_CORS_Preflight(t *testing.T) {
	h := testServer(t, nil)

	t.Run("allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/v1/feedback", nil)
		req.Header.Set("Origin", "https://app.example.com")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status=%d", rec.Code)
		}
		if rec.Header().Get("Access-Control-Allow-Methods") == "" {
			t.Fatal("missing allow methods")
		}
		if rec.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
			t.Fatal("missing allow origin")
		}
	})

	t.Run("rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/v1/feedback", nil)
		req.Header.Set("Origin", "https://evil.example.net")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHealthz(t *testing.T) {
	h := testServer(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHealthz_StagePrefixWithoutStrip_NotFound(t *testing.T) {
	h := testServer(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/prod/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStripStagePrefix_APIGatewayNamedStage(t *testing.T) {
	pub := &recordingPublisher{}
	h := httpapi.StripStagePrefix("prod", testServer(t, pub))

	t.Run("GET /prod/healthz", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/prod/healthz", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("missing security header")
		}
		var resp map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp["status"] != "ok" {
			t.Fatalf("status=%q", resp["status"])
		}
	})

	t.Run("POST /prod/v1/feedback", func(t *testing.T) {
		raw, err := json.Marshal(validBody())
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/prod/v1/feedback", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://app.example.com")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("OPTIONS /prod/v1/feedback", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/prod/v1/feedback", nil)
		req.Header.Set("Origin", "https://app.example.com")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("unprefixed /healthz still works", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestStripStagePrefix_Table(t *testing.T) {
	tests := []struct {
		name     string
		stage    string
		path     string
		wantCode int
	}{
		{name: "empty stage leaves prefix unmatched", stage: "", path: "/prod/healthz", wantCode: http.StatusNotFound},
		{name: "default stage is a no-op", stage: "$default", path: "/healthz", wantCode: http.StatusOK},
		{name: "default stage does not strip literal $default", stage: "$default", path: "/$default/healthz", wantCode: http.StatusNotFound},
		{name: "trims slashes on stage", stage: "/prod/", path: "/prod/healthz", wantCode: http.StatusOK},
		{name: "does not strip a longer first segment", stage: "prod", path: "/production/healthz", wantCode: http.StatusNotFound},
		{name: "other stage name", stage: "staging", path: "/staging/healthz", wantCode: http.StatusOK},
		{name: "wrong stage left intact", stage: "staging", path: "/prod/healthz", wantCode: http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := httpapi.StripStagePrefix(tc.stage, testServer(t, nil))
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.wantCode {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
