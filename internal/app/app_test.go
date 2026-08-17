package app_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peniakoff/weles/internal/app"
)

func TestBuild_RejectsUnknownAndLegacyNotifiers(t *testing.T) {
	apps := filepath.Join("..", "..", "config", "apps.example.yaml")
	t.Setenv("WELLS_TURNSTILE_MODE", "skip")
	t.Setenv("WELLS_APPS_CONFIG", apps)
	t.Setenv("WELLS_APPS_YAML", "")
	t.Setenv("WELLS_APPS_SSM", "")

	for _, notifier := range []string{"sns", "unknown"} {
		t.Run(notifier, func(t *testing.T) {
			t.Setenv("WELLS_NOTIFIER", notifier)
			_, err := app.Build(context.Background())
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "WELLS_NOTIFIER") {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
