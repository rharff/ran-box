package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort    string
	AppEnv     string

	JWTSecret      string
	JWTExpiryHours int

	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string
	DBSSLMode  string

	S3Endpoint       string
	S3Bucket         string
	S3AccessKey      string
	S3SecretKey      string
	S3Region         string
	S3ForcePathStyle bool

	BlockSizeMB int
}

// DSN returns the PostgreSQL connection string.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBName, c.DBUser, c.DBPassword, c.DBSSLMode,
	)
}

// BlockSizeBytes returns block size in bytes.
func (c *Config) BlockSizeBytes() int {
	return c.BlockSizeMB * 1024 * 1024
}

// Load reads .env (if present) then environment variables.
func Load() (*Config, error) {
	// Best-effort: load .env file, ignore error if not found
	_ = godotenv.Load()

	cfg := &Config{
		AppPort:    getEnv("APP_PORT", "8080"),
		AppEnv:     getEnv("APP_ENV", "development"),

		JWTSecret:      mustGetEnv("JWT_SECRET"),
		JWTExpiryHours: getEnvInt("JWT_EXPIRY_HOURS", 24),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBName:     getEnv("DB_NAME", "naratel_box"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		S3Endpoint:       mustGetEnv("S3_ENDPOINT"),
		S3Bucket:         mustGetEnv("S3_BUCKET"),
		S3AccessKey:      mustGetEnv("S3_ACCESS_KEY"),
		S3SecretKey:      mustGetEnv("S3_SECRET_KEY"),
		S3Region:         getEnv("S3_REGION", "us-east-1"),
		S3ForcePathStyle: getEnvBool("S3_FORCE_PATH_STYLE", true),

		BlockSizeMB: getEnvInt("BLOCK_SIZE_MB", 8),
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %q is not set", key))
	}
	return v
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
