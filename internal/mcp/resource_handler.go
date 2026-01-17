package mcp

import (
	"context"
	"strings"

	"github.com/kfreiman/vibecheck/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// StorageResourceHandler handles cv:// and jd:// resource requests
type StorageResourceHandler struct {
	storageManager *storage.StorageManager
}

// NewStorageResourceHandler creates a new storage resource handler
func NewStorageResourceHandler(storageManager *storage.StorageManager) *StorageResourceHandler {
	return &StorageResourceHandler{
		storageManager: storageManager,
	}
}

// ReadResource processes resource requests for stored documents
func (h *StorageResourceHandler) ReadResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := req.Params.URI

	// Parse the URI
	_, _, err := storage.ParseURI(uri)
	if err != nil {
		return nil, mcp.ResourceNotFoundError(uri)
	}

	// Read the document
	content, err := h.storageManager.ReadDocument(uri)
	if err != nil {
		return nil, mcp.ResourceNotFoundError(uri)
	}

	// Strip frontmatter for the text content
	textContent := string(content)
	frontmatterStart := strings.Index(textContent, "---\n")
	if frontmatterStart >= 0 {
		// Find end of frontmatter
		frontmatterEnd := strings.Index(textContent[frontmatterStart+4:], "---\n")
		if frontmatterEnd >= 0 {
			textContent = textContent[frontmatterStart+4+frontmatterEnd+4:]
		}
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: "text/markdown",
			Text:     textContent,
		}},
	}, nil
}

// ListResources lists all available resources
func (h *StorageResourceHandler) ListResources() []*mcp.Resource {
	cvUUIDs, jdUUIDs, _ := h.storageManager.ListAllDocuments()

	resources := []*mcp.Resource{}

	// Add individual CV resources
	for _, uuid := range cvUUIDs {
		resources = append(resources, &mcp.Resource{
			URI:         "cv://" + uuid,
			Name:        "CV: " + uuid,
			Description: "Stored CV document",
			MIMEType:    "text/markdown",
		})
	}

	// Add individual JD resources
	for _, uuid := range jdUUIDs {
		resources = append(resources, &mcp.Resource{
			URI:         "jd://" + uuid,
			Name:        "JD: " + uuid,
			Description: "Stored job description",
			MIMEType:    "text/markdown",
		})
	}

	// Add stats resource if there are any documents
	if len(cvUUIDs) > 0 || len(jdUUIDs) > 0 {
		resources = append(resources, &mcp.Resource{
			URI:         "vibecheck://storage/stats",
			Name:        "Storage Statistics",
			Description: "Statistics about stored documents",
			MIMEType:    "text/markdown",
		})
	}

	return resources
}

// ListResourceTemplates returns resource templates for dynamic URIs
func (h *StorageResourceHandler) ListResourceTemplates() []mcp.ResourceTemplate {
	return []mcp.ResourceTemplate{
		{
			URITemplate: "cv://{id}",
			Name:        "CV Document",
			Description: "Access a stored CV document by its UUID",
			MIMEType:    "text/markdown",
		},
		{
			URITemplate: "jd://{id}",
			Name:        "Job Description",
			Description: "Access a stored job description by its UUID",
			MIMEType:    "text/markdown",
		},
	}
}
