package server_utils

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"
)

var (
	endpoint = GetEnv("GCS_PUBLIC_ENDPOINT", "https://storage.googleapis.com")
	bucket   = GetEnv("GCS_BUCKET_NAME", "")
)

// InitStorage initializes the Google Cloud Storage client and resolves the required
// configuration (bucket name and public endpoint). It ensures the bucket is set
// and returns the constructed client alongside the configuration struct.
func InitStorage(ctx context.Context) (*storage.Client, error) {
	if bucket == "" {
		return nil, fmt.Errorf("GCS_BUCKET_NAME must be set")
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCS client: %w", err)
	}

	return client, nil
}
