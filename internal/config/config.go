// Package config loads runtime settings and the application allowlist registry.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// App describes one integrating application and its browser allowlists.
type App struct {
	ID      string   `yaml:"id"`
	Origins []string `yaml:"origins"`
	Hosts   []string `yaml:"hosts"`
}

// AppsFile is the on-disk shape of the application registry.
type AppsFile struct {
	Apps []App `yaml:"apps"`
}

// AppRegistry is the allowlist lookup used by validation and HTTP handlers.
type AppRegistry interface {
	Get(id string) (App, bool)
	AllowsOrigin(appID, origin string) bool
	AllowsHost(appID, host string) bool
	OriginAllowed(origin string) bool
}

// Registry looks up apps by ID and validates origins/hosts.
type Registry struct {
	byID map[string]App
}

// LoadAppsFile reads an apps.yaml (or apps.example.yaml) from disk.
func LoadAppsFile(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read apps config: %w", err)
	}
	return ParseAppsYAML(data)
}

// ParseAppsYAML parses application registry YAML bytes.
func ParseAppsYAML(data []byte) (*Registry, error) {
	var file AppsFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse apps config: %w", err)
	}
	if len(file.Apps) == 0 {
		return nil, fmt.Errorf("apps config: at least one app is required")
	}

	byID := make(map[string]App, len(file.Apps))
	for i, app := range file.Apps {
		id := strings.TrimSpace(app.ID)
		if id == "" {
			return nil, fmt.Errorf("apps config: app[%d] missing id", i)
		}
		if _, exists := byID[id]; exists {
			return nil, fmt.Errorf("apps config: duplicate app id %q", id)
		}
		if len(app.Origins) == 0 {
			return nil, fmt.Errorf("apps config: app %q requires at least one origin", id)
		}
		if len(app.Hosts) == 0 {
			return nil, fmt.Errorf("apps config: app %q requires at least one host", id)
		}

		origins := make([]string, 0, len(app.Origins))
		for _, o := range app.Origins {
			o = strings.TrimSpace(o)
			if o == "" {
				continue
			}
			origins = append(origins, o)
		}
		hosts := make([]string, 0, len(app.Hosts))
		for _, h := range app.Hosts {
			h = strings.ToLower(strings.TrimSpace(h))
			if h == "" {
				continue
			}
			hosts = append(hosts, h)
		}
		if len(origins) == 0 || len(hosts) == 0 {
			return nil, fmt.Errorf("apps config: app %q has empty allowlists after trim", id)
		}
		byID[id] = App{ID: id, Origins: origins, Hosts: hosts}
	}

	return &Registry{byID: byID}, nil
}

// Get returns an app by ID.
func (r *Registry) Get(id string) (App, bool) {
	app, ok := r.byID[strings.TrimSpace(id)]
	return app, ok
}

// AllowsOrigin reports whether origin is allowed for the given app.
func (r *Registry) AllowsOrigin(appID, origin string) bool {
	app, ok := r.Get(appID)
	if !ok {
		return false
	}
	origin = strings.TrimSpace(origin)
	for _, allowed := range app.Origins {
		if allowed == origin {
			return true
		}
	}
	return false
}

// AllowsHost reports whether host is allowed for the given app.
// host should already be hostname without port (e.g. from url.Hostname).
func (r *Registry) AllowsHost(appID, host string) bool {
	app, ok := r.Get(appID)
	if !ok {
		return false
	}
	host = strings.ToLower(strings.TrimSpace(host))
	for _, allowed := range app.Hosts {
		if allowed == host {
			return true
		}
	}
	return false
}

// OriginAllowed reports whether any registered app allows the Origin.
func (r *Registry) OriginAllowed(origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return false
	}
	for _, app := range r.byID {
		for _, allowed := range app.Origins {
			if allowed == origin {
				return true
			}
		}
	}
	return false
}

// ReloadingRegistry periodically reloads an underlying registry (e.g. from SSM).
type ReloadingRegistry struct {
	TTL  time.Duration
	Load func() (*Registry, error)

	mu      sync.RWMutex
	current *Registry
	loaded  time.Time
	lastErr error
}

// NewReloadingRegistry loads once and refreshes after TTL. TTL <= 0 means load once only.
func NewReloadingRegistry(ttl time.Duration, load func() (*Registry, error)) (*ReloadingRegistry, error) {
	reg, err := load()
	if err != nil {
		return nil, err
	}
	return &ReloadingRegistry{
		TTL:     ttl,
		Load:    load,
		current: reg,
		loaded:  time.Now(),
	}, nil
}

func (r *ReloadingRegistry) snapshot() *Registry {
	r.mu.RLock()
	ttl := r.TTL
	loaded := r.loaded
	current := r.current
	r.mu.RUnlock()

	if ttl <= 0 || time.Since(loaded) < ttl {
		return current
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.TTL > 0 && time.Since(r.loaded) >= r.TTL {
		next, err := r.Load()
		if err != nil {
			// Keep serving the last good registry; surface via lastErr for operators.
			r.lastErr = err
		} else {
			r.current = next
			r.loaded = time.Now()
			r.lastErr = nil
		}
	}
	return r.current
}

// LastLoadError returns the most recent reload failure, if any.
func (r *ReloadingRegistry) LastLoadError() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastErr
}

// Get implements AppRegistry.
func (r *ReloadingRegistry) Get(id string) (App, bool) {
	return r.snapshot().Get(id)
}

// AllowsOrigin implements AppRegistry.
func (r *ReloadingRegistry) AllowsOrigin(appID, origin string) bool {
	return r.snapshot().AllowsOrigin(appID, origin)
}

// AllowsHost implements AppRegistry.
func (r *ReloadingRegistry) AllowsHost(appID, host string) bool {
	return r.snapshot().AllowsHost(appID, host)
}

// OriginAllowed implements AppRegistry.
func (r *ReloadingRegistry) OriginAllowed(origin string) bool {
	return r.snapshot().OriginAllowed(origin)
}

// NormalizeSSMName ensures an SSM parameter name starts with "/".
func NormalizeSSMName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if !strings.HasPrefix(name, "/") {
		return "/" + name
	}
	return name
}

// Env holds process environment configuration.
type Env struct {
	ListenAddr       string
	AppsConfigPath   string
	AppsYAML         string // optional inline YAML; preferred over file when set
	AppsSSMParam     string // optional SSM parameter name with apps YAML
	AppsReloadEvery  time.Duration
	TurnstileMode    string // "cloudflare" | "skip"
	TurnstileSecret  string // local/dev; prefer TurnstileSSM in production
	TurnstileSSM     string // SSM SecureString parameter name
	Notifier         string // "ses" | "stdout"
	SESFrom          string
	SESTo            string
	AWSRegion        string
	MaxBodyBytes     int64
	LogLevel         string
	TrustProxyXFF    bool   // when true, prefer first X-Forwarded-For hop
	APIStage         string // API Gateway HTTP API stage; stripped from Lambda request paths
}

// LoadEnv reads configuration from environment variables with safe defaults for local use.
func LoadEnv() Env {
	return Env{
		ListenAddr:      getenv("WELLS_LISTEN_ADDR", ":8080"),
		AppsConfigPath:  getenv("WELLS_APPS_CONFIG", "config/apps.example.yaml"),
		AppsYAML:        os.Getenv("WELLS_APPS_YAML"),
		AppsSSMParam:    NormalizeSSMName(os.Getenv("WELLS_APPS_SSM")),
		AppsReloadEvery: getenvDuration("WELLS_APPS_RELOAD_SECONDS", 0),
		TurnstileMode:   strings.ToLower(getenv("WELLS_TURNSTILE_MODE", "skip")),
		TurnstileSecret: os.Getenv("WELLS_TURNSTILE_SECRET"),
		TurnstileSSM:    NormalizeSSMName(os.Getenv("WELLS_TURNSTILE_SSM")),
		Notifier:        strings.ToLower(getenv("WELLS_NOTIFIER", "stdout")),
		SESFrom:         strings.TrimSpace(os.Getenv("WELLS_SES_FROM")),
		SESTo:           strings.TrimSpace(os.Getenv("WELLS_SES_TO")),
		AWSRegion:       getenv("AWS_REGION", "eu-central-1"),
		MaxBodyBytes:    getenvInt64("WELLS_MAX_BODY_BYTES", 8*1024),
		LogLevel:        strings.ToLower(getenv("WELLS_LOG_LEVEL", "info")),
		TrustProxyXFF:   getenvBool("WELLS_TRUST_PROXY_XFF", false),
		APIStage:        strings.Trim(strings.TrimSpace(os.Getenv("WELLS_API_STAGE")), "/"),
	}
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getenvInt64(key string, fallback int64) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return fallback
	}
	return time.Duration(n) * time.Second
}

func getenvBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
