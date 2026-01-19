package ingest

import (
	"log/slog"

	"github.com/kfreiman/vibecheck/internal/converter"
	"github.com/kfreiman/vibecheck/internal/storage"
)

// IngestorConfig holds configuration for the document ingestor
type IngestorConfig struct {
	StorageManager    *storage.StorageManager
	DocumentConverter converter.DocumentConverter
	Logger            *slog.Logger
}

// NewIngestorWithConfig creates a new document ingestor with configuration
func NewIngestorWithConfig(config IngestorConfig) *DocumentIngestor {
	ingestor := &DocumentIngestor{
		storageManager:    config.StorageManager,
		documentConverter: config.DocumentConverter,
		logger:            config.Logger,
	}

	if ingestor.logger == nil {
		ingestor.logger = slog.Default()
	}

	return ingestor
}
