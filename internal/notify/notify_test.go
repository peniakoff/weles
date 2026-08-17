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

	"github.com/peniakoff/weles/internal/feedback"
	"github.com/peniakoff/weles/internal/notify"
)

func TestNewSES_RequiresFromAndTo(t *testing.T) {
	if _, err := notify.NewSES(context.Background(), "eu-central-1", "", "ops@example.com"); err == nil {
		t.Fatal("expected error for empty from")
	}
	if _, err := notify.NewSES(context.Background(), "eu-central-1", "noreply@example.com", ""); err == nil {
		t.Fatal("expected error for empty to")
	}
}

func TestNewSES_RejectsInvalidAddresses(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
	}{
		{name: "bad from", from: "not-an-email", to: "ops@example.com"},
		{name: "bad to", from: "noreply@example.com", to: "not-an-email"},
		{name: "display name from", from: "Ops <ops@example.com>", to: "ops@example.com"},
		{name: "crlf from", from: "noreply@example.com\ninjected", to: "ops@example.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := notify.NewSES(context.Background(), "eu-central-1", tc.from, tc.to); err == nil {
				t.Fatal("expected error")
			}
		})
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
	fake := &fakeSES{}
	pub := &notify.SES{Client: fake, From: "noreply@example.com", To: "ops@example.com"}
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
	if fake.last.FromEmailAddress == nil || *fake.last.FromEmailAddress != pub.From {
		t.Fatalf("from=%v", fake.last.FromEmailAddress)
	}
	if fake.last.Destination == nil || len(fake.last.Destination.ToAddresses) != 1 || fake.last.Destination.ToAddresses[0] != pub.To {
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
	fake := &fakeSES{}
	pub := &notify.SES{Client: fake, From: "noreply@example.com", To: "ops@example.com"}
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
	fake := &fakeSES{err: errors.New("boom")}
	pub := &notify.SES{Client: fake, From: "noreply@example.com", To: "ops@example.com"}
	err := pub.Publish(context.Background(), &feedback.Report{ID: "1", AppID: "example-app", Category: "bug", Message: "x"})
	if err == nil {
		t.Fatal("expected error")
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
