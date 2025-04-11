package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/Suhaibinator/SProto/internal/config"
)

// LocalStorage implements the StorageProvider interface using the local filesystem.
type LocalStorage struct {
	basePath string
}

// NewLocalStorage creates and initializes a new LocalStorage provider.
// It ensures the base storage directory exists.
func NewLocalStorage(cfg config.Config) (*LocalStorage, error) {
	basePath := cfg.LocalStoragePath
	if basePath == "" {
		return nil, fmt.Errorf("local storage path cannot be empty")
	}

	// Ensure the base directory exists
	err := os.MkdirAll(basePath, 0755) // rwxr-xr-x permissions
	if err != nil {
		log.Printf("Failed to create local storage directory '%s': %v", basePath, err)
		return nil, fmt.Errorf("failed to create local storage directory: %w", err)
	}

	log.Printf("Local storage initialized at path: %s", basePath)

	return &LocalStorage{
		basePath: basePath,
	}, nil
}

// getFullPath resolves the absolute path for a given object name within the storage base path.
// It also ensures the necessary subdirectories are created.
func (l *LocalStorage) getFullPath(objectName string) (string, error) {
	// Clean the objectName to prevent path traversal issues (e.g., "../..")
	// Note: filepath.Join also helps clean paths.
	cleanObjectName := filepath.Clean(objectName)
	if cleanObjectName == "." || cleanObjectName == "/" || cleanObjectName == "" {
		return "", fmt.Errorf("invalid object name: %s", objectName)
	}
	// Prevent absolute paths in objectName
	if filepath.IsAbs(cleanObjectName) {
		return "", fmt.Errorf("object name cannot be an absolute path: %s", objectName)
	}

	fullPath := filepath.Join(l.basePath, cleanObjectName)

	// Ensure the directory for the file exists
	dir := filepath.Dir(fullPath)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create directory structure for %s: %w", fullPath, err)
	}

	return fullPath, nil
}

// UploadFile saves data to the local filesystem.
func (l *LocalStorage) UploadFile(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error {
	// Note: size and contentType are ignored in this basic local implementation,
	// but kept for interface compatibility.
	fullPath, err := l.getFullPath(objectName)
	if err != nil {
		return err
	}

	// Create the destination file
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create local file %s: %w", fullPath, err)
	}
	defer file.Close() // Ensure file is closed

	// Copy data from reader to file
	_, err = io.Copy(file, reader)
	if err != nil {
		// Attempt to remove partially written file on error
		_ = os.Remove(fullPath)
		return fmt.Errorf("failed to write data to local file %s: %w", fullPath, err)
	}

	return nil
}

// DownloadFile retrieves a file from the local filesystem.
func (l *LocalStorage) DownloadFile(ctx context.Context, objectName string) (io.ReadCloser, error) {
	fullPath, err := l.getFullPath(objectName)
	if err != nil {
		// If getFullPath failed because the object name was invalid, return that error.
		// We don't expect MkdirAll to fail here if the path is valid but file doesn't exist.
		return nil, err
	}

	// Check if file exists before opening (getFullPath only ensures directory)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("object %s not found locally: %w", objectName, os.ErrNotExist)
	} else if err != nil {
		return nil, fmt.Errorf("failed to stat local file %s: %w", fullPath, err)
	}

	file, err := os.Open(fullPath)
	if err != nil {
		// This check might be redundant given the Stat check above, but handles race conditions or permission issues.
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("object %s not found locally: %w", objectName, err)
		}
		return nil, fmt.Errorf("failed to open local file %s: %w", fullPath, err)
	}

	// Caller is responsible for closing the file.
	return file, nil
}

// DeleteFile removes a file from the local filesystem.
func (l *LocalStorage) DeleteFile(ctx context.Context, objectName string) error {
	fullPath, err := l.getFullPath(objectName)
	if err != nil {
		// If the path is invalid, we can't delete it.
		return err
	}

	err = os.Remove(fullPath)
	if err != nil {
		// If the file doesn't exist, treat it as success (idempotent delete)
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to remove local file %s: %w", fullPath, err)
	}

	// Optional: Clean up empty parent directories? (Could be complex/risky)

	return nil
}

// FileExists checks if a file exists on the local filesystem.
func (l *LocalStorage) FileExists(ctx context.Context, objectName string) (bool, error) {
	fullPath, err := l.getFullPath(objectName)
	if err != nil {
		// If the path is invalid, it can't exist.
		return false, err
	}

	_, err = os.Stat(fullPath)
	if err == nil {
		return true, nil // File exists
	}
	if os.IsNotExist(err) {
		return false, nil // File does not exist
	}
	// Some other error occurred
	return false, fmt.Errorf("failed to stat local file %s: %w", fullPath, err)
}
