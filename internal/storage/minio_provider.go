package storage

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/Suhaibinator/SProto/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioStorage implements the StorageProvider interface using MinIO.
type MinioStorage struct {
	client *minio.Client
	bucket string
}

// NewMinioStorage creates and initializes a new MinioStorage provider.
func NewMinioStorage(cfg config.Config) (*MinioStorage, error) {
	ctx := context.Background()

	// Initialize minio client object.
	minioClient, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Secure: cfg.MinioUseSSL,
	})
	if err != nil {
		log.Printf("Failed to initialize MinIO client: %v", err)
		return nil, fmt.Errorf("failed to initialize MinIO client: %w", err)
	}

	log.Printf("MinIO client initialized for endpoint: %s", cfg.MinioEndpoint)

	// Check if the bucket already exists.
	exists, err := minioClient.BucketExists(ctx, cfg.MinioBucket)
	if err != nil {
		log.Printf("Failed to check if MinIO bucket '%s' exists: %v", cfg.MinioBucket, err)
		return nil, fmt.Errorf("failed to check MinIO bucket existence: %w", err)
	}

	if !exists {
		// Create the bucket if it does not exist.
		log.Printf("MinIO bucket '%s' does not exist. Creating...", cfg.MinioBucket)
		err = minioClient.MakeBucket(ctx, cfg.MinioBucket, minio.MakeBucketOptions{}) // Use default region
		if err != nil {
			log.Printf("Failed to create MinIO bucket '%s': %v", cfg.MinioBucket, err)
			return nil, fmt.Errorf("failed to create MinIO bucket: %w", err)
		}
		log.Printf("Successfully created MinIO bucket '%s'", cfg.MinioBucket)
	} else {
		log.Printf("MinIO bucket '%s' already exists.", cfg.MinioBucket)
	}

	return &MinioStorage{
		client: minioClient,
		bucket: cfg.MinioBucket,
	}, nil
}

// UploadFile uploads data to MinIO.
func (m *MinioStorage) UploadFile(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error {
	opts := minio.PutObjectOptions{
		ContentType: contentType,
		// Consider adding UserMetadata if needed
	}
	_, err := m.client.PutObject(ctx, m.bucket, objectName, reader, size, opts)
	if err != nil {
		return fmt.Errorf("failed to upload object %s to minio: %w", objectName, err)
	}
	return nil
}

// DownloadFile retrieves a file from MinIO.
func (m *MinioStorage) DownloadFile(ctx context.Context, objectName string) (io.ReadCloser, error) {
	object, err := m.client.GetObject(ctx, m.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		// Check if the error is 'object not found'
		errResponse := minio.ToErrorResponse(err)
		if errResponse.Code == "NoSuchKey" {
			// Consider returning a more specific error type like os.ErrNotExist
			return nil, fmt.Errorf("object %s not found in minio: %w", objectName, err)
		}
		return nil, fmt.Errorf("failed to get object %s from minio: %w", objectName, err)
	}
	// The caller is responsible for closing the object reader.
	return object, nil
}

// DeleteFile removes a file from MinIO.
func (m *MinioStorage) DeleteFile(ctx context.Context, objectName string) error {
	opts := minio.RemoveObjectOptions{}
	err := m.client.RemoveObject(ctx, m.bucket, objectName, opts)
	if err != nil {
		return fmt.Errorf("failed to remove object %s from minio: %w", objectName, err)
	}
	return nil
}

// FileExists checks if a file exists in MinIO.
func (m *MinioStorage) FileExists(ctx context.Context, objectName string) (bool, error) {
	_, err := m.client.StatObject(ctx, m.bucket, objectName, minio.StatObjectOptions{})
	if err != nil {
		errResponse := minio.ToErrorResponse(err)
		if errResponse.Code == "NoSuchKey" {
			return false, nil // Object does not exist
		}
		// Some other error occurred
		return false, fmt.Errorf("failed to stat object %s in minio: %w", objectName, err)
	}
	return true, nil // Object exists
}
