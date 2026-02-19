package config

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/mujhtech/dagryn/pkg/database"
	"github.com/mujhtech/dagryn/pkg/redis"
	"github.com/mujhtech/dagryn/pkg/telemetry"
)

type CacheProvider string

const (
	// RedisCacheProvider indicates Redis is used for caching.
	RedisCacheProvider    CacheProvider = "redis"
	InMemoryCacheProvider CacheProvider = "in_memory"
)

// CacheConfig holds cache configuration.
type CacheConfig struct {
	Provider CacheProvider `toml:"provider"`
}

// WorkerConfig holds background worker configuration.
type WorkerConfig struct {
	Enabled     bool   `toml:"enabled"`
	Concurrency int    `toml:"concurrency"`
	RoutePrefix string `toml:"route_prefix"`
}

// ContainerServerConfig holds server-level container isolation defaults.
type ContainerServerConfig struct {
	Enabled      bool   `toml:"enabled"`
	DefaultImage string `toml:"default_image"`
	MemoryLimit  string `toml:"memory_limit"`
	CPULimit     string `toml:"cpu_limit"`
	Network      string `toml:"network"`
}

// StripeConfig holds Stripe API configuration.
type StripeConfig struct {
	SecretKey      string `toml:"secret_key"`
	WebhookSecret  string `toml:"webhook_secret"`
	PublishableKey string `toml:"publishable_key"`
}

// LicenseConfig holds self-hosted license key configuration.
type LicenseConfig struct {
	Key             string `toml:"key"`                            // The full license key string
	ServerURL       string `toml:"server_url"`                     // External License Server URL
	CheckRevocation *bool  `toml:"check_revocation" envconfig:"-"` // nil => default true; handled manually
	InstanceName    string `toml:"instance_name"`                  // Human-readable instance name
}

// IsRevocationCheckEnabled returns whether periodic license revocation checks are enabled.
func (c *LicenseConfig) IsRevocationCheckEnabled() bool {
	if c.CheckRevocation == nil {
		return true
	}
	return *c.CheckRevocation
}

// AIServerConfig holds server-level AI analysis configuration.
type AIServerConfig struct {
	Enabled               bool   `toml:"enabled"`
	Provider              string `toml:"provider"` // "openai", "google", "gemini"
	APIKey                string `toml:"api_key"`
	TimeoutSeconds        int    `toml:"timeout_seconds"`
	MaxTokens             int    `toml:"max_tokens"`
	BackendMode           string `toml:"backend_mode"`
	AgentEndpoint         string `toml:"agent_endpoint"`
	AgentToken            string `toml:"agent_token"`
	MaxAnalysesPerHour    int    `toml:"max_analyses_per_hour"`
	CooldownSeconds       int    `toml:"cooldown_seconds"`
	MaxConcurrentAnalyses int    `toml:"max_concurrent_analyses"`
	RawResponseTTLHours   int    `toml:"raw_response_ttl_hours"`
	RawResponseStorage    string `toml:"raw_response_storage"`
}

// Config holds all server configuration.
type Config struct {
	Worker          WorkerConfig          `toml:"worker"`
	Server          ServerConfig          `toml:"server"`
	Database        database.Config       `toml:"database"`
	Redis           redis.Config          `toml:"redis"`
	Auth            AuthConfig            `toml:"auth"`
	OAuth           OAuthConfig           `toml:"oauth"`
	Telemetry       telemetry.Config      `toml:"telemetry"`
	Job             JobConfig             `toml:"job"`
	Health          HealthConfig          `toml:"health"`
	GitHubApp       GitHubAppConfig       `toml:"github_app"`
	CacheStorage    StorageConfig         `toml:"cache_storage"`
	ArtifactStorage StorageConfig         `toml:"artifact_storage"`
	Container       ContainerServerConfig `toml:"container"`
	Stripe          StripeConfig          `toml:"stripe"`
	License         LicenseConfig         `toml:"license"`
	Cache           CacheConfig           `toml:"cache"`
	AI              AIServerConfig        `toml:"ai"`
}

// StorageConfig holds cache storage backend configuration.
type StorageConfig struct {
	Provider        string `toml:"provider"`
	Bucket          string `toml:"bucket"`
	Region          string `toml:"region"`
	Endpoint        string `toml:"endpoint"`
	AccessKeyID     string `toml:"access_key_id"`
	SecretAccessKey string `toml:"secret_access_key"`
	UsePathStyle    bool   `toml:"use_path_style"`
	BasePath        string `toml:"base_path"`
	Prefix          string `toml:"prefix"`
	CredentialsFile string `toml:"credentials_file"`
}

// HealthConfig holds health/readiness check configuration.
type HealthConfig struct {
	// ReadyCheckDatabase runs a DB ping in /ready when true (default true).
	ReadyCheckDatabase bool `toml:"ready_check_database"`
	// ReadyCheckRedis runs a Redis ping in /ready when true (default false).
	ReadyCheckRedis bool `toml:"ready_check_redis"`
}

// DefaultBaseURL is the public-facing URL used in GitHub check runs, AI comments, etc.
const DefaultBaseURL = "https://dagryn.dev"

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Host            string        `toml:"host"`
	Port            int           `toml:"port"`
	BaseURL         string        `toml:"base_url" envconfig:"BASE_URL"` // Public-facing URL (default: https://dagryn.dev)
	ReadTimeout     time.Duration `toml:"read_timeout" envconfig:"-"`
	WriteTimeout    time.Duration `toml:"write_timeout" envconfig:"-"`
	ShutdownTimeout time.Duration `toml:"shutdown_timeout" envconfig:"-"`
	Swagger         SwaggerConfig `toml:"swagger" envconfig:"-"`
}

// SwaggerConfig holds Swagger UI configuration.
type SwaggerConfig struct {
	Enabled bool   `toml:"enabled"`
	Path    string `toml:"path"`
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	JWTSecret        string        `toml:"jwt_secret"`
	JWTAccessExpiry  time.Duration `toml:"jwt_access_expiry" envconfig:"-"`
	JWTRefreshExpiry time.Duration `toml:"jwt_refresh_expiry" envconfig:"-"`
}

// OAuthConfig holds OAuth provider configuration.
type OAuthConfig struct {
	GitHub GitHubOAuthConfig `toml:"github"`
	Google GoogleOAuthConfig `toml:"google"`
}

// GitHubAppConfig holds configuration for the GitHub App integration.
type GitHubAppConfig struct {
	// AppID is the numeric GitHub App ID.
	AppID int64 `toml:"app_id" envconfig:"ID"`
	// ClientID is the OAuth client ID for the GitHub App (used for connect/install flows).
	ClientID string `toml:"client_id"`
	// PrivateKey is the PEM-encoded private key for the GitHub App.
	// It can be provided inline via config or via environment variable.
	PrivateKey string `toml:"private_key"`
	// WebhookSecret is the shared secret used to verify GitHub webhook signatures.
	WebhookSecret string `toml:"webhook_secret"`
}

// GitHubOAuthConfig holds GitHub OAuth configuration.
type GitHubOAuthConfig struct {
	ClientID     string `toml:"client_id"`
	ClientSecret string `toml:"client_secret"`
}

// GoogleOAuthConfig holds Google OAuth configuration.
type GoogleOAuthConfig struct {
	ClientID     string `toml:"client_id"`
	ClientSecret string `toml:"client_secret"`
}

// JobConfig holds background job worker configuration.
type JobConfig struct {
	// Enabled determines if the job worker should be started with the server.
	Enabled bool `toml:"enabled"`
	// Concurrency is the number of concurrent job workers.
	Concurrency int `toml:"concurrency"`
	// EncryptionKey is the key used to encrypt job payloads (must be 32 bytes for AES-256).
	EncryptionKey string `toml:"encryption_key"`
}

// DefaultJobConfig returns sensible defaults for job configuration.
func DefaultJobConfig() JobConfig {
	return JobConfig{
		Enabled:     false,
		Concurrency: 10,
	}
}

// DefaultConfig returns sensible defaults for server configuration.
func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Host:            "localhost",
			Port:            9000,
			BaseURL:         DefaultBaseURL,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			ShutdownTimeout: 10 * time.Second,
			Swagger: SwaggerConfig{
				Enabled: true,
				Path:    "/swagger",
			},
		},
		Database: database.DefaultConfig(),
		Redis:    redis.DefaultConfig(),
		Auth: AuthConfig{
			JWTAccessExpiry:  15 * time.Minute,
			JWTRefreshExpiry: 7 * 24 * time.Hour,
		},
		Telemetry: telemetry.DefaultConfig(),
		Job:       DefaultJobConfig(),
		Health: HealthConfig{
			ReadyCheckDatabase: true,
			ReadyCheckRedis:    false,
		},
		GitHubApp: GitHubAppConfig{
			// Defaults to disabled; all zero values until configured.
		},
		Worker: WorkerConfig{
			Enabled:     true,
			RoutePrefix: "worker",
		},
	}
}

// LoadConfig loads configuration with priority: CLI flags > env vars > config file > defaults.
// The opts parameter contains values from CLI flags (empty strings/zero values are ignored).
type ConfigOpts struct {
	ConfigFile string
	Host       string
	Port       int
	Database   string
	JWTSecret  string

	// OAuth
	GitHubClientID     string
	GitHubClientSecret string
	GoogleClientID     string
	GoogleClientSecret string
}

// LoadConfig loads the configuration with the specified priority.
func LoadConfig(opts ConfigOpts) (*Config, error) {
	// Start with defaults
	cfg := DefaultConfig()

	// Load .env file + backward-compat aliases
	LoadDotEnv()

	// Load from config file if specified
	if opts.ConfigFile != "" {
		if err := loadConfigFile(opts.ConfigFile, &cfg); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Override with environment variables (envconfig-based)
	ProcessEnvVars(&cfg)

	// Override with CLI flags (highest priority)
	applyCLIFlags(&cfg, opts)

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// loadConfigFile loads configuration from a TOML file.
func loadConfigFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, use defaults
		}
		return err
	}
	return toml.Unmarshal(data, cfg)
}

// dotenvOnce ensures .env loading and env normalization happen only once.
var dotenvOnce sync.Once

// LoadDotEnv loads a .env file (if present) and normalizes backward-compat
// env var aliases. Safe to call multiple times; work is done only once.
func LoadDotEnv() {
	dotenvOnce.Do(func() {
		_ = godotenv.Load() // silent if .env not found
		normalizeEnv()
	})
}

// normalizeEnv copies unprefixed env vars to their DAGRYN-prefixed equivalents
// so that envconfig.Process (which reads only DAGRYN-prefixed vars) picks them up.
// The DAGRYN-prefixed var is only set if it is not already present.
// Aliases are listed in priority order: higher-priority aliases come first.
func normalizeEnv() {
	aliases := []struct{ from, to string }{
		// Database
		{"DATABASE_URL", "DAGRYN_DATABASE_URL"},
		{"POSTGRES_URL", "DAGRYN_DATABASE_URL"},
		// Auth
		{"JWT_SECRET", "DAGRYN_JWT_SECRET"},
		// Redis
		{"REDIS_HOST", "DAGRYN_REDIS_HOST"},
		{"REDIS_PORT", "DAGRYN_REDIS_PORT"},
		{"REDIS_PASSWORD", "DAGRYN_REDIS_PASSWORD"},
		// GitHub OAuth
		{"GITHUB_CLIENT_ID", "DAGRYN_GITHUB_CLIENT_ID"},
		{"GITHUB_CLIENT_SECRET", "DAGRYN_GITHUB_CLIENT_SECRET"},
		// GitHub App
		{"GITHUB_APP_ID", "DAGRYN_GITHUB_APP_ID"},
		{"GITHUB_APP_CLIENT_ID", "DAGRYN_GITHUB_APP_CLIENT_ID"},
		{"GITHUB_APP_PRIVATE_KEY", "DAGRYN_GITHUB_APP_PRIVATE_KEY"},
		{"GITHUB_APP_WEBHOOK_SECRET", "DAGRYN_GITHUB_APP_WEBHOOK_SECRET"},
		// Google OAuth
		{"GOOGLE_CLIENT_ID", "DAGRYN_GOOGLE_CLIENT_ID"},
		{"GOOGLE_CLIENT_SECRET", "DAGRYN_GOOGLE_CLIENT_SECRET"},
		// Job
		{"JOB_ENCRYPTION_KEY", "DAGRYN_JOB_ENCRYPTION_KEY"},
		{"JOB_CONCURRENCY", "DAGRYN_JOB_CONCURRENCY"},
		// Stripe
		{"STRIPE_SECRET_KEY", "DAGRYN_STRIPE_SECRET_KEY"},
		{"STRIPE_WEBHOOK_SECRET", "DAGRYN_STRIPE_WEBHOOK_SECRET"},
		{"STRIPE_PUBLISHABLE_KEY", "DAGRYN_STRIPE_PUBLISHABLE_KEY"},
	}
	for _, a := range aliases {
		if os.Getenv(a.to) == "" {
			if v := os.Getenv(a.from); v != "" {
				_ = os.Setenv(a.to, v)
			}
		}
	}
}

// ProcessEnvVars reads DAGRYN-prefixed environment variables into cfg using
// envconfig. Each config section is processed with its own prefix so that
// the flat env var naming (DAGRYN_HOST, not DAGRYN_SERVER_HOST) is preserved.
// Errors are silently ignored to match the previous fail-open behavior.
func ProcessEnvVars(cfg *Config) {
	// Per-section envconfig processing.
	// envconfig only overwrites fields where the env var is actually set;
	// defaults and config-file values are preserved for unset vars.
	_ = envconfig.Process("DAGRYN", &cfg.Server)
	_ = envconfig.Process("DAGRYN_DATABASE", &cfg.Database)
	_ = envconfig.Process("DAGRYN", &cfg.Auth)
	_ = envconfig.Process("DAGRYN_REDIS", &cfg.Redis)
	_ = envconfig.Process("DAGRYN_GITHUB", &cfg.OAuth.GitHub)
	_ = envconfig.Process("DAGRYN_GOOGLE", &cfg.OAuth.Google)
	_ = envconfig.Process("DAGRYN_GITHUB_APP", &cfg.GitHubApp)
	_ = envconfig.Process("DAGRYN_JOB", &cfg.Job)
	_ = envconfig.Process("DAGRYN_CACHE_STORAGE", &cfg.CacheStorage)
	_ = envconfig.Process("DAGRYN_ARTIFACT_STORAGE", &cfg.ArtifactStorage)
	_ = envconfig.Process("DAGRYN_CONTAINER", &cfg.Container)
	_ = envconfig.Process("DAGRYN_LICENSE", &cfg.License)
	_ = envconfig.Process("DAGRYN", &cfg.Health)
	_ = envconfig.Process("DAGRYN_STRIPE", &cfg.Stripe)
	_ = envconfig.Process("DAGRYN_AI", &cfg.AI)

	// OTEL vars use standard naming (not DAGRYN-prefixed).
	if v := os.Getenv("OTEL_SERVICE_NAME"); v != "" {
		cfg.Telemetry.ServiceName = v
	}
	if v := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); v != "" {
		cfg.Telemetry.Traces.Endpoint = v
		cfg.Telemetry.Metrics.Endpoint = v
	}

	// LicenseConfig.CheckRevocation is *bool — envconfig can't reliably
	// handle pointer-to-bool with our "nil means default true" semantics.
	if v := os.Getenv("DAGRYN_LICENSE_CHECK_REVOCATION"); v == "false" || v == "0" {
		f := false
		cfg.License.CheckRevocation = &f
	}
}

// applyCLIFlags applies CLI flag overrides (highest priority).
func applyCLIFlags(cfg *Config, opts ConfigOpts) {
	if opts.Host != "" {
		cfg.Server.Host = opts.Host
	}
	if opts.Port > 0 {
		cfg.Server.Port = opts.Port
	}
	if opts.Database != "" {
		cfg.Database.URL = opts.Database
	}
	if opts.JWTSecret != "" {
		cfg.Auth.JWTSecret = opts.JWTSecret
	}
	if opts.GitHubClientID != "" {
		cfg.OAuth.GitHub.ClientID = opts.GitHubClientID
	}
	if opts.GitHubClientSecret != "" {
		cfg.OAuth.GitHub.ClientSecret = opts.GitHubClientSecret
	}
	if opts.GoogleClientID != "" {
		cfg.OAuth.Google.ClientID = opts.GoogleClientID
	}
	if opts.GoogleClientSecret != "" {
		cfg.OAuth.Google.ClientSecret = opts.GoogleClientSecret
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Auth.JWTSecret == "" {
		return &ConfigError{
			Field:   "auth.jwt_secret",
			Message: "JWT secret is required. Set via --jwt-secret flag, JWT_SECRET env var, or config file.",
		}
	}

	if len(c.Auth.JWTSecret) < 32 {
		return &ConfigError{
			Field:   "auth.jwt_secret",
			Message: "JWT secret must be at least 32 characters long.",
		}
	}

	// Redis is required for job queues, caching, and pub/sub
	if c.Redis.Host == "" {
		return &ConfigError{
			Field:   "redis.host",
			Message: "Redis host is required. Set via DAGRYN_REDIS_HOST env var or config file.",
		}
	}

	if c.Redis.Port <= 0 {
		return &ConfigError{
			Field:   "redis.port",
			Message: "Redis port must be a positive integer. Set via DAGRYN_REDIS_PORT env var or config file.",
		}
	}

	// At least one OAuth provider must be configured
	hasOAuth := (c.OAuth.GitHub.ClientID != "" && c.OAuth.GitHub.ClientSecret != "") ||
		(c.OAuth.Google.ClientID != "" && c.OAuth.Google.ClientSecret != "")
	if !hasOAuth {
		return &ConfigError{
			Field:   "oauth",
			Message: "At least one OAuth provider (GitHub or Google) must be configured.",
		}
	}

	return nil
}

// Address returns the server address (host:port).
func (c *ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// ConfigError represents a configuration error.
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error: %s: %s", e.Field, e.Message)
}
