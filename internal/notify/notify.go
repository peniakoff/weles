// Package notify delivers validated feedback reports to operators.
package notify

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"github.com/peniakoff/weles/internal/feedback"
)

// Publisher sends a report notification.
type Publisher interface {
	Publish(ctx context.Context, report *feedback.Report) error
}

// Stdout writes a plaintext email body to stdout (local development).
type Stdout struct {
	Logger *slog.Logger
}

// Publish implements Publisher.
func (s Stdout) Publish(_ context.Context, report *feedback.Report) error {
	body := feedback.FormatPlainEmail(report)
	if _, err := fmt.Fprintln(os.Stdout, body); err != nil {
		return err
	}
	if s.Logger != nil {
		s.Logger.Info("feedback published to stdout", "reportId", report.ID, "appId", report.AppID)
	}
	return nil
}

// SES sends a plaintext email via Amazon SES v2.
type SES struct {
	Client SESAPI
	From   string
	To     string
}

// SESAPI is the subset of the SES v2 client used by Weles (for tests).
type SESAPI interface {
	SendEmail(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

// NewSES builds an SES publisher using the default AWS credential chain.
func NewSES(ctx context.Context, region, from, to string) (*SES, error) {
	from, err := requireEmailAddress("SES from", from)
	if err != nil {
		return nil, err
	}
	to, err = requireEmailAddress("SES to", to)
	if err != nil {
		return nil, err
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	return &SES{
		Client: sesv2.NewFromConfig(cfg),
		From:   from,
		To:     to,
	}, nil
}

func requireEmailAddress(label, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("%s address is required", label)
	}
	if strings.ContainsAny(raw, "\r\n") {
		return "", fmt.Errorf("%s address is invalid", label)
	}
	addr, err := mail.ParseAddress(raw)
	if err != nil || addr.Address != raw {
		return "", fmt.Errorf("%s address is invalid", label)
	}
	return addr.Address, nil
}

// Publish implements Publisher.
func (s *SES) Publish(ctx context.Context, report *feedback.Report) error {
	subject := formatSubject(report)
	body := feedback.FormatPlainEmail(report)
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(s.From),
		Destination: &types.Destination{
			ToAddresses: []string{s.To},
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data:    aws.String(subject),
					Charset: aws.String("UTF-8"),
				},
				Body: &types.Body{
					Text: &types.Content{
						Data:    aws.String(body),
						Charset: aws.String("UTF-8"),
					},
				},
			},
		},
	}
	if replyTo := strings.TrimSpace(report.Email); replyTo != "" {
		input.ReplyToAddresses = []string{replyTo}
	}
	_, err := s.Client.SendEmail(ctx, input)
	if err != nil {
		return fmt.Errorf("ses send email: %w", err)
	}
	return nil
}

// formatSubject builds an ASCII email subject.
func formatSubject(report *feedback.Report) string {
	return fmt.Sprintf("[Weles] %s %s %s", report.AppID, report.Category, report.ID)
}
