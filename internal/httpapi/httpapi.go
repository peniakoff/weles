// Package httpapi exposes the feedback HTTP API (net/http compatible).
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/peniakoff/weles/internal/config"
	"github.com/peniakoff/weles/internal/feedback"
	"github.com/peniakoff/weles/internal/notify"
	"github.com/peniakoff/weles/internal/turnstile"
)

// Server wires dependencies for the feedback API.
type Server struct {
	Registry      config.AppRegistry
	Verifier      turnstile.Verifier
	Publisher     notify.Publisher
	MaxBody       int64
	Logger        *slog.Logger
	TrustProxyXFF bool
}

// NewMux returns the HTTP handler with routes and middleware.
func (s *Server) NewMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("POST /v1/feedback", s.handleFeedback)
	mux.HandleFunc("OPTIONS /v1/feedback", s.handleFeedbackOptions)
	return s.withSecurityHeaders(mux)
}

func (s *Server) log() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func (s *Server) maxBody() int64 {
	if s.MaxBody > 0 {
		return s.MaxBody
	}
	return feedback.MaxBodyBytes()
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleFeedbackOptions(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" || !s.anyAppAllowsOrigin(origin) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	setCORS(w, origin)
	w.Header().Set("Access-Control-Max-Age", "600")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleFeedback(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin != "" {
		if !s.anyAppAllowsOrigin(origin) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		setCORS(w, origin)
	}

	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(strings.ToLower(ct), "application/json") {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.maxBody())
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	var sub feedback.Submission
	// Unknown JSON fields are ignored (not persisted or logged).
	if err := json.Unmarshal(body, &sub); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	report, err := feedback.Validate(sub, s.Registry, origin)
	if err != nil {
		var verr *feedback.ValidationError
		var ferr *feedback.ForbiddenError
		switch {
		case errors.As(err, &ferr):
			writeError(w, http.StatusForbidden, "forbidden")
		case errors.As(err, &verr):
			writeError(w, http.StatusBadRequest, "bad request")
		default:
			s.log().Error("validation unexpected", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	remoteIP := clientIP(r, s.TrustProxyXFF)
	if err := s.Verifier.Verify(r.Context(), sub.TurnstileToken, remoteIP); err != nil {
		s.log().Info("turnstile failed", "reportId", report.ID, "appId", report.AppID)
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := s.Publisher.Publish(r.Context(), report); err != nil {
		s.log().Error("publish failed", "reportId", report.ID, "appId", report.AppID, "err", err)
		writeError(w, http.StatusServiceUnavailable, "unavailable")
		return
	}

	s.log().Info("feedback accepted", "reportId", report.ID, "appId", report.AppID, "category", report.Category)
	writeJSON(w, http.StatusAccepted, map[string]string{"reportId": report.ID})
}

func (s *Server) anyAppAllowsOrigin(origin string) bool {
	return s.Registry.OriginAllowed(origin)
}

func (s *Server) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func setCORS(w http.ResponseWriter, origin string) {
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Vary", "Origin")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// clientIP returns the peer address for Turnstile remoteip.
// By default it prefers RemoteAddr (set from API Gateway requestContext by the
// Lambda adapter) and ignores client-controlled X-Forwarded-For. Set
// TrustProxyXFF when a trusted reverse proxy is the only ingress.
func clientIP(r *http.Request, trustProxyXFF bool) string {
	if trustProxyXFF {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := strings.TrimSpace(parts[0]); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
