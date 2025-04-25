package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Regular expressions for validation
var (
	// Allows letters, numbers, underscores, hyphens in namespace and name
	moduleNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+$`)
	// Basic check for Go import path characters (allows letters, numbers, underscore, hyphen, dot, slash)
	// A more sophisticated validation might be needed for edge cases.
	importPathRegex = regexp.MustCompile(`^[a-zA-Z0-9_./-]+$`)
	// Allows letters, numbers, underscores, hyphens for namespace/name parts
	dependencyNamePartRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// Config holds all configuration for the application (server and potentially CLI).
type Config struct {
	// Server specific configuration
	ServerPort string `mapstructure:"SERVER_PORT"`

	// Database configuration
	DbType     string `mapstructure:"DB_TYPE"`     // "postgres" or "sqlite"
	DbDsn      string `mapstructure:"DB_DSN"`      // Data Source Name for Postgres
	SqlitePath string `mapstructure:"SQLITE_PATH"` // Path for SQLite database file

	// Storage configuration
	StorageType      string `mapstructure:"STORAGE_TYPE"`       // "minio" or "local"
	LocalStoragePath string `mapstructure:"LOCAL_STORAGE_PATH"` // Path for local file storage

	// MinIO specific configuration (only used if StorageType is "minio")
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
	viper.SetDefault("DB_TYPE", "postgres") // Default to postgres
	viper.SetDefault("DB_DSN", "host=localhost user=postgres password=postgres dbname=sproto port=5432 sslmode=disable")
	viper.SetDefault("SQLITE_PATH", "sproto.db")               // Default SQLite path
	viper.SetDefault("STORAGE_TYPE", "minio")                  // Default to minio
	viper.SetDefault("LOCAL_STORAGE_PATH", "./sproto-storage") // Default local storage path
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

// --- SProto Module Configuration (`sproto.yaml`) ---

// SProtoConfig represents the structure of the sproto.yaml file.
type SProtoConfig struct {
	Version      string       `yaml:"version"`
	Name         string       `yaml:"name"` // Format: "namespace/name"
	ImportPath   string       `yaml:"import_path"`
	Dependencies []Dependency `yaml:"dependencies,omitempty"`
	Generate     []Generate   `yaml:"generate,omitempty"` // Added generation configurations
}

// Dependency represents a single dependency listed in sproto.yaml.
type Dependency struct {
	Namespace  string `yaml:"namespace"`
	Name       string `yaml:"name"`
	Version    string `yaml:"version"` // Version constraint string
	ImportPath string `yaml:"import_path"`
}

// ParseConfigBytes parses SProto configuration from a byte slice.
func ParseConfigBytes(data []byte) (*SProtoConfig, error) {
	var config SProtoConfig
	err := yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal sproto config: %w", err)
	}
	// Validate the parsed configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid sproto config: %w", err)
	}
	return &config, nil
}

// ParseConfig reads and parses SProto configuration from a file path.
func ParseConfig(configPath string) (*SProtoConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read sproto config file '%s': %w", configPath, err)
	}
	return ParseConfigBytes(data)
}

// FullName returns the combined namespace and name from the config.
func (c *SProtoConfig) FullName() string {
	return c.Name // Assumes Name is already in "namespace/name" format
}

// GetDependencyByName searches for a dependency by its module name (namespace/name).
func (c *SProtoConfig) GetDependencyByName(fullName string) (*Dependency, bool) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 {
		return nil, false // Invalid format
	}
	ns, name := parts[0], parts[1]

	for i := range c.Dependencies {
		dep := &c.Dependencies[i] // Use pointer to avoid copying
		if dep.Namespace == ns && dep.Name == name {
			return dep, true
		}
	}
	return nil, false
}

// Validate checks the SProtoConfig for correctness according to the specification.
func (c *SProtoConfig) Validate() error {
	var errs []string

	// 1. Check Version
	if c.Version != "v1" {
		errs = append(errs, fmt.Sprintf("unsupported configuration version '%s', expected 'v1'", c.Version))
	}

	// 2. Check Name Format
	if !moduleNameRegex.MatchString(c.Name) {
		errs = append(errs, fmt.Sprintf("invalid module name format '%s', expected 'namespace/name' using letters, numbers, underscores, hyphens", c.Name))
	}

	// 3. Check Import Path Format
	if !importPathRegex.MatchString(c.ImportPath) {
		errs = append(errs, fmt.Sprintf("invalid import path format '%s'", c.ImportPath))
	}

	// 4. Check Dependencies
	depNames := make(map[string]struct{}) // For checking duplicates
	for i, dep := range c.Dependencies {
		depIdentifier := fmt.Sprintf("%s/%s", dep.Namespace, dep.Name)
		depIndexStr := fmt.Sprintf("dependency #%d (%s)", i+1, depIdentifier)

		// Check Namespace format
		if !dependencyNamePartRegex.MatchString(dep.Namespace) {
			errs = append(errs, fmt.Sprintf("%s: invalid namespace format '%s'", depIndexStr, dep.Namespace))
		}
		// Check Name format
		if !dependencyNamePartRegex.MatchString(dep.Name) {
			errs = append(errs, fmt.Sprintf("%s: invalid name format '%s'", depIndexStr, dep.Name))
		}

		// Check Version Constraint Syntax
		_, err := semver.NewConstraint(dep.Version)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: invalid version constraint '%s': %v", depIndexStr, dep.Version, err))
		}

		// Check Import Path Format
		if !importPathRegex.MatchString(dep.ImportPath) {
			errs = append(errs, fmt.Sprintf("%s: invalid import path format '%s'", depIndexStr, dep.ImportPath))
		}

		// Check for Duplicate Dependencies (by namespace/name)
		if _, exists := depNames[depIdentifier]; exists {
			errs = append(errs, fmt.Sprintf("duplicate dependency detected: '%s'", depIdentifier))
		}
		depNames[depIdentifier] = struct{}{}
	}

	// Note: Circular dependency checks are handled implicitly by the DAG library when adding edges.
	if len(errs) > 0 {
		return errors.New("validation failed:\n - " + strings.Join(errs, "\n - "))
	}

	return nil
}

// Note: Circular dependency checks are more complex and might involve building a graph.
// This basic validation focuses on format and syntax.

// Generate represents a single generation template in sproto.yaml.
type Generate struct {
	Name    string            `yaml:"name"`    // Name of the template (e.g., "go", "grpc-gateway")
	Output  string            `yaml:"output"`  // Output directory for generated files
	Options map[string]string `yaml:"options"` // Options passed to protoc (e.g., --go_out, --go_opt)
	Plugins []string          `yaml:"plugins"` // Protoc plugins to use
}
