// Package app wires shared runtime construction for server and lambda entrypoints.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/peniakoff/weles/internal/config"
	"github.com/peniakoff/weles/internal/httpapi"
	"github.com/peniakoff/weles/internal/notify"
	"github.com/peniakoff/weles/internal/turnstile"
)

// Dependencies holds constructed runtime services.
type Dependencies struct {
	Env    config.Env
	Server *httpapi.Server
	Logger *slog.Logger
}

// Build constructs registry, verifier, publisher, and HTTP server from environment.
func Build(ctx context.Context) (*Dependencies, error) {
	env := config.LoadEnv()
	logger := newLogger(env.LogLevel)

	registry, err := loadRegistry(ctx, env)
	if err != nil {
		return nil, err
	}

	verifier, err := buildVerifier(ctx, env)
	if err != nil {
		return nil, err
	}

	publisher, err := buildPublisher(ctx, env, logger)
	if err != nil {
		return nil, err
	}

	srv := &httpapi.Server{
		Registry:      registry,
		Verifier:      verifier,
		Publisher:     publisher,
		MaxBody:       env.MaxBodyBytes,
		Logger:        logger,
		TrustProxyXFF: env.TrustProxyXFF,
	}

	return &Dependencies{Env: env, Server: srv, Logger: logger}, nil
}

// Handler returns the HTTP mux.
func (d *Dependencies) Handler() http.Handler {
	return d.Server.NewMux()
}

func loadRegistry(ctx context.Context, env config.Env) (config.AppRegistry, error) {
	load := func() (*config.Registry, error) {
		if yaml := strings.TrimSpace(env.AppsYAML); yaml != "" {
			return config.ParseAppsYAML([]byte(yaml))
		}
		if env.AppsSSMParam != "" {
			return loadYAMLFromSSM(ctx, env.AWSRegion, env.AppsSSMParam)
		}
		return config.LoadAppsFile(env.AppsConfigPath)
	}

	ttl := env.AppsReloadEvery
	// Default a modest TTL when the registry comes from SSM so allowlist edits apply without redeploy.
	if ttl == 0 && env.AppsSSMParam != "" && strings.TrimSpace(env.AppsYAML) == "" {
		ttl = 5 * time.Minute
	}
	if ttl > 0 {
		return config.NewReloadingRegistry(ttl, load)
	}
	return load()
}

func loadYAMLFromSSM(ctx context.Context, region, paramName string) (*config.Registry, error) {
	value, err := getSSMParameter(ctx, region, paramName)
	if err != nil {
		return nil, err
	}
	return config.ParseAppsYAML([]byte(value))
}

func getSSMParameter(ctx context.Context, region, paramName string) (string, error) {
	paramName = config.NormalizeSSMName(paramName)
	if paramName == "" {
		return "", fmt.Errorf("ssm parameter name is empty")
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return "", fmt.Errorf("aws config for ssm: %w", err)
	}
	out, err := ssm.NewFromConfig(cfg).GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(paramName),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("ssm get parameter %s: %w", paramName, err)
	}
	if out.Parameter == nil || out.Parameter.Value == nil {
		return "", fmt.Errorf("ssm parameter %s empty", paramName)
	}
	return *out.Parameter.Value, nil
}

func buildVerifier(ctx context.Context, env config.Env) (turnstile.Verifier, error) {
	switch env.TurnstileMode {
	case "skip", "dev", "none":
		return turnstile.Skip{}, nil
	case "cloudflare", "on", "enabled":
		secret := strings.TrimSpace(env.TurnstileSecret)
		if secret == "" && env.TurnstileSSM != "" {
			var err error
			secret, err = getSSMParameter(ctx, env.AWSRegion, env.TurnstileSSM)
			if err != nil {
				return nil, fmt.Errorf("turnstile secret from ssm: %w", err)
			}
		}
		return turnstile.NewCloudflare(secret)
	default:
		return nil, fmt.Errorf("unknown WELES_TURNSTILE_MODE %q", env.TurnstileMode)
	}
}

func buildPublisher(ctx context.Context, env config.Env, logger *slog.Logger) (notify.Publisher, error) {
	switch env.Notifier {
	case "stdout", "log":
		return notify.Stdout{Logger: logger}, nil
	case "ses":
		return notify.NewSES(ctx, env.AWSRegion, env.SESFrom, env.SESTo)
	default:
		return nil, fmt.Errorf("unknown WELES_NOTIFIER %q", env.Notifier)
	}
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
