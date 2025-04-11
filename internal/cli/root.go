package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	cfgFile     string // Path to config file (passed via flag)
	registryURL string
	apiToken    string
	logLevel    string // Flag for log level
	logger      *zap.Logger
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "protoreg-cli",
	Short: "CLI client for the SProto Protobuf Registry",
	Long: `protoreg-cli is a command-line tool to interact with the SProto registry,
allowing you to publish and fetch Protobuf module artifacts.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize logger based on the log level flag
		initLogger(logLevel)
	},
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig) // Called after flags are parsed

	// Persistent flags available to all subcommands
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/protoreg/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&registryURL, "registry-url", "", "Registry server URL (overrides config/env)")
	rootCmd.PersistentFlags().StringVar(&apiToken, "api-token", "", "API token for authentication (overrides config/env)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Set logging level (debug, info, warn, error)")

	// Bind persistent flags to Viper
	viper.BindPFlag("registry_url", rootCmd.PersistentFlags().Lookup("registry-url"))
	viper.BindPFlag("api_token", rootCmd.PersistentFlags().Lookup("api-token"))
	// Note: We don't bind cfgFile or logLevel to viper directly, they control viper/logger setup.
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		cobra.CheckErr(err)

		// Search config in ~/.config/protoreg directory with name "config" (without extension).
		configPath := filepath.Join(home, ".config", "protoreg")
		viper.AddConfigPath(configPath)
		viper.SetConfigName("config") // Name of config file (without extension)
		viper.SetConfigType("yaml")   // REQUIRED if the config file does not have the extension in the name
	}

	// Read environment variables with PROTOREG_ prefix
	viper.SetEnvPrefix("PROTOREG")
	viper.AutomaticEnv()                                   // read in environment variables that match
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_")) // e.g. PROTOREG_REGISTRY_URL

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	} else {
		// Only show file not found error if a specific file was requested
		if cfgFile != "" || !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "Error reading config file:", err)
		}
	}

	// --- Get final config values (Precedence: Flag > Env > Config File > Default) ---
	// Viper automatically handles precedence for bound flags and env vars.
	// We retrieve them here just to potentially log or use them during init if needed.
	// The actual values used by commands will be retrieved via viper.GetString() etc.
	finalRegistryURL := viper.GetString("registry_url") // Key matches viper.BindPFlag or env var
	// finalApiToken := viper.GetString("api_token") // Commented out as unused for now

	// Set defaults if nothing else provided them
	if finalRegistryURL == "" {
		viper.SetDefault("registry_url", "http://localhost:8080") // Default from server config
	}
	// No default for API token - it should be explicitly provided for commands needing it.

	// Log final effective settings (optional, consider logging level)
	// logger.Debug("Effective Registry URL", zap.String("url", viper.GetString("registry_url")))
	// logger.Debug("API Token Provided", zap.Bool("set", viper.GetString("api_token") != ""))
}

// initLogger initializes the Zap logger based on the desired level.
func initLogger(level string) {
	var zapLevel zapcore.Level
	switch strings.ToLower(level) {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn", "warning":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel // Default to info
	}

	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(zapLevel),
		Development: false,     // Set to true for more verbose, human-friendly output
		Encoding:    "console", // or "json"
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder, // Capitalize level names (INFO, WARN)
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	var err error
	logger, err = config.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing logger: %v\n", err)
		os.Exit(1)
	}
	// Replace global logger (optional, but convenient)
	// zap.ReplaceGlobals(logger)
}

// GetLogger returns the initialized logger instance.
func GetLogger() *zap.Logger {
	if logger == nil {
		// Initialize with default level if not already done (shouldn't happen with PersistentPreRun)
		initLogger("info")
	}
	return logger
}
