package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/Suhaibinator/SProto/internal/config"
)

// StorageProvider defines the interface for interacting with the storage backend.
// This allows swapping between Minio, local filesystem, or other providers.
type StorageProvider interface {
	// UploadFile uploads data from a reader to the storage backend.
	// objectName is the full path/key for the object in the storage.
	// reader is the source of the data.
	// size is the total size of the data, required by some providers like MinIO.
	// contentType is the MIME type of the file (e.g., "application/octet-stream").
	UploadFile(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error

	// DownloadFile retrieves a file from the storage backend.
	// objectName is the full path/key of the object to retrieve.
	// Returns an io.ReadCloser which must be closed by the caller.
	DownloadFile(ctx context.Context, objectName string) (io.ReadCloser, error)

	// DeleteFile removes a file from the storage backend.
	// objectName is the full path/key of the object to delete.
	DeleteFile(ctx context.Context, objectName string) error

	// FileExists checks if a file exists in the storage backend.
	// objectName is the full path/key of the object to check.
	FileExists(ctx context.Context, objectName string) (bool, error)

	// GetPresignedURL generates a temporary URL for downloading a file (optional, may not be supported by all providers).
	// objectName is the full path/key of the object.
	// Returns the presigned URL string and an error if the operation fails or is unsupported.
	// GetPresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) // Example, not implementing yet
}

// Global storage provider instance
var provider StorageProvider

// InitStorage initializes the appropriate storage provider based on config.
func InitStorage(cfg config.Config) (StorageProvider, error) {
	var err error
	storageType := strings.ToLower(cfg.StorageType)
	log.Printf("Initializing storage provider: %s", storageType)

	switch storageType {
	case "minio":
		provider, err = NewMinioStorage(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Minio storage: %w", err)
		}
	case "local":
		provider, err = NewLocalStorage(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize local storage: %w", err)
		}
	default:
		return nil, fmt.Errorf("invalid STORAGE_TYPE: %s. Must be 'minio' or 'local'", cfg.StorageType)
	}

	log.Printf("Storage provider '%s' initialized successfully.", storageType)
	return provider, nil
}

// GetStorageProvider returns the initialized storage provider instance.
// Panics if InitStorage has not been called successfully.
func GetStorageProvider() StorageProvider {
	if provider == nil {
		panic("Storage provider has not been initialized. Call storage.InitStorage first.")
	}
	return provider
}

// SetStorageProvider is a test helper function.
// !! Use only in tests !!
func SetStorageProvider(p StorageProvider) {
	provider = p
}
