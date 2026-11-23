//go:build gcp

package claude

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go/v2/googlecloud"
)

// GoogleCloudConfig names the project and workspace of Claude Models on
// Google Cloud (Anthropic managed). Empty values fall back to the SDK's
// environment variables. Credentials come from Application Default
// Credentials (the job's service account); there is no API key.
type GoogleCloudConfig struct {
	Project     string
	WorkspaceID string
	Location    string
	Model       string
	MaxTokens   int64
}

// NewGoogleCloud builds the client for the cloud targets.
func NewGoogleCloud(ctx context.Context, cfg GoogleCloudConfig) (*Gen, error) {
	c, err := googlecloud.NewClient(ctx, googlecloud.ClientConfig{
		Project: cfg.Project, WorkspaceID: cfg.WorkspaceID, Location: cfg.Location,
	})
	if err != nil {
		return nil, fmt.Errorf("claude: google cloud client: %w", err)
	}
	return newGen(c.Messages, c.Beta.Messages, cfg.Model, cfg.MaxTokens), nil
}
