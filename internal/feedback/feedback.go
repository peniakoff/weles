// Package feedback validates and sanitizes inbound feedback submissions.
package feedback

import (
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/oklog/ulid/v2"

	"github.com/peniakoff/weles/internal/config"
)

const (
	minMessageLen = 10
	maxMessageLen = 4000
	maxEmailLen   = 254
	maxURLLen     = 2048
	maxLocaleLen  = 16
	maxMetaKeys   = 10
	maxMetaKeyLen = 64
	maxMetaValLen = 256
	maxAppIDLen   = 64
	maxBodyHint   = 8 * 1024
)

// Category is the feedback type selected by the user.
type Category string

const (
	CategoryBug        Category = "bug"
	CategorySuggestion Category = "suggestion"
	CategoryOther      Category = "other"
)

// Submission is the JSON body accepted by POST /v1/feedback.
type Submission struct {
	AppID          string            `json:"appId"`
	Category       Category          `json:"category"`
	Message        string            `json:"message"`
	Email          string            `json:"email,omitempty"`
	Context        Context           `json:"context"`
	TurnstileToken string            `json:"turnstileToken"`
	SubmittedAt    string            `json:"submittedAt,omitempty"`
}

// Context carries page metadata; Metadata is opaque client labels.
type Context struct {
	Locale   string            `json:"locale"`
	PageURL  string            `json:"pageUrl"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Report is a validated submission ready for notification.
type Report struct {
	ID             string
	AppID          string
	Category       Category
	Message        string
	Email          string
	Locale         string
	PageURL        string
	Metadata       map[string]string
	ClientSubmittedAt string
	ReceivedAt     time.Time
}

// ValidationError is a client-facing validation failure (maps to HTTP 400).
type ValidationError struct {
	Reason string
}

func (e *ValidationError) Error() string {
	return e.Reason
}

// ForbiddenError maps to HTTP 403 (unknown app, origin/host mismatch).
type ForbiddenError struct {
	Reason string
}

func (e *ForbiddenError) Error() string {
	return e.Reason
}

// Validate sanitizes and validates a submission against the app registry.
// origin may be empty for non-browser callers; when non-empty it must be allowlisted.
func Validate(sub Submission, registry config.AppRegistry, origin string) (*Report, error) {
	appID := strings.TrimSpace(sub.AppID)
	if appID == "" || utf8.RuneCountInString(appID) > maxAppIDLen || !isSafeToken(appID) {
		return nil, &ValidationError{Reason: "invalid appId"}
	}
	app, ok := registry.Get(appID)
	if !ok {
		return nil, &ForbiddenError{Reason: "unknown app"}
	}
	_ = app

	if origin != "" && !registry.AllowsOrigin(appID, origin) {
		return nil, &ForbiddenError{Reason: "origin not allowed"}
	}

	switch Category(strings.TrimSpace(string(sub.Category))) {
	case CategoryBug, CategorySuggestion, CategoryOther:
		// ok
	default:
		return nil, &ValidationError{Reason: "invalid category"}
	}
	category := Category(strings.TrimSpace(string(sub.Category)))

	message := sanitizeMultiline(sub.Message)
	msgLen := utf8.RuneCountInString(message)
	if msgLen < minMessageLen || msgLen > maxMessageLen {
		return nil, &ValidationError{Reason: "invalid message length"}
	}

	email := strings.TrimSpace(sub.Email)
	if email != "" {
		if strings.ContainsAny(email, "\r\n") {
			return nil, &ValidationError{Reason: "invalid email"}
		}
		if utf8.RuneCountInString(email) > maxEmailLen {
			return nil, &ValidationError{Reason: "invalid email"}
		}
		addr, err := mail.ParseAddress(email)
		if err != nil || addr.Address != email {
			return nil, &ValidationError{Reason: "invalid email"}
		}
		email = addr.Address
	}

	locale := sanitizeSingleLine(sub.Context.Locale)
	if locale == "" || utf8.RuneCountInString(locale) > maxLocaleLen || !isSafeToken(locale) {
		return nil, &ValidationError{Reason: "invalid locale"}
	}

	pageURL, host, err := validatePageURL(sub.Context.PageURL)
	if err != nil {
		return nil, err
	}
	if !registry.AllowsHost(appID, host) {
		return nil, &ForbiddenError{Reason: "page host not allowed"}
	}

	meta, err := sanitizeMetadata(sub.Context.Metadata)
	if err != nil {
		return nil, err
	}

	token := strings.TrimSpace(sub.TurnstileToken)
	if token == "" {
		return nil, &ValidationError{Reason: "missing turnstileToken"}
	}

	clientSubmitted := strings.TrimSpace(sub.SubmittedAt)
	if clientSubmitted != "" {
		if _, err := time.Parse(time.RFC3339Nano, clientSubmitted); err != nil {
			if _, err2 := time.Parse(time.RFC3339, clientSubmitted); err2 != nil {
				return nil, &ValidationError{Reason: "invalid submittedAt"}
			}
		}
	}

	id := ulid.Make().String()
	return &Report{
		ID:                id,
		AppID:             appID,
		Category:          category,
		Message:           message,
		Email:             email,
		Locale:            locale,
		PageURL:           pageURL,
		Metadata:          meta,
		ClientSubmittedAt: clientSubmitted,
		ReceivedAt:        time.Now().UTC(),
	}, nil
}

func validatePageURL(raw string) (normalized string, host string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxURLLen {
		return "", "", &ValidationError{Reason: "invalid pageUrl"}
	}
	if strings.ContainsAny(raw, "\r\n\x00") {
		return "", "", &ValidationError{Reason: "invalid pageUrl"}
	}
	u, parseErr := url.Parse(raw)
	if parseErr != nil {
		return "", "", &ValidationError{Reason: "invalid pageUrl"}
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		// Production clients should use https; allow http only for localhost in allowlist.
		return "", "", &ValidationError{Reason: "invalid pageUrl scheme"}
	}
	if u.User != nil {
		return "", "", &ValidationError{Reason: "invalid pageUrl"}
	}
	host = strings.ToLower(u.Hostname())
	if host == "" {
		return "", "", &ValidationError{Reason: "invalid pageUrl"}
	}
	// Reject non-https except localhost (dev).
	if u.Scheme != "https" && host != "localhost" && host != "127.0.0.1" {
		return "", "", &ValidationError{Reason: "invalid pageUrl scheme"}
	}
	u.Fragment = ""
	return u.String(), host, nil
}

func sanitizeMetadata(in map[string]string) (map[string]string, error) {
	if len(in) == 0 {
		return nil, nil
	}
	if len(in) > maxMetaKeys {
		return nil, &ValidationError{Reason: "too many metadata keys"}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		k = sanitizeSingleLine(k)
		v = sanitizeSingleLine(v)
		if k == "" || utf8.RuneCountInString(k) > maxMetaKeyLen {
			return nil, &ValidationError{Reason: "invalid metadata key"}
		}
		if utf8.RuneCountInString(v) > maxMetaValLen {
			return nil, &ValidationError{Reason: "invalid metadata value"}
		}
		out[k] = v
	}
	return out, nil
}

func sanitizeMultiline(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\t' || (r >= 32 && r != 127) || unicode.IsLetter(r) || unicode.IsNumber(r) {
			if unicode.IsControl(r) && r != '\n' && r != '\t' {
				continue
			}
			b.WriteRune(r)
			continue
		}
		if !unicode.IsControl(r) {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func sanitizeSingleLine(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func isSafeToken(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

// FormatPlainEmail builds a plaintext notification body (not HTML).
func FormatPlainEmail(r *Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Weles feedback report\n")
	fmt.Fprintf(&b, "=====================\n\n")
	fmt.Fprintf(&b, "Report ID:  %s\n", r.ID)
	fmt.Fprintf(&b, "App:        %s\n", r.AppID)
	fmt.Fprintf(&b, "Category:   %s\n", r.Category)
	fmt.Fprintf(&b, "Locale:     %s\n", r.Locale)
	fmt.Fprintf(&b, "Page URL:   %s\n", r.PageURL)
	fmt.Fprintf(&b, "Received:   %s\n", r.ReceivedAt.Format(time.RFC3339))
	if r.ClientSubmittedAt != "" {
		fmt.Fprintf(&b, "Client at:  %s\n", r.ClientSubmittedAt)
	}
	if r.Email != "" {
		fmt.Fprintf(&b, "Contact:    %s\n", r.Email)
	} else {
		fmt.Fprintf(&b, "Contact:    (not provided)\n")
	}
	if len(r.Metadata) > 0 {
		fmt.Fprintf(&b, "\nMetadata:\n")
		for k, v := range r.Metadata {
			fmt.Fprintf(&b, "  %s: %s\n", k, v)
		}
	}
	fmt.Fprintf(&b, "\nMessage:\n%s\n", r.Message)
	return b.String()
}

// MaxBodyBytes is the recommended HTTP body size limit.
func MaxBodyBytes() int64 {
	return maxBodyHint
}
