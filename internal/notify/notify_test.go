package notify_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"

	"github.com/peniakoff/weles/internal/config"
	"github.com/peniakoff/weles/internal/feedback"
	"github.com/peniakoff/weles/internal/notify"
)

func testRegistry(t *testing.T, yaml string) *config.Registry {
	t.Helper()
	reg, err := config.ParseAppsYAML([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestNewSES_RequiresRegistry(t *testing.T) {
	if _, err := notify.NewSES(context.Background(), "eu-central-1", nil); err == nil {
		t.Fatal("expected error for nil registry")
	}
}

type fakeSES struct {
	last *sesv2.SendEmailInput
	err  error
}

func (f *fakeSES) SendEmail(_ context.Context, params *sesv2.SendEmailInput, _ ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	f.last = params
	if f.err != nil {
		return nil, f.err
	}
	return &sesv2.SendEmailOutput{}, nil
}

func TestSES_Publish(t *testing.T) {
	reg := testRegistry(t, `
apps:
  - id: example-app
    origins: ["https://app.example.com"]
    hosts: ["app.example.com"]
    notificationEmail: ops@example.com
    fromEmail: noreply@example.com
`)
	fake := &fakeSES{}
	pub := &notify.SES{Client: fake, Registry: reg}
	report := &feedback.Report{
		ID:       "01TESTREPORTID",
		AppID:    "example-app",
		Category: feedback.CategoryBug,
		Message:  "Something broke in the calculator flow.",
		PageURL:  "https://app.example.com/x",
		Locale:   "en",
		Email:    "user@example.com",
	}
	if err := pub.Publish(context.Background(), report); err != nil {
		t.Fatal(err)
	}
	if fake.last == nil {
		t.Fatal("expected SendEmail call")
	}
	if fake.last.FromEmailAddress == nil || *fake.last.FromEmailAddress != "noreply@example.com" {
		t.Fatalf("from=%v", fake.last.FromEmailAddress)
	}
	if fake.last.Destination == nil || len(fake.last.Destination.ToAddresses) != 1 || fake.last.Destination.ToAddresses[0] != "ops@example.com" {
		t.Fatalf("destination=%v", fake.last.Destination)
	}
	if len(fake.last.ReplyToAddresses) != 1 || fake.last.ReplyToAddresses[0] != report.Email {
		t.Fatalf("replyTo=%v", fake.last.ReplyToAddresses)
	}
	if fake.last.Content == nil || fake.last.Content.Simple == nil {
		t.Fatal("expected simple content")
	}
	simple := fake.last.Content.Simple
	if simple.Subject == nil || simple.Subject.Data == nil || !strings.Contains(*simple.Subject.Data, "example-app") {
		t.Fatalf("subject=%v", simple.Subject)
	}
	if strings.Contains(*simple.Subject.Data, "·") {
		t.Fatal("subject must stay ASCII (no middot)")
	}
	if simple.Body == nil || simple.Body.Text == nil || simple.Body.Text.Data == nil {
		t.Fatal("expected text body")
	}
	if !strings.Contains(*simple.Body.Text.Data, report.Message) {
		t.Fatal("message body missing report text")
	}
	if strings.Contains(strings.ToLower(*simple.Body.Text.Data), "<html") {
		t.Fatal("must be plaintext")
	}
	if simple.Body.Html != nil {
		t.Fatal("must not set HTML body")
	}
}

func TestSES_Publish_NoReplyToWithoutEmail(t *testing.T) {
	reg := testRegistry(t, `
apps:
  - id: example-app
    origins: ["https://app.example.com"]
    hosts: ["app.example.com"]
    notificationEmail: ops@example.com
    fromEmail: noreply@example.com
`)
	fake := &fakeSES{}
	pub := &notify.SES{Client: fake, Registry: reg}
	err := pub.Publish(context.Background(), &feedback.Report{
		ID:       "01TEST",
		AppID:    "example-app",
		Category: feedback.CategoryOther,
		Message:  "long enough message",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.last.ReplyToAddresses) != 0 {
		t.Fatalf("unexpected replyTo=%v", fake.last.ReplyToAddresses)
	}
}

func TestSES_Publish_Error(t *testing.T) {
	reg := testRegistry(t, `
apps:
  - id: example-app
    origins: ["https://app.example.com"]
    hosts: ["app.example.com"]
    notificationEmail: ops@example.com
    fromEmail: noreply@example.com
`)
	fake := &fakeSES{err: errors.New("boom")}
	pub := &notify.SES{Client: fake, Registry: reg}
	err := pub.Publish(context.Background(), &feedback.Report{ID: "1", AppID: "example-app", Category: "bug", Message: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSES_Publish_PerAppEmails(t *testing.T) {
	reg := testRegistry(t, `
apps:
  - id: example-app
    origins: ["https://app.example.com"]
    hosts: ["app.example.com"]
    notificationEmail: app-ops@example.com
    fromEmail: app-noreply@example.com
  - id: other-app
    origins: ["https://other.example.com"]
    hosts: ["other.example.com"]
    notificationEmail: other-ops@example.com
    fromEmail: other-noreply@example.com
`)

	tests := []struct {
		name     string
		appID    string
		wantFrom string
		wantTo   string
	}{
		{
			name:     "example-app",
			appID:    "example-app",
			wantFrom: "app-noreply@example.com",
			wantTo:   "app-ops@example.com",
		},
		{
			name:     "other-app",
			appID:    "other-app",
			wantFrom: "other-noreply@example.com",
			wantTo:   "other-ops@example.com",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeSES{}
			pub := &notify.SES{Client: fake, Registry: reg}
			err := pub.Publish(context.Background(), &feedback.Report{
				ID:       "01TEST",
				AppID:    tc.appID,
				Category: feedback.CategoryBug,
				Message:  "Something broke somewhere.",
			})
			if err != nil {
				t.Fatal(err)
			}
			if fake.last.FromEmailAddress == nil || *fake.last.FromEmailAddress != tc.wantFrom {
				t.Fatalf("from=%v want %q", fake.last.FromEmailAddress, tc.wantFrom)
			}
			if fake.last.Destination == nil || len(fake.last.Destination.ToAddresses) != 1 || fake.last.Destination.ToAddresses[0] != tc.wantTo {
				t.Fatalf("to=%v want %q", fake.last.Destination, tc.wantTo)
			}
		})
	}
}

func TestSES_Publish_UnknownApp(t *testing.T) {
	reg := testRegistry(t, `
apps:
  - id: example-app
    origins: ["https://app.example.com"]
    hosts: ["app.example.com"]
    notificationEmail: ops@example.com
    fromEmail: noreply@example.com
`)
	fake := &fakeSES{}
	pub := &notify.SES{Client: fake, Registry: reg}
	err := pub.Publish(context.Background(), &feedback.Report{
		ID:       "01TEST",
		AppID:    "missing-app",
		Category: feedback.CategoryBug,
		Message:  "Something broke somewhere.",
	})
	if err == nil {
		t.Fatal("expected error for unknown app")
	}
	if fake.last != nil {
		t.Fatal("SendEmail must not be called")
	}
}

func TestStdout_Publish(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	pub := notify.Stdout{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	err = pub.Publish(context.Background(), &feedback.Report{
		ID:       "01TEST",
		AppID:    "example-app",
		Category: feedback.CategorySuggestion,
		Message:  "Please add an export button somewhere.",
		Locale:   "en",
		PageURL:  "https://app.example.com/",
	})
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(r)
	if !strings.Contains(string(out), "Please add an export button somewhere.") {
		t.Fatalf("stdout missing message: %s", out)
	}
}
