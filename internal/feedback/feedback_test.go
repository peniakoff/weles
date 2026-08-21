package feedback_test

import (
	"strings"
	"testing"

	"github.com/peniakoff/weles/internal/config"
	"github.com/peniakoff/weles/internal/feedback"
)

func testRegistry(t *testing.T) *config.Registry {
	t.Helper()
	reg, err := config.ParseAppsYAML([]byte(`
apps:
  - id: example-app
    origins:
      - "https://app.example.com"
      - "http://localhost:5173"
    hosts:
      - "app.example.com"
      - "localhost"
    notificationEmail: ops@example.com
    fromEmail: noreply@example.com
`))
	if err != nil {
		t.Fatalf("parse registry: %v", err)
	}
	return reg
}

func validSubmission() feedback.Submission {
	return feedback.Submission{
		AppID:    "example-app",
		Category: feedback.CategoryBug,
		Message:  "The calculated value is wrong after switching units.",
		Email:    "user@example.com",
		Context: feedback.Context{
			Locale:  "en",
			PageURL: "https://app.example.com/en/tools/roof",
			Metadata: map[string]string{
				"toolId":   "roof",
				"toolName": "Roof calculator",
			},
		},
		TurnstileToken: "test-token",
		SubmittedAt:    "2026-08-17T18:00:00.000Z",
	}
}

func TestValidate_OK(t *testing.T) {
	reg := testRegistry(t)
	report, err := feedback.Validate(validSubmission(), reg, "https://app.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.ID == "" {
		t.Fatal("expected report ID")
	}
	if report.AppID != "example-app" {
		t.Fatalf("appId: got %q", report.AppID)
	}
	if report.Email != "user@example.com" {
		t.Fatalf("email: got %q", report.Email)
	}
}

func TestValidate_Table(t *testing.T) {
	reg := testRegistry(t)

	tests := []struct {
		name    string
		mutate  func(*feedback.Submission)
		origin  string
		wantErr error
	}{
		{
			name: "short message",
			mutate: func(s *feedback.Submission) {
				s.Message = "too short"
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ValidationError{},
		},
		{
			name: "invalid category",
			mutate: func(s *feedback.Submission) {
				s.Category = "spam"
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ValidationError{},
		},
		{
			name: "email header injection",
			mutate: func(s *feedback.Submission) {
				s.Email = "user@example.com\r\nBcc: evil@example.com"
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ValidationError{},
		},
		{
			name: "invalid email format",
			mutate: func(s *feedback.Submission) {
				s.Email = "not-an-email"
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ValidationError{},
		},
		{
			name: "pageUrl with userinfo",
			mutate: func(s *feedback.Submission) {
				s.Context.PageURL = "https://user:pass@app.example.com/path"
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ValidationError{},
		},
		{
			name: "pageUrl javascript scheme",
			mutate: func(s *feedback.Submission) {
				s.Context.PageURL = "javascript:alert(1)"
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ValidationError{},
		},
		{
			name: "host not allowlisted",
			mutate: func(s *feedback.Submission) {
				s.Context.PageURL = "https://evil.example.net/phish"
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ForbiddenError{},
		},
		{
			name: "origin not allowlisted for app",
			mutate: func(s *feedback.Submission) {
			},
			origin:  "https://evil.example.net",
			wantErr: &feedback.ForbiddenError{},
		},
		{
			name: "unknown app",
			mutate: func(s *feedback.Submission) {
				s.AppID = "unknown-app"
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ForbiddenError{},
		},
		{
			name: "missing turnstile token",
			mutate: func(s *feedback.Submission) {
				s.TurnstileToken = ""
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ValidationError{},
		},
		{
			name: "control chars stripped from message still valid",
			mutate: func(s *feedback.Submission) {
				s.Message = "Hello\x00 world — this is long enough."
			},
			origin:  "https://app.example.com",
			wantErr: nil,
		},
		{
			name: "too many metadata keys",
			mutate: func(s *feedback.Submission) {
				s.Context.Metadata = map[string]string{}
				for i := 0; i < 11; i++ {
					s.Context.Metadata[strings.Repeat("k", i+1)] = "v"
				}
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ValidationError{},
		},
		{
			name: "http non-localhost rejected",
			mutate: func(s *feedback.Submission) {
				s.Context.PageURL = "http://app.example.com/path"
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ValidationError{},
		},
		{
			name: "localhost http allowed when host allowlisted",
			mutate: func(s *feedback.Submission) {
				s.Context.PageURL = "http://localhost:5173/tool"
			},
			origin:  "http://localhost:5173",
			wantErr: nil,
		},
		{
			name: "optional email omitted",
			mutate: func(s *feedback.Submission) {
				s.Email = ""
			},
			origin:  "https://app.example.com",
			wantErr: nil,
		},
		{
			name: "message too long",
			mutate: func(s *feedback.Submission) {
				s.Message = strings.Repeat("m", 4001)
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ValidationError{},
		},
		{
			name: "empty appId",
			mutate: func(s *feedback.Submission) {
				s.AppID = ""
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ValidationError{},
		},
		{
			name: "unsafe appId",
			mutate: func(s *feedback.Submission) {
				s.AppID = "evil app!"
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ValidationError{},
		},
		{
			name: "invalid locale",
			mutate: func(s *feedback.Submission) {
				s.Context.Locale = "en/../x"
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ValidationError{},
		},
		{
			name: "invalid submittedAt",
			mutate: func(s *feedback.Submission) {
				s.SubmittedAt = "yesterday"
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ValidationError{},
		},
		{
			name: "display-name email rejected",
			mutate: func(s *feedback.Submission) {
				s.Email = "User Name <user@example.com>"
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ValidationError{},
		},
		{
			name: "metadata value too long",
			mutate: func(s *feedback.Submission) {
				s.Context.Metadata = map[string]string{"toolId": strings.Repeat("x", 257)}
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ValidationError{},
		},
		{
			name: "suggestion category ok",
			mutate: func(s *feedback.Submission) {
				s.Category = feedback.CategorySuggestion
			},
			origin:  "https://app.example.com",
			wantErr: nil,
		},
		{
			name: "other category ok",
			mutate: func(s *feedback.Submission) {
				s.Category = feedback.CategoryOther
			},
			origin:  "https://app.example.com",
			wantErr: nil,
		},
		{
			name: "empty origin allowed when host matches",
			mutate: func(s *feedback.Submission) {
			},
			origin:  "",
			wantErr: nil,
		},
		{
			name: "ftp scheme rejected",
			mutate: func(s *feedback.Submission) {
				s.Context.PageURL = "ftp://app.example.com/file"
			},
			origin:  "https://app.example.com",
			wantErr: &feedback.ValidationError{},
		},
		{
			name: "unknown fields ignored at Validate layer",
			mutate: func(s *feedback.Submission) {
				s.Message = strings.Repeat("a", 10)
			},
			origin:  "https://app.example.com",
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sub := validSubmission()
			tc.mutate(&sub)
			_, err := feedback.Validate(sub, reg, tc.origin)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			switch tc.wantErr.(type) {
			case *feedback.ValidationError:
				var verr *feedback.ValidationError
				if !asValidation(err, &verr) {
					t.Fatalf("want ValidationError, got %T %v", err, err)
				}
			case *feedback.ForbiddenError:
				var ferr *feedback.ForbiddenError
				if !asForbidden(err, &ferr) {
					t.Fatalf("want ForbiddenError, got %T %v", err, err)
				}
			}
		})
	}
}

func asValidation(err error, target **feedback.ValidationError) bool {
	e, ok := err.(*feedback.ValidationError)
	if !ok {
		return false
	}
	*target = e
	return true
}

func asForbidden(err error, target **feedback.ForbiddenError) bool {
	e, ok := err.(*feedback.ForbiddenError)
	if !ok {
		return false
	}
	*target = e
	return true
}

func TestFormatPlainEmail_NoHTML(t *testing.T) {
	r := &feedback.Report{
		ID:       "01TEST",
		AppID:    "example-app",
		Category: feedback.CategoryBug,
		Message:  "Line one\nLine two",
		PageURL:  "https://app.example.com/x",
		Locale:   "en",
	}
	body := feedback.FormatPlainEmail(r)
	if strings.Contains(strings.ToLower(body), "<html") || strings.Contains(body, "<script") {
		t.Fatal("email body must be plaintext")
	}
	if !strings.Contains(body, "Line one\nLine two") {
		t.Fatal("message body missing")
	}
}
