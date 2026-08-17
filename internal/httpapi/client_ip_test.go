package httpapi

import (
	"net/http"
	"testing"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name          string
		remoteAddr    string
		xff           string
		trustProxyXFF bool
		want          string
	}{
		{
			name:       "prefers RemoteAddr over XFF by default",
			remoteAddr: "203.0.113.50:12345",
			xff:        "198.51.100.1",
			want:       "203.0.113.50",
		},
		{
			name:          "uses XFF when trust enabled",
			remoteAddr:    "203.0.113.50:12345",
			xff:           "198.51.100.1, 203.0.113.50",
			trustProxyXFF: true,
			want:          "198.51.100.1",
		},
		{
			name:       "remote without port",
			remoteAddr: "203.0.113.50",
			want:       "203.0.113.50",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: tc.remoteAddr,
				Header:     http.Header{},
			}
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			got := clientIP(req, tc.trustProxyXFF)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
