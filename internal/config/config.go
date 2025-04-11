package config

import (
	"github.com/spf13/viper"
)

// Config holds all configuration for the application (server and potentially CLI).
type Config struct {
	// Server specific configuration
	ServerPort string `mapstructure:"SERVER_PORT"`

	// Database configuration
	DbDsn string `mapstructure:"DB_DSN"` // Data Source Name (e.g., "host=localhost user=user password=pass dbname=sproto port=5432 sslmode=disable")

	// MinIO configuration
	MinioEndpoint  string `mapstructure:"MINIO_ENDPOINT"`
	MinioAccessKey string `mapstructure:"MINIO_ACCESS_KEY"`
	MinioSecretKey string `mapstructure:"MINIO_SECRET_KEY"`
	MinioBucket    string `mapstructure:"MINIO_BUCKET"`
	MinioUseSSL    bool   `mapstructure:"MINIO_USE_SSL"`

	// Authentication
	AuthToken string `mapstructure:"AUTH_TOKEN"` // Static bearer token for publish operations

	// CLI specific configuration (can also be loaded by CLI)
	RegistryURL string `mapstructure:"REGISTRY_URL"` // URL for the CLI to connect to
}

// LoadConfig loads configuration from environment variables and sets defaults.
func LoadConfig() (config Config, err error) {
	// Set default values
	viper.SetDefault("SERVER_PORT", "8080")
	viper.SetDefault("DB_DSN", "host=localhost user=postgres password=postgres dbname=sproto port=5432 sslmode=disable")
	viper.SetDefault("MINIO_ENDPOINT", "localhost:9000")
	viper.SetDefault("MINIO_ACCESS_KEY", "minioadmin")
	viper.SetDefault("MINIO_SECRET_KEY", "minioadmin")
	viper.SetDefault("MINIO_BUCKET", "sproto-artifacts")
	viper.SetDefault("MINIO_USE_SSL", false)
	viper.SetDefault("AUTH_TOKEN", "supersecrettoken") // CHANGE THIS IN PRODUCTION
	viper.SetDefault("REGISTRY_URL", "http://localhost:8080")

	// Tell viper to look for environment variables with a specific prefix
	viper.SetEnvPrefix("PROTOREG") // e.g., PROTOREG_SERVER_PORT, PROTOREG_DB_DSN
	viper.AutomaticEnv()           // Read in environment variables that match

	// Replace dots with underscores for environment variable compatibility if needed
	// viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_")) // Not strictly needed with explicit mapstructure tags

	// Unmarshal the configuration into the struct
	err = viper.Unmarshal(&config)
	return
}

// Note: For CLI configuration, we might want a separate LoadCliConfig function
// or enhance this one to also check flags and config files (~/.config/protoreg/config.yaml)
// as specified in the original requirements. This initial version focuses on server needs via env vars.
