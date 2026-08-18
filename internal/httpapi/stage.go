package httpapi

import (
	"net/http"
	"strings"
)

// StripStagePrefix removes an API Gateway HTTP API stage prefix from the
// request path before calling next.
//
// Named stages (see template.yaml StageName, default prod) make
// httpadapter.NewV2 pass "/{stage}/healthz" into Go 1.22 ServeMux, which
// matches paths exactly. Empty stage and "$default" are no-ops. Paths that
// do not start with the stage segment are left unchanged.
func StripStagePrefix(stage string, next http.Handler) http.Handler {
	stage = strings.Trim(stage, "/")
	if stage == "" || stage == "$default" {
		return next
	}
	prefix := "/" + stage
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if stripped, ok := stripExactPathPrefix(r.URL.Path, prefix); ok {
			r.URL.Path = stripped
			if r.URL.RawPath != "" {
				if raw, ok := stripExactPathPrefix(r.URL.RawPath, prefix); ok {
					r.URL.RawPath = raw
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func stripExactPathPrefix(path, prefix string) (string, bool) {
	if path == prefix {
		return "/", true
	}
	if strings.HasPrefix(path, prefix+"/") {
		out := path[len(prefix):]
		if out == "" {
			return "/", true
		}
		return out, true
	}
	return path, false
}
