package httpapi

import "testing"

func TestStripExactPathPrefix(t *testing.T) {
	tests := []struct {
		path, prefix, want string
		ok                 bool
	}{
		{path: "/prod/healthz", prefix: "/prod", want: "/healthz", ok: true},
		{path: "/prod/v1/feedback", prefix: "/prod", want: "/v1/feedback", ok: true},
		{path: "/prod", prefix: "/prod", want: "/", ok: true},
		{path: "/prod/", prefix: "/prod", want: "/", ok: true},
		{path: "/healthz", prefix: "/prod", want: "/healthz", ok: false},
		{path: "/production/healthz", prefix: "/prod", want: "/production/healthz", ok: false},
		{path: "/staging/healthz", prefix: "/prod", want: "/staging/healthz", ok: false},
	}
	for _, tc := range tests {
		got, ok := stripExactPathPrefix(tc.path, tc.prefix)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("stripExactPathPrefix(%q, %q)=(%q, %v) want (%q, %v)",
				tc.path, tc.prefix, got, ok, tc.want, tc.ok)
		}
	}
}
