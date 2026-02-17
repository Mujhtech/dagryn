package config

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/mujhtech/dagryn/pkg/telemetry"
	"github.com/mujhtech/dagryn/pkg/database"
	"github.com/mujhtech/dagryn/pkg/redis"
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
	Key             string `toml:"key"`              // The full license key string
	ServerURL       string `toml:"server_url"`       // External License Server URL
	CheckRevocation *bool  `toml:"check_revocation"` // nil => default true
	InstanceName    string `toml:"instance_name"`    // Human-readable instance name
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

	// CloudMode when true indicates this is the managed cloud deployment.
	// In cloud mode, the license system is completely disabled and all feature/quota
	// gating is handled by the billing system. Self-hosted users with their own Stripe
	// key should leave this false.
	// Set via DAGRYN_CLOUD_MODE=true or [cloud_mode] in config.
	CloudMode bool           `toml:"cloud_mode"`
	AI        AIServerConfig `toml:"ai"`
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
	BaseURL         string        `toml:"base_url"` // Public-facing URL (default: https://dagryn.dev)
	ReadTimeout     time.Duration `toml:"read_timeout"`
	WriteTimeout    time.Duration `toml:"write_timeout"`
	ShutdownTimeout time.Duration `toml:"shutdown_timeout"`
	Swagger         SwaggerConfig `toml:"swagger"`
}

// SwaggerConfig holds Swagger UI configuration.
type SwaggerConfig struct {
	Enabled bool   `toml:"enabled"`
	Path    string `toml:"path"`
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	JWTSecret        string        `toml:"jwt_secret"`
	JWTAccessExpiry  time.Duration `toml:"jwt_access_expiry"`
	JWTRefreshExpiry time.Duration `toml:"jwt_refresh_expiry"`
}

// OAuthConfig holds OAuth provider configuration.
type OAuthConfig struct {
	GitHub GitHubOAuthConfig `toml:"github"`
	Google GoogleOAuthConfig `toml:"google"`
}

// GitHubAppConfig holds configuration for the GitHub App integration.
type GitHubAppConfig struct {
	// AppID is the numeric GitHub App ID.
	AppID int64 `toml:"app_id"`
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

	// Load from config file if specified
	if opts.ConfigFile != "" {
		if err := loadConfigFile(opts.ConfigFile, &cfg); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Override with environment variables
	applyEnvVars(&cfg)

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

// applyEnvVars applies environment variable overrides.
func applyEnvVars(cfg *Config) {
	// Server
	if v := getEnvAny("DAGRYN_BASE_URL"); v != "" {
		cfg.Server.BaseURL = v
	}
	if v := os.Getenv("DAGRYN_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("DAGRYN_PORT"); v != "" {
		var port int
		if _, err := fmt.Sscanf(v, "%d", &port); err == nil && port > 0 {
			cfg.Server.Port = port
		}
	}

	// Database - check multiple env var names
	if v := getEnvAny("DATABASE_URL", "DAGRYN_DATABASE_URL", "POSTGRES_URL"); v != "" {
		cfg.Database.URL = v
	}

	// Auth
	if v := getEnvAny("JWT_SECRET", "DAGRYN_JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}

	// GitHub OAuth
	if v := getEnvAny("GITHUB_CLIENT_ID", "DAGRYN_GITHUB_CLIENT_ID"); v != "" {
		cfg.OAuth.GitHub.ClientID = v
	}
	if v := getEnvAny("GITHUB_CLIENT_SECRET", "DAGRYN_GITHUB_CLIENT_SECRET"); v != "" {
		cfg.OAuth.GitHub.ClientSecret = v
	}

	// GitHub App
	if v := getEnvAny("GITHUB_APP_ID", "DAGRYN_GITHUB_APP_ID"); v != "" {
		var id int64
		if _, err := fmt.Sscanf(v, "%d", &id); err == nil && id > 0 {
			cfg.GitHubApp.AppID = id
		}
	}
	if v := getEnvAny("GITHUB_APP_CLIENT_ID", "DAGRYN_GITHUB_APP_CLIENT_ID"); v != "" {
		cfg.GitHubApp.ClientID = v
	}
	if v := getEnvAny("GITHUB_APP_PRIVATE_KEY", "DAGRYN_GITHUB_APP_PRIVATE_KEY"); v != "" {
		cfg.GitHubApp.PrivateKey = v
	}
	if v := getEnvAny("GITHUB_APP_WEBHOOK_SECRET", "DAGRYN_GITHUB_APP_WEBHOOK_SECRET"); v != "" {
		cfg.GitHubApp.WebhookSecret = v
	}

	// Google OAuth
	if v := getEnvAny("GOOGLE_CLIENT_ID", "DAGRYN_GOOGLE_CLIENT_ID"); v != "" {
		cfg.OAuth.Google.ClientID = v
	}
	if v := getEnvAny("GOOGLE_CLIENT_SECRET", "DAGRYN_GOOGLE_CLIENT_SECRET"); v != "" {
		cfg.OAuth.Google.ClientSecret = v
	}

	// Telemetry
	if v := os.Getenv("OTEL_SERVICE_NAME"); v != "" {
		cfg.Telemetry.ServiceName = v
	}
	if v := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); v != "" {
		cfg.Telemetry.Traces.Endpoint = v
		cfg.Telemetry.Metrics.Endpoint = v
	}

	// Redis
	if v := getEnvAny("REDIS_HOST", "DAGRYN_REDIS_HOST"); v != "" {
		cfg.Redis.Host = v
	}
	if v := getEnvAny("REDIS_PORT", "DAGRYN_REDIS_PORT"); v != "" {
		var port int
		if _, err := fmt.Sscanf(v, "%d", &port); err == nil && port > 0 {
			cfg.Redis.Port = port
		}
	}
	if v := getEnvAny("REDIS_PASSWORD", "DAGRYN_REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}

	// Job
	if v := getEnvAny("JOB_ENCRYPTION_KEY", "DAGRYN_JOB_ENCRYPTION_KEY"); v != "" {
		cfg.Job.EncryptionKey = v
	}
	if v := os.Getenv("DAGRYN_JOB_ENABLED"); v == "true" || v == "1" {
		cfg.Job.Enabled = true
	}

	// Cache Storage
	if v := getEnvAny("DAGRYN_CACHE_STORAGE_PROVIDER"); v != "" {
		cfg.CacheStorage.Provider = v
	}
	if v := getEnvAny("DAGRYN_CACHE_STORAGE_BUCKET"); v != "" {
		cfg.CacheStorage.Bucket = v
	}
	if v := getEnvAny("DAGRYN_CACHE_STORAGE_REGION"); v != "" {
		cfg.CacheStorage.Region = v
	}
	if v := getEnvAny("DAGRYN_CACHE_STORAGE_ENDPOINT"); v != "" {
		cfg.CacheStorage.Endpoint = v
	}
	if v := getEnvAny("DAGRYN_CACHE_STORAGE_ACCESS_KEY_ID"); v != "" {
		cfg.CacheStorage.AccessKeyID = v
	}
	if v := getEnvAny("DAGRYN_CACHE_STORAGE_SECRET_ACCESS_KEY"); v != "" {
		cfg.CacheStorage.SecretAccessKey = v
	}
	if v := getEnvAny("DAGRYN_CACHE_STORAGE_BASE_PATH"); v != "" {
		cfg.CacheStorage.BasePath = v
	}
	if v := getEnvAny("DAGRYN_CACHE_STORAGE_PREFIX"); v != "" {
		cfg.CacheStorage.Prefix = v
	}
	if v := getEnvAny("DAGRYN_CACHE_STORAGE_CREDENTIALS_FILE"); v != "" {
		cfg.CacheStorage.CredentialsFile = v
	}
	if v := os.Getenv("DAGRYN_CACHE_STORAGE_USE_PATH_STYLE"); v == "true" || v == "1" {
		cfg.CacheStorage.UsePathStyle = true
	}

	// Artifact Storage
	if v := getEnvAny("DAGRYN_ARTIFACT_STORAGE_PROVIDER"); v != "" {
		cfg.ArtifactStorage.Provider = v
	}
	if v := getEnvAny("DAGRYN_ARTIFACT_STORAGE_BUCKET"); v != "" {
		cfg.ArtifactStorage.Bucket = v
	}
	if v := getEnvAny("DAGRYN_ARTIFACT_STORAGE_REGION"); v != "" {
		cfg.ArtifactStorage.Region = v
	}
	if v := getEnvAny("DAGRYN_ARTIFACT_STORAGE_ENDPOINT"); v != "" {
		cfg.ArtifactStorage.Endpoint = v
	}
	if v := getEnvAny("DAGRYN_ARTIFACT_STORAGE_ACCESS_KEY_ID"); v != "" {
		cfg.ArtifactStorage.AccessKeyID = v
	}
	if v := getEnvAny("DAGRYN_ARTIFACT_STORAGE_SECRET_ACCESS_KEY"); v != "" {
		cfg.ArtifactStorage.SecretAccessKey = v
	}
	if v := getEnvAny("DAGRYN_ARTIFACT_STORAGE_BASE_PATH"); v != "" {
		cfg.ArtifactStorage.BasePath = v
	}
	if v := getEnvAny("DAGRYN_ARTIFACT_STORAGE_PREFIX"); v != "" {
		cfg.ArtifactStorage.Prefix = v
	}
	if v := getEnvAny("DAGRYN_ARTIFACT_STORAGE_CREDENTIALS_FILE"); v != "" {
		cfg.ArtifactStorage.CredentialsFile = v
	}
	if v := os.Getenv("DAGRYN_ARTIFACT_STORAGE_USE_PATH_STYLE"); v == "true" || v == "1" {
		cfg.ArtifactStorage.UsePathStyle = true
	}

	// Container
	if v := os.Getenv("DAGRYN_CONTAINER_ENABLED"); v == "true" || v == "1" {
		cfg.Container.Enabled = true
	}
	if v := getEnvAny("DAGRYN_CONTAINER_DEFAULT_IMAGE"); v != "" {
		cfg.Container.DefaultImage = v
	}
	if v := getEnvAny("DAGRYN_CONTAINER_MEMORY_LIMIT"); v != "" {
		cfg.Container.MemoryLimit = v
	}
	if v := getEnvAny("DAGRYN_CONTAINER_CPU_LIMIT"); v != "" {
		cfg.Container.CPULimit = v
	}
	if v := getEnvAny("DAGRYN_CONTAINER_NETWORK"); v != "" {
		cfg.Container.Network = v
	}

	// License
	if v := getEnvAny("DAGRYN_LICENSE_KEY"); v != "" {
		cfg.License.Key = v
	}
	if v := getEnvAny("DAGRYN_LICENSE_SERVER_URL"); v != "" {
		cfg.License.ServerURL = v
	}
	if v := os.Getenv("DAGRYN_LICENSE_CHECK_REVOCATION"); v == "false" || v == "0" {
		f := false
		cfg.License.CheckRevocation = &f
	}
	if v := getEnvAny("DAGRYN_LICENSE_INSTANCE_NAME"); v != "" {
		cfg.License.InstanceName = v
	}

	// Health
	if v := os.Getenv("DAGRYN_READY_CHECK_DATABASE"); v == "false" || v == "0" {
		cfg.Health.ReadyCheckDatabase = false
	}
	if v := os.Getenv("DAGRYN_READY_CHECK_REDIS"); v == "true" || v == "1" {
		cfg.Health.ReadyCheckRedis = true
	}

	// Cloud mode
	if v := os.Getenv("DAGRYN_CLOUD_MODE"); v == "true" || v == "1" {
		cfg.CloudMode = true
	}
	// AI
	if v := os.Getenv("DAGRYN_AI_ENABLED"); v == "true" || v == "1" {
		cfg.AI.Enabled = true
	}
	if v := getEnvAny("DAGRYN_AI_PROVIDER"); v != "" {
		cfg.AI.Provider = v
	}
	if v := getEnvAny("DAGRYN_AI_API_KEY"); v != "" {
		cfg.AI.APIKey = v
	}
	if v := os.Getenv("DAGRYN_AI_TIMEOUT_SECONDS"); v != "" {
		var secs int
		if _, err := fmt.Sscanf(v, "%d", &secs); err == nil && secs > 0 {
			cfg.AI.TimeoutSeconds = secs
		}
	}
	if v := os.Getenv("DAGRYN_AI_MAX_TOKENS"); v != "" {
		var tokens int
		if _, err := fmt.Sscanf(v, "%d", &tokens); err == nil && tokens > 0 {
			cfg.AI.MaxTokens = tokens
		}
	}
	if v := getEnvAny("DAGRYN_AI_BACKEND_MODE"); v != "" {
		cfg.AI.BackendMode = v
	}
	if v := getEnvAny("DAGRYN_AI_AGENT_ENDPOINT"); v != "" {
		cfg.AI.AgentEndpoint = v
	}
	if v := getEnvAny("DAGRYN_AI_AGENT_TOKEN"); v != "" {
		cfg.AI.AgentToken = v
	}
	if v := os.Getenv("DAGRYN_AI_MAX_ANALYSES_PER_HOUR"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.AI.MaxAnalysesPerHour = n
		}
	}
	if v := os.Getenv("DAGRYN_AI_COOLDOWN_SECONDS"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.AI.CooldownSeconds = n
		}
	}
	if v := os.Getenv("DAGRYN_AI_MAX_CONCURRENT_ANALYSES"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.AI.MaxConcurrentAnalyses = n
		}
	}
	if v := os.Getenv("DAGRYN_AI_RAW_RESPONSE_TTL_HOURS"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.AI.RawResponseTTLHours = n
		}
	}
	if v := getEnvAny("DAGRYN_AI_RAW_RESPONSE_STORAGE"); v != "" {
		cfg.AI.RawResponseStorage = v
	}

	// Stripe
	if v := getEnvAny("STRIPE_SECRET_KEY", "DAGRYN_STRIPE_SECRET_KEY"); v != "" {
		cfg.Stripe.SecretKey = v
	}
	if v := getEnvAny("STRIPE_WEBHOOK_SECRET", "DAGRYN_STRIPE_WEBHOOK_SECRET"); v != "" {
		cfg.Stripe.WebhookSecret = v
	}
	if v := getEnvAny("STRIPE_PUBLISHABLE_KEY", "DAGRYN_STRIPE_PUBLISHABLE_KEY"); v != "" {
		cfg.Stripe.PublishableKey = v
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

// getEnvAny returns the value of the first non-empty environment variable.
func getEnvAny(keys ...string) string {
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}
