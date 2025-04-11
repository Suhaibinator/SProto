package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	configureRegistryURL string
	configureApiToken    string
)

// configureCmd represents the configure command
var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure registry URL and API token",
	Long: `Saves the SProto registry server URL and API token to the configuration file.
Configuration is stored in ~/.config/protoreg/config.yaml by default.

Precedence order for configuration values:
1. Command-line flags (--registry-url, --api-token)
2. Environment variables (PROTOREG_REGISTRY_URL, PROTOREG_API_TOKEN)
3. Configuration file (~/.config/protoreg/config.yaml)
4. Default values

This command updates the configuration file directly.`,
	Run: func(cmd *cobra.Command, args []string) {
		log := GetLogger()

		// Check if at least one flag was provided
		urlFlagSet := cmd.Flags().Changed("registry-url")
		tokenFlagSet := cmd.Flags().Changed("api-token")

		if !urlFlagSet && !tokenFlagSet {
			log.Error("At least one flag (--registry-url or --api-token) must be provided")
			cmd.Usage() // Show usage information
			os.Exit(1)
		}

		// Determine config file path
		var configFilePath string
		if cfgFile != "" {
			configFilePath = cfgFile
		} else {
			home, err := homedir.Dir()
			if err != nil {
				log.Fatal("Failed to get home directory", zap.Error(err))
			}
			configFilePath = filepath.Join(home, ".config", "protoreg", "config.yaml")
		}
		configDir := filepath.Dir(configFilePath)

		// Ensure config directory exists
		if err := os.MkdirAll(configDir, 0750); err != nil { // Use 0750 for permissions
			log.Fatal("Failed to create config directory", zap.String("path", configDir), zap.Error(err))
		}

		// Update viper settings based on flags
		if urlFlagSet {
			viper.Set("registry_url", configureRegistryURL)
			log.Info("Setting registry_url in config", zap.String("value", configureRegistryURL))
		}
		if tokenFlagSet {
			viper.Set("api_token", configureApiToken)
			log.Info("Setting api_token in config") // Don't log the token itself
		}

		// Write the config file
		log.Info("Writing configuration", zap.String("path", configFilePath))
		err := viper.WriteConfigAs(configFilePath)
		if err != nil {
			// Handle case where config file doesn't exist yet
			if os.IsNotExist(err) {
				err = viper.SafeWriteConfigAs(configFilePath) // Attempt safe write first
				if err != nil {
					log.Fatal("Failed to write new config file", zap.String("path", configFilePath), zap.Error(err))
				}
			} else {
				log.Fatal("Failed to write config file", zap.String("path", configFilePath), zap.Error(err))
			}
		}

		fmt.Printf("Configuration successfully saved to %s\n", configFilePath)
	},
}

func init() {
	rootCmd.AddCommand(configureCmd)

	// Flags specific to the configure command
	configureCmd.Flags().StringVar(&configureRegistryURL, "registry-url", "", "Registry server URL to save")
	configureCmd.Flags().StringVar(&configureApiToken, "api-token", "", "API token to save")

	// We don't mark them as required here because the Run function checks if at least one is set.
}
