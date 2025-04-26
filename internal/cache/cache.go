// Package cache provides a local caching mechanism for SProto module artifacts.
package cache

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Standard cache locations by OS
const (
	DefaultCacheDirName = "sproto" // Base directory name within user cache
)

// Cache subdirectories
const (
	ModulesDir   = "modules"  // Stores module artifacts and extracted files
	MetadataDir  = "metadata" // Stores cache metadata
	ArtifactFile = "artifact.zip"
	ConfigFile   = "sproto.yaml"
	ExtractedDir = "extracted" // Stores extracted proto files
)

// LockFileName is the name of the lock file used to prevent concurrent cache operations
const LockFileName = ".cache.lock"

// Cache errors
var (
	ErrCacheDir       = errors.New("failed to determine cache directory")
	ErrCacheInit      = errors.New("failed to initialize cache directory")
	ErrModuleNotFound = errors.New("module not found in cache")
	ErrLockFailed     = errors.New("failed to acquire cache lock")
)

// Cache represents the local cache for SProto modules.
type Cache struct {
	RootDir  string     // Root directory for the cache
	mu       sync.Mutex // Mutex for operations that require synchronization
	lockFile *os.File   // File handle for the lock file
}

// CacheConfig holds configuration options for the cache.
type CacheConfig struct {
	// Custom root directory, if empty, the default will be used
	CustomRootDir string
	// Whether to clean the cache on initialization
	CleanOnInit bool
}

// NewCache initializes a new cache with the default location.
func NewCache() (*Cache, error) {
	return NewCacheWithConfig(CacheConfig{})
}

// NewCacheWithConfig initializes a new cache with custom configuration.
func NewCacheWithConfig(config CacheConfig) (*Cache, error) {
	var rootDir string

	if config.CustomRootDir != "" {
		rootDir = config.CustomRootDir
	} else {
		// Determine cache directory based on OS
		userCacheDir, err := os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCacheDir, err)
		}
		rootDir = filepath.Join(userCacheDir, DefaultCacheDirName)
	}

	// Ensure the root directory exists
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCacheInit, err)
	}

	// Ensure the modules directory exists
	modulesDir := filepath.Join(rootDir, ModulesDir)
	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCacheInit, err)
	}

	// Ensure the metadata directory exists
	metadataDir := filepath.Join(rootDir, MetadataDir)
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCacheInit, err)
	}

	cache := &Cache{
		RootDir: rootDir,
	}

	// Clean the cache if requested
	if config.CleanOnInit {
		if err := cache.Clean(); err != nil {
			return nil, fmt.Errorf("failed to clean cache on init: %w", err)
		}
	}

	return cache, nil
}

// GetModulePath returns the path to a module's directory in the cache.
func (c *Cache) GetModulePath(namespace, name, version string) string {
	return filepath.Join(c.RootDir, ModulesDir, namespace, name, version)
}

// GetArtifactPath returns the path to a cached artifact zip.
// The bool return value indicates whether the artifact exists in the cache.
func (c *Cache) GetArtifactPath(namespace, name, version string) (string, bool, error) {
	modulePath := c.GetModulePath(namespace, name, version)
	artifactPath := filepath.Join(modulePath, ArtifactFile)

	// Check if the artifact exists
	if _, err := os.Stat(artifactPath); err != nil {
		if os.IsNotExist(err) {
			return artifactPath, false, nil
		}
		return "", false, err
	}

	return artifactPath, true, nil
}

// GetExtractedPath returns the path to the extracted files directory.
// The bool return value indicates whether the extracted directory exists.
func (c *Cache) GetExtractedPath(namespace, name, version string) (string, bool, error) {
	modulePath := c.GetModulePath(namespace, name, version)
	extractedPath := filepath.Join(modulePath, ExtractedDir)

	// Check if the extracted directory exists
	if _, err := os.Stat(extractedPath); err != nil {
		if os.IsNotExist(err) {
			return extractedPath, false, nil
		}
		return "", false, err
	}

	return extractedPath, true, nil
}

// Lock acquires a file lock to synchronize cache operations.
// It returns an error if the lock cannot be acquired.
func (c *Cache) Lock() error {
	c.mu.Lock()

	// If already locked, return success
	if c.lockFile != nil {
		return nil
	}

	lockPath := filepath.Join(c.RootDir, LockFileName)

	// Create lock file with exclusive flag
	var err error
	flags := os.O_CREATE | os.O_WRONLY

	// Use platform-specific flags for file locking if available
	if runtime.GOOS != "windows" {
		// Most Unix-like systems
		flags |= os.O_EXCL
	}

	c.lockFile, err = os.OpenFile(lockPath, flags, 0644)
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("%w: %v", ErrLockFailed, err)
	}

	// On Windows, attempt file locking differently
	// This is simplified - a real implementation would use LockFileEx on Windows

	return nil
}

// Unlock releases the file lock.
func (c *Cache) Unlock() error {
	if c.lockFile != nil {
		err := c.lockFile.Close()
		c.lockFile = nil

		// Try to remove the lock file, but don't fail if we can't
		lockPath := filepath.Join(c.RootDir, LockFileName)
		_ = os.Remove(lockPath)

		c.mu.Unlock()
		return err
	}

	c.mu.Unlock()
	return nil
}

// Clean performs cache cleanup operations.
// This is a placeholder and should be expanded with actual cleanup logic.
func (c *Cache) Clean() error {
	// Lock the cache during cleaning
	if err := c.Lock(); err != nil {
		return err
	}
	defer c.Unlock()

	// Placeholder for cleanup logic
	// Ideas:
	// - Remove old unused modules
	// - Apply size limits
	// - Remove corrupted artifacts
	// - Remove empty directories

	return nil
}

/*
Cache Directory Structure:

~/.cache/sproto/                           # Root cache directory (OS-specific)
├── modules/                               # Module artifacts storage
│   ├── <namespace>/                       # Module namespace
│   │   ├── <module_name>/                 # Module name
│   │   │   ├── <version>/                 # Module version
│   │   │   │   ├── artifact.zip           # Original downloaded artifact
│   │   │   │   ├── sproto.yaml            # Extracted config (if present)
│   │   │   │   └── extracted/             # Extracted proto files
│   │   │   │       └── <import_path>/     # Organized by import path
│   │   │   │           └── ...
│   │   │   └── ...                        # Other versions
│   │   └── ...                            # Other modules in namespace
│   └── ...                                # Other namespaces
├── metadata/                              # Cache metadata storage
│   ├── index.json                         # Index of cached modules/versions
│   └── stats.json                         # Cache usage statistics
└── .cache.lock                            # Lock file for synchronization

*/
