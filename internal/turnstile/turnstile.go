// Package turnstile verifies Cloudflare Turnstile tokens.
package turnstile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultSiteverifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// Verifier checks a Turnstile response token.
type Verifier interface {
	Verify(ctx context.Context, token, remoteIP string) error
}

// Cloudflare calls the official siteverify endpoint.
type Cloudflare struct {
	Secret    string
	Client    *http.Client
	VerifyURL string // optional; defaults to Cloudflare siteverify
}

// NewCloudflare builds a production verifier.
func NewCloudflare(secret string) (*Cloudflare, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, fmt.Errorf("turnstile secret is required")
	}
	return &Cloudflare{
		Secret: secret,
		Client: &http.Client{Timeout: 5 * time.Second},
	}, nil
}

type siteverifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// Verify implements Verifier.
func (c *Cloudflare) Verify(ctx context.Context, token, remoteIP string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("empty turnstile token")
	}

	form := url.Values{}
	form.Set("secret", c.Secret)
	form.Set("response", token)
	if remoteIP = strings.TrimSpace(remoteIP); remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	endpoint := c.VerifyURL
	if endpoint == "" {
		endpoint = defaultSiteverifyURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("turnstile request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("turnstile verify: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return fmt.Errorf("turnstile read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("turnstile status %d", resp.StatusCode)
	}

	var parsed siteverifyResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("turnstile decode: %w", err)
	}
	if !parsed.Success {
		return fmt.Errorf("turnstile rejected")
	}
	return nil
}

// Skip always accepts tokens (local development only).
type Skip struct{}

// Verify implements Verifier.
func (Skip) Verify(_ context.Context, token, _ string) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("empty turnstile token")
	}
	return nil
}
