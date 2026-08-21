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
	t.Setenv("WELES_TURNSTILE_MODE", "skip")
	t.Setenv("WELES_APPS_CONFIG", apps)
	t.Setenv("WELES_APPS_YAML", "")
	t.Setenv("WELES_APPS_SSM", "")

	for _, notifier := range []string{"sns", "unknown"} {
		t.Run(notifier, func(t *testing.T) {
			t.Setenv("WELES_NOTIFIER", notifier)
			_, err := app.Build(context.Background())
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "WELES_NOTIFIER") {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
