package storage

import (
	"context"
	"fmt"
	"log" // Using standard log for simplicity

	"github.com/Suhaibinator/SProto/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioClient is the global MinIO client instance
var MinioClient *minio.Client

// InitMinio initializes the MinIO client and ensures the bucket exists.
func InitMinio(cfg config.Config) (*minio.Client, error) {
	var err error
	ctx := context.Background()

	// Initialize minio client object.
	MinioClient, err = minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Secure: cfg.MinioUseSSL,
	})
	if err != nil {
		log.Printf("Failed to initialize MinIO client: %v", err)
		return nil, fmt.Errorf("failed to initialize MinIO client: %w", err)
	}

	log.Printf("MinIO client initialized for endpoint: %s", cfg.MinioEndpoint)

	// Check if the bucket already exists.
	exists, err := MinioClient.BucketExists(ctx, cfg.MinioBucket)
	if err != nil {
		log.Printf("Failed to check if MinIO bucket '%s' exists: %v", cfg.MinioBucket, err)
		return nil, fmt.Errorf("failed to check MinIO bucket existence: %w", err)
	}

	if !exists {
		// Create the bucket if it does not exist.
		log.Printf("MinIO bucket '%s' does not exist. Creating...", cfg.MinioBucket)
		err = MinioClient.MakeBucket(ctx, cfg.MinioBucket, minio.MakeBucketOptions{}) // Use default region
		if err != nil {
			log.Printf("Failed to create MinIO bucket '%s': %v", cfg.MinioBucket, err)
			return nil, fmt.Errorf("failed to create MinIO bucket: %w", err)
		}
		log.Printf("Successfully created MinIO bucket '%s'", cfg.MinioBucket)
	} else {
		log.Printf("MinIO bucket '%s' already exists.", cfg.MinioBucket)
	}

	return MinioClient, nil
}

// GetMinioClient returns the initialized MinIO client instance.
// Panics if InitMinio has not been called successfully.
func GetMinioClient() *minio.Client {
	if MinioClient == nil {
		log.Fatal("MinIO client has not been initialized. Call storage.InitMinio first.")
	}
	return MinioClient
}
