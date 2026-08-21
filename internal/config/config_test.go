package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/peniakoff/weles/internal/config"
)

func TestParseAppsYAML(t *testing.T) {
	reg, err := config.ParseAppsYAML([]byte(`
apps:
  - id: example-app
    origins: ["https://app.example.com"]
    hosts: ["app.example.com"]
    notificationEmail: ops@example.com
    fromEmail: noreply@example.com
`))
	if err != nil {
		t.Fatal(err)
	}
	if !reg.AllowsOrigin("example-app", "https://app.example.com") {
		t.Fatal("expected origin allowed")
	}
	if reg.AllowsOrigin("example-app", "https://other.example.com") {
		t.Fatal("unexpected origin")
	}
	if !reg.OriginAllowed("https://app.example.com") {
		t.Fatal("OriginAllowed failed")
	}
	if !reg.AllowsHost("example-app", "app.example.com") {
		t.Fatal("host allow failed")
	}
	if _, ok := reg.Get("missing"); ok {
		t.Fatal("expected missing app")
	}
}

func TestParseAppsYAML_RequiredEmails(t *testing.T) {
	reg, err := config.ParseAppsYAML([]byte(`
apps:
  - id: example-app
    origins: ["https://app.example.com"]
    hosts: ["app.example.com"]
    notificationEmail: ops@example.com
    fromEmail: noreply@example.com
  - id: other-app
    origins: ["https://other.example.com"]
    hosts: ["other.example.com"]
    notificationEmail: other-ops@example.com
    fromEmail: other-noreply@example.com
`))
	if err != nil {
		t.Fatal(err)
	}
	app, ok := reg.Get("example-app")
	if !ok {
		t.Fatal("expected example-app")
	}
	if app.NotificationEmail != "ops@example.com" {
		t.Fatalf("notificationEmail=%q", app.NotificationEmail)
	}
	if app.FromEmail != "noreply@example.com" {
		t.Fatalf("fromEmail=%q", app.FromEmail)
	}
	other, ok := reg.Get("other-app")
	if !ok {
		t.Fatal("expected other-app")
	}
	if other.NotificationEmail != "other-ops@example.com" || other.FromEmail != "other-noreply@example.com" {
		t.Fatalf("other-app emails: %+v", other)
	}
}

func TestParseAppsYAML_TableErrors(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{name: "empty apps", yaml: "apps: []\n"},
		{name: "duplicate id", yaml: `
apps:
  - id: a
    origins: ["https://a.example.com"]
    hosts: ["a.example.com"]
    notificationEmail: ops@example.com
    fromEmail: noreply@example.com
  - id: a
    origins: ["https://b.example.com"]
    hosts: ["b.example.com"]
    notificationEmail: ops@example.com
    fromEmail: noreply@example.com
`},
		{name: "missing origins", yaml: `
apps:
  - id: a
    origins: []
    hosts: ["a.example.com"]
    notificationEmail: ops@example.com
    fromEmail: noreply@example.com
`},
		{name: "missing hosts", yaml: `
apps:
  - id: a
    origins: ["https://a.example.com"]
    hosts: []
    notificationEmail: ops@example.com
    fromEmail: noreply@example.com
`},
		{name: "missing id", yaml: `
apps:
  - id: ""
    origins: ["https://a.example.com"]
    hosts: ["a.example.com"]
    notificationEmail: ops@example.com
    fromEmail: noreply@example.com
`},
		{name: "invalid yaml", yaml: "apps: ["},
		{name: "missing notificationEmail", yaml: `
apps:
  - id: a
    origins: ["https://a.example.com"]
    hosts: ["a.example.com"]
    fromEmail: noreply@example.com
`},
		{name: "missing fromEmail", yaml: `
apps:
  - id: a
    origins: ["https://a.example.com"]
    hosts: ["a.example.com"]
    notificationEmail: ops@example.com
`},
		{name: "bad notificationEmail", yaml: `
apps:
  - id: a
    origins: ["https://a.example.com"]
    hosts: ["a.example.com"]
    notificationEmail: "Ops <ops@example.com>"
    fromEmail: noreply@example.com
`},
		{name: "bad fromEmail", yaml: `
apps:
  - id: a
    origins: ["https://a.example.com"]
    hosts: ["a.example.com"]
    notificationEmail: ops@example.com
    fromEmail: "not-an-email"
`},
		{name: "crlf notificationEmail", yaml: `
apps:
  - id: a
    origins: ["https://a.example.com"]
    hosts: ["a.example.com"]
    notificationEmail: "ops@example.com\ninjected"
    fromEmail: noreply@example.com
`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := config.ParseAppsYAML([]byte(tc.yaml)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestLoadAppsFile_Example(t *testing.T) {
	path := filepath.Join("..", "..", "config", "apps.example.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Skip("apps.example.yaml not found from test cwd")
	}
	reg, err := config.LoadAppsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reg.OriginAllowed("https://app.example.com") {
		t.Fatal("example origin should be allowed")
	}
	app, ok := reg.Get("example-app")
	if !ok || app.NotificationEmail == "" || app.FromEmail == "" {
		t.Fatalf("example app emails missing: %+v", app)
	}
}

func TestLoadAppsFile_Missing(t *testing.T) {
	if _, err := config.LoadAppsFile(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadEnv_APIStage(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"prod", "prod"},
		{"/prod/", "prod"},
		{"  staging  ", "staging"},
		{"$default", "$default"},
	}
	for _, tc := range tests {
		t.Setenv("WELES_API_STAGE", tc.in)
		t.Setenv("WELLS_API_STAGE", "")
		if got := config.LoadEnv().APIStage; got != tc.want {
			t.Fatalf("WELES_API_STAGE=%q → %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestLoadEnv_LegacyWELLSPrefix(t *testing.T) {
	t.Setenv("WELES_NOTIFIER", "")
	t.Setenv("WELLS_NOTIFIER", "ses")
	t.Setenv("WELES_TURNSTILE_MODE", "")
	t.Setenv("WELLS_TURNSTILE_MODE", "cloudflare")
	env := config.LoadEnv()
	if env.Notifier != "ses" {
		t.Fatalf("notifier=%q want ses", env.Notifier)
	}
	if env.TurnstileMode != "cloudflare" {
		t.Fatalf("turnstileMode=%q want cloudflare", env.TurnstileMode)
	}
}

func TestLoadEnv_WELESWinsOverWELLS(t *testing.T) {
	t.Setenv("WELES_NOTIFIER", "stdout")
	t.Setenv("WELLS_NOTIFIER", "ses")
	if got := config.LoadEnv().Notifier; got != "stdout" {
		t.Fatalf("notifier=%q want stdout", got)
	}
}

func TestNormalizeSSMName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"/weles/prod/apps", "/weles/prod/apps"},
		{"weles/prod/apps", "/weles/prod/apps"},
		{"  /x  ", "/x"},
	}
	for _, tc := range tests {
		if got := config.NormalizeSSMName(tc.in); got != tc.want {
			t.Fatalf("NormalizeSSMName(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestReloadingRegistry(t *testing.T) {
	var calls atomic.Int32
	yamlA := []byte(`
apps:
  - id: example-app
    origins: ["https://app.example.com"]
    hosts: ["app.example.com"]
    notificationEmail: ops@example.com
    fromEmail: noreply@example.com
`)
	yamlB := []byte(`
apps:
  - id: example-app
    origins: ["https://other.example.com"]
    hosts: ["other.example.com"]
    notificationEmail: ops@example.com
    fromEmail: noreply@example.com
`)
	useB := atomic.Bool{}
	load := func() (*config.Registry, error) {
		calls.Add(1)
		if useB.Load() {
			return config.ParseAppsYAML(yamlB)
		}
		return config.ParseAppsYAML(yamlA)
	}

	reg, err := config.NewReloadingRegistry(10*time.Millisecond, load)
	if err != nil {
		t.Fatal(err)
	}
	if !reg.OriginAllowed("https://app.example.com") {
		t.Fatal("expected initial origin")
	}

	useB.Store(true)
	time.Sleep(25 * time.Millisecond)
	if !reg.OriginAllowed("https://other.example.com") {
		t.Fatal("expected reloaded origin")
	}
	if calls.Load() < 2 {
		t.Fatalf("expected reload calls, got %d", calls.Load())
	}
}

func TestReloadingRegistry_KeepsLastOnError(t *testing.T) {
	yamlOK := []byte(`
apps:
  - id: example-app
    origins: ["https://app.example.com"]
    hosts: ["app.example.com"]
    notificationEmail: ops@example.com
    fromEmail: noreply@example.com
`)
	fail := atomic.Bool{}
	load := func() (*config.Registry, error) {
		if fail.Load() {
			return nil, errors.New("ssm down")
		}
		return config.ParseAppsYAML(yamlOK)
	}
	reg, err := config.NewReloadingRegistry(10*time.Millisecond, load)
	if err != nil {
		t.Fatal(err)
	}
	fail.Store(true)
	time.Sleep(25 * time.Millisecond)
	if !reg.OriginAllowed("https://app.example.com") {
		t.Fatal("should keep last good registry")
	}
	if reg.LastLoadError() == nil {
		t.Fatal("expected last load error")
	}
}
