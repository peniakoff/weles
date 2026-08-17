package turnstile_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/peniakoff/weles/internal/turnstile"
)

func TestSkip_Verify(t *testing.T) {
	var s turnstile.Skip
	if err := s.Verify(context.Background(), "tok", ""); err != nil {
		t.Fatalf("expected ok: %v", err)
	}
	if err := s.Verify(context.Background(), "  ", ""); err == nil {
		t.Fatal("expected empty token error")
	}
}

func TestNewCloudflare_RequiresSecret(t *testing.T) {
	if _, err := turnstile.NewCloudflare(""); err == nil {
		t.Fatal("expected error for empty secret")
	}
	if _, err := turnstile.NewCloudflare("  "); err == nil {
		t.Fatal("expected error for blank secret")
	}
}

func TestCloudflare_Verify_Table(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		token      string
		remoteIP   string
		wantErr    bool
		wantRemote bool
	}{
		{
			name:    "success",
			status:  http.StatusOK,
			body:    `{"success":true}`,
			token:   "good",
			wantErr: false,
		},
		{
			name:    "rejected",
			status:  http.StatusOK,
			body:    `{"success":false,"error-codes":["invalid-input-response"]}`,
			token:   "bad",
			wantErr: true,
		},
		{
			name:    "http error",
			status:  http.StatusBadGateway,
			body:    `oops`,
			token:   "tok",
			wantErr: true,
		},
		{
			name:    "invalid json",
			status:  http.StatusOK,
			body:    `{`,
			token:   "tok",
			wantErr: true,
		},
		{
			name:    "empty token",
			status:  http.StatusOK,
			body:    `{"success":true}`,
			token:   "",
			wantErr: true,
		},
		{
			name:       "forwards remoteip",
			status:     http.StatusOK,
			body:       `{"success":true}`,
			token:      "tok",
			remoteIP:   "203.0.113.10",
			wantErr:    false,
			wantRemote: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var sawRemote bool
			var hit bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hit = true
				if err := r.ParseForm(); err != nil {
					t.Errorf("parse form: %v", err)
				}
				if r.Form.Get("secret") != "test-secret" {
					t.Errorf("secret=%q", r.Form.Get("secret"))
				}
				if r.Form.Get("remoteip") == "203.0.113.10" {
					sawRemote = true
				}
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprint(w, tc.body)
			}))
			t.Cleanup(srv.Close)

			cf, err := turnstile.NewCloudflare("test-secret")
			if err != nil {
				t.Fatal(err)
			}
			cf.VerifyURL = srv.URL
			cf.Client = srv.Client()

			err = cf.Verify(context.Background(), tc.token, tc.remoteIP)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.token != "" && !tc.wantErr && !hit {
				t.Fatal("expected HTTP call")
			}
			if tc.wantRemote && !sawRemote {
				t.Fatal("expected remoteip to be forwarded")
			}
		})
	}
}
