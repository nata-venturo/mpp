package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Database DatabaseConfig
	Server   ServerConfig
	Security SecurityConfig
	Auth     AuthConfig
	WAHA     WAHAConfig
	OpenAI   OpenAIConfig
	Redis    RedisConfig
	GCS      GCSConfig
	Firebase FirebaseConfig
	MPP      MPPConfig
}

// MPPConfig holds MPP-domain settings. One MPP building = one company,
// so public (unauthenticated) MPP reads resolve their tenant from
// CompanyID rather than from a request header.
//
// ponytail: core.companies has no slug column and nothing reads
// X-Company-Slug server-side yet. Add companies.slug + a lookup
// middleware when a second building actually exists.
type MPPConfig struct {
	CompanyID string
	LocalTZ   string
	// Location is the operating-day timezone. Storage stays UTC; this only
	// decides which calendar day a booking/queue number belongs to.
	Location *time.Location
}

type GCSConfig struct {
	BucketName      string
	ProjectID       string
	CredentialsJSON string
}

// FirebaseConfig holds credentials for the Firebase Admin SDK used to
// verify ID tokens coming from FE Google sign-in (and any future
// Firebase-backed providers).
//
// ProjectID is mandatory — VerifyIDToken refuses to validate without it.
// CredentialsJSON is the raw JSON content of a service-account key.
// Leave it empty in environments where Application Default Credentials
// are available (e.g. GKE workload identity) — the SDK will pick them
// up automatically.
type FirebaseConfig struct {
	ProjectID       string
	CredentialsJSON string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type ServerConfig struct {
	Port string
	Env  string
	// AllowedOrigins is the CORS allow-list. Browsers preflight any
	// request carrying a custom header (the FE sends X-Company-Slug on
	// every call), so an origin missing here fails as a 403 on OPTIONS
	// before the handler is ever reached. Override per deployment with
	// CORS_ALLOWED_ORIGINS rather than editing the router.
	AllowedOrigins []string
}

type SecurityConfig struct {
	EmailVerificationRequired bool
	MaxLoginAttempts          int
	AccountLockoutDuration    time.Duration
}

type AuthConfig struct {
	// DefaultAdminRoleID is the role assigned to a newly registered user
	// in their freshly created company. Maps to core.roles.id.
	DefaultAdminRoleID string
}

type WAHAConfig struct {
	BaseURL       string
	APIKey        string
	WebhookURL    string
	WebhookSecret string
	HTTPTimeout   int // HTTP timeout in seconds
}

type OpenAIConfig struct {
	APIKey  string
	Model   string
	Timeout int // API timeout in seconds
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
	// PermissionTTL controls how long a user's effective permission set is
	// cached before being re-fetched from the database. Short values favour
	// prompt permission-revoke propagation; long values favour DB load.
	PermissionTTL time.Duration
}

func Load() *Config {
	mppTZ := getEnv("MPP_LOCAL_TZ", "Asia/Jakarta")
	loc, err := time.LoadLocation(mppTZ)
	if err != nil {
		// Windows/scratch containers often ship without tzdata. WIB is the
		// default operating zone, so fall back to it rather than silently
		// running the whole queue domain in UTC.
		loc = time.FixedZone("WIB", 7*60*60)
	}

	return &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			DBName:   getEnv("DB_NAME", "tuai"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
			Env:  getEnv("ENV", "development"),
			AllowedOrigins: getEnvList("CORS_ALLOWED_ORIGINS", []string{
				// MPP frontend (apps/web dev server and production build).
				"http://localhost:8002",
				"http://127.0.0.1:8002",
				// Skeleton defaults, kept so existing clients keep working.
				"http://localhost:3000",
				"http://localhost:3001",
				"http://localhost:5173",
				"http://localhost:8081",
				"https://app.tuai.id",
				"https://jesuit.venturo.pro",
				"https://skeleton.venturo.id",
			}),
		},
		Security: SecurityConfig{
			EmailVerificationRequired: getEnvBool("EMAIL_VERIFICATION_REQUIRED", true),
			MaxLoginAttempts:          getEnvInt("MAX_LOGIN_ATTEMPTS", 5),
			AccountLockoutDuration:    getEnvDuration("ACCOUNT_LOCKOUT_DURATION", 30*time.Minute),
		},
		Auth: AuthConfig{
			DefaultAdminRoleID: getEnv("AUTH_DEFAULT_ADMIN_ROLE_ID", "00000000-0000-0000-0000-000000000002"),
		},
		WAHA: WAHAConfig{
			BaseURL:       getEnv("WAHA_BASE_URL", "https://wapi.venturo.id"),
			APIKey:        getEnv("WAHA_API_KEY", ""),
			WebhookURL:    getEnv("WAHA_WEBHOOK_URL", ""),
			WebhookSecret: getEnv("WAHA_WEBHOOK_SECRET", ""),
			HTTPTimeout:   getEnvInt("WAHA_HTTP_TIMEOUT", 30),
		},
		OpenAI: OpenAIConfig{
			APIKey:  getEnv("OPENAI_API_KEY", ""),
			Model:   getEnv("OPENAI_MODEL", "gpt-4o-mini"),
			Timeout: getEnvInt("OPENAI_TIMEOUT", 120),
		},
		Redis: RedisConfig{
			Host:          getEnv("REDIS_HOST", "localhost"),
			Port:          getEnv("REDIS_PORT", "6379"),
			Password:      getEnv("REDIS_PASSWORD", ""),
			DB:            getEnvInt("REDIS_DB", 10),
			PermissionTTL: getEnvDuration("REDIS_PERMISSION_TTL", 10*time.Minute),
		},
		GCS: GCSConfig{
			BucketName:      getEnv("GCS_BUCKET_NAME", ""),
			ProjectID:       getEnv("GCS_PROJECT_ID", ""),
			CredentialsJSON: getEnv("GCS_CREDENTIALS_JSON", ""),
		},
		Firebase: FirebaseConfig{
			ProjectID:       getEnv("FIREBASE_PROJECT_ID", ""),
			CredentialsJSON: getEnv("FIREBASE_CREDENTIALS_JSON", ""),
		},
		MPP: MPPConfig{
			CompanyID: getEnv("MPP_COMPANY_ID", "a1000000-0000-0000-0000-000000000001"),
			LocalTZ:   mppTZ,
			Location:  loc,
		},
	}
}

func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s&timezone=UTC",
		c.User, c.Password, c.Host, c.Port, c.DBName, c.SSLMode,
	)
}

func (c *DatabaseConfig) GetMigrationURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s&timezone=UTC",
		c.User, c.Password, c.Host, c.Port, c.DBName, c.SSLMode,
	)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvList reads a comma-separated env var, trimming blanks. An unset
// or empty value keeps the default rather than yielding an empty list —
// an empty CORS allow-list would reject every browser silently.
func getEnvList(key string, defaultValue []string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue
	}

	out := make([]string, 0, len(defaultValue))
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return defaultValue
	}

	return out
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return defaultValue
		}
		return boolValue
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return defaultValue
		}
		return intValue
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil {
			return defaultValue
		}
		return duration
	}
	return defaultValue
}