package cli

import (
	"fmt"
	"os"

	"github.com/Suhaibinator/SProto/internal/config"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// ValidateConfigCmd returns the cobra command for validating sproto.yaml files.
func ValidateConfigCmd(logger *zap.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [path/to/sproto.yaml]",
		Short: "Validate a sproto.yaml configuration file",
		Long: `Reads and validates a sproto.yaml file based on the SProto v1 specification.
Checks for correct format, required fields, valid names, import paths, and dependency syntax.
If no path is provided, it defaults to './sproto.yaml'.`,
		Args: cobra.MaximumNArgs(1), // Allow zero or one argument (the path)
		Run: func(cmd *cobra.Command, args []string) {
			configPath := "sproto.yaml" // Default path
			if len(args) > 0 {
				configPath = args[0]
			}

			logger.Info("Validating configuration file", zap.String("path", configPath))

			// Check if file exists before attempting to parse
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				logger.Error("Configuration file not found", zap.String("path", configPath), zap.Error(err))
				fmt.Fprintf(os.Stderr, "Error: Configuration file not found at %s\n", configPath)
				os.Exit(1)
			}

			// ParseConfig now includes validation
			_, err := config.ParseConfig(configPath)
			if err != nil {
				logger.Error("Configuration validation failed", zap.String("path", configPath), zap.Error(err))
				// Print user-friendly error message to stderr
				fmt.Fprintf(os.Stderr, "Validation failed for %s:\n%v\n", configPath, err)
				os.Exit(1) // Indicate failure
			}

			logger.Info("Configuration validation successful", zap.String("path", configPath))
			fmt.Printf("Validation successful for %s\n", configPath)
		},
	}
	return cmd
}
