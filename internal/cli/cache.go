package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/Suhaibinator/SProto/internal/cache"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	cachePath      string
	cacheOutputFmt string
)

// cacheCmd represents the cache command
var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the local cache of module artifacts",
	Long: `Manage the local cache of module artifacts.

The 'cache' command provides a set of subcommands for working with the local cache, including
listing cached modules, finding paths to specific modules, cleaning the cache, invalidating
cache entries, and reporting cache statistics.

Examples:
  # List all modules in the cache
  protoreg-cli cache list

  # Print the path to a specific module in the cache
  protoreg-cli cache path myorg/common@v1.0.0

  # Clean the cache (remove unused artifacts)
  protoreg-cli cache clean

  # Remove a specific module from the cache
  protoreg-cli cache invalidate myorg/common@v1.0.0

  # Show the total size of the cache
  protoreg-cli cache size`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default behavior when no subcommand is specified - show help
		cmd.Help()
	},
}

// cacheListCmd represents the cache list subcommand
var cacheListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all modules in the cache",
	Long: `List all modules currently stored in the local cache.

This command displays a list of all cached modules in the format:
namespace/module_name@version

Examples:
  # List all modules in the cache
  protoreg-cli cache list

  # List all modules in the cache with detailed format
  protoreg-cli cache list --format detailed`,
	Run: func(cmd *cobra.Command, args []string) {
		log := GetLogger()

		// Initialize the cache
		c, err := cache.NewCache()
		if err != nil {
			log.Fatal("Failed to initialize cache", zap.Error(err))
		}

		// List all modules in the cache
		modules, err := c.ListModules()
		if err != nil {
			log.Fatal("Failed to list cached modules", zap.Error(err))
		}

		if len(modules) == 0 {
			fmt.Println("No modules found in cache.")
			return
		}

		fmt.Printf("Found %d modules in cache:\n\n", len(modules))
		for _, module := range modules {
			fmt.Println(module)
		}
	},
}

// cachePathCmd represents the cache path subcommand
var cachePathCmd = &cobra.Command{
	Use:   "path <module_ref@version>",
	Short: "Print the filesystem path to a cached module",
	Long: `Print the filesystem path to a cached module.

Specify a module reference in the format 'namespace/module_name@version' to get the path
to that module's artifact or extracted files in the cache.

Examples:
  # Get path to a module's artifact.zip
  protoreg-cli cache path myorg/common@v1.0.0

  # Get path to a module's extracted files
  protoreg-cli cache path myorg/common@v1.0.0 --extracted`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log := GetLogger()

		// Parse module reference
		moduleRef := args[0]
		parts := strings.Split(moduleRef, "@")
		if len(parts) != 2 {
			log.Fatal("Invalid module reference format. Expected 'namespace/module_name@version'.")
		}

		moduleName := parts[0]
		version := parts[1]

		moduleNameParts := strings.Split(moduleName, "/")
		if len(moduleNameParts) != 2 {
			log.Fatal("Invalid module name format. Expected 'namespace/name'.")
		}

		namespace := moduleNameParts[0]
		name := moduleNameParts[1]

		// Initialize the cache
		c, err := cache.NewCache()
		if err != nil {
			log.Fatal("Failed to initialize cache", zap.Error(err))
		}

		// Get the path based on the flag
		var path string
		var exists bool
		if cachePath == "extracted" {
			path, exists, err = c.GetExtractedPath(namespace, name, version)
			if err != nil {
				log.Fatal("Failed to get extracted path", zap.Error(err))
			}
			if !exists {
				log.Fatal("Extracted files not found for module",
					zap.String("module", moduleRef))
			}
		} else {
			path, exists, err = c.GetArtifactPath(namespace, name, version)
			if err != nil {
				log.Fatal("Failed to get artifact path", zap.Error(err))
			}
			if !exists {
				log.Fatal("Module artifact not found in cache",
					zap.String("module", moduleRef))
			}
		}

		// Print the path
		fmt.Println(path)
	},
}

// cacheCleanCmd represents the cache clean subcommand
var cacheCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean the cache",
	Long: `Clean the cache by removing old or unused artifacts.

This command invokes the cache cleaning process, which may remove:
- Old versions of modules that haven't been accessed recently
- Corrupted artifacts
- Empty directories

Examples:
  protoreg-cli cache clean`,
	Run: func(cmd *cobra.Command, args []string) {
		log := GetLogger()

		// Initialize the cache
		c, err := cache.NewCache()
		if err != nil {
			log.Fatal("Failed to initialize cache", zap.Error(err))
		}

		// Get cache size before cleaning
		sizeBefore, err := c.GetCacheSize()
		if err != nil {
			log.Fatal("Failed to get cache size", zap.Error(err))
		}

		fmt.Printf("Cache size before cleaning: %.2f MB\n", float64(sizeBefore)/(1024*1024))

		// Clean the cache
		err = c.Clean()
		if err != nil {
			log.Fatal("Failed to clean cache", zap.Error(err))
		}

		// Get cache size after cleaning
		sizeAfter, err := c.GetCacheSize()
		if err != nil {
			log.Fatal("Failed to get cache size", zap.Error(err))
		}

		fmt.Printf("Cache size after cleaning: %.2f MB\n", float64(sizeAfter)/(1024*1024))
		fmt.Printf("Space freed: %.2f MB\n", float64(sizeBefore-sizeAfter)/(1024*1024))
		fmt.Println("Cache cleaned successfully.")
	},
}

// cacheInvalidateCmd represents the cache invalidate subcommand
var cacheInvalidateCmd = &cobra.Command{
	Use:   "invalidate <module_ref[@version]>",
	Short: "Remove a module from the cache",
	Long: `Remove a specific module or module version from the cache.

Specify a module reference in the format 'namespace/module_name[@version]' to invalidate
that module in the cache. If version is omitted, all versions of the module will be removed.

Examples:
  # Remove a specific version of a module from the cache
  protoreg-cli cache invalidate myorg/common@v1.0.0

  # Remove all versions of a module from the cache
  protoreg-cli cache invalidate myorg/common`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log := GetLogger()

		// Parse module reference
		moduleRef := args[0]
		var namespace, name, version string

		// Check if version is specified
		if strings.Contains(moduleRef, "@") {
			parts := strings.Split(moduleRef, "@")
			if len(parts) != 2 {
				log.Fatal("Invalid module reference format. Expected 'namespace/module_name[@version]'.")
			}

			moduleName := parts[0]
			version = parts[1]

			moduleNameParts := strings.Split(moduleName, "/")
			if len(moduleNameParts) != 2 {
				log.Fatal("Invalid module name format. Expected 'namespace/name'.")
			}

			namespace = moduleNameParts[0]
			name = moduleNameParts[1]
		} else {
			moduleNameParts := strings.Split(moduleRef, "/")
			if len(moduleNameParts) != 2 {
				log.Fatal("Invalid module name format. Expected 'namespace/name'.")
			}

			namespace = moduleNameParts[0]
			name = moduleNameParts[1]
		}

		// Initialize the cache
		c, err := cache.NewCache()
		if err != nil {
			log.Fatal("Failed to initialize cache", zap.Error(err))
		}

		// Invalidate the module or version
		err = c.Invalidate(namespace, name, version)
		if err != nil {
			log.Fatal("Failed to invalidate module", zap.Error(err))
		}

		if version == "" {
			fmt.Printf("Successfully removed all versions of module %s/%s from cache.\n", namespace, name)
		} else {
			fmt.Printf("Successfully removed module %s/%s@%s from cache.\n", namespace, name, version)
		}
	},
}

// cacheSizeCmd represents the cache size subcommand
var cacheSizeCmd = &cobra.Command{
	Use:   "size",
	Short: "Report the total size of the cache",
	Long: `Calculate and report the total disk space used by the cache.

This command walks the cache directory and calculates the total size of all files.

Examples:
  protoreg-cli cache size`,
	Run: func(cmd *cobra.Command, args []string) {
		log := GetLogger()

		// Initialize the cache
		c, err := cache.NewCache()
		if err != nil {
			log.Fatal("Failed to initialize cache", zap.Error(err))
		}

		// Get cache size
		size, err := c.GetCacheSize()
		if err != nil {
			log.Fatal("Failed to calculate cache size", zap.Error(err))
		}

		// Print size in human-readable format
		if size < 1024 {
			fmt.Printf("Cache size: %d bytes\n", size)
		} else if size < 1024*1024 {
			fmt.Printf("Cache size: %.2f KB\n", float64(size)/1024)
		} else if size < 1024*1024*1024 {
			fmt.Printf("Cache size: %.2f MB\n", float64(size)/(1024*1024))
		} else {
			fmt.Printf("Cache size: %.2f GB\n", float64(size)/(1024*1024*1024))
		}

		// If we have the cache root dir, print it too
		rootDir := c.RootDir
		if rootDir != "" {
			fmt.Printf("Cache location: %s\n", rootDir)

			// Check if the cache directory exists
			if _, err := os.Stat(rootDir); os.IsNotExist(err) {
				fmt.Println("Note: Cache directory does not exist yet.")
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(cacheCmd)

	// Add subcommands
	cacheCmd.AddCommand(cacheListCmd, cachePathCmd, cacheCleanCmd, cacheInvalidateCmd, cacheSizeCmd)

	// Add flags to subcommands
	cachePathCmd.Flags().StringVar(&cachePath, "path-type", "artifact", "Type of path to return (artifact or extracted)")

	// Format flag for list command (for future enhancement)
	cacheListCmd.Flags().StringVar(&cacheOutputFmt, "format", "simple", "Output format (simple, detailed)")
}
