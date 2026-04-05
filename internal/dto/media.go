package dto

import (
	"time"

	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
)

// MediaToJSON converts ICInterviewMedia to the legacy JSON response shape.
func MediaToJSON(m iris.ICInterviewMedia) map[string]any {
	result := map[string]any{
		"id": m.ID, "name": m.Name, "description": m.Description,
		"projectId": m.ProjectID, "s3Key": nil, "hash": nil,
		"status": m.Status, "createdOn": m.CreatedOn.Format(time.RFC3339),
		"createdBy": m.CreatedBy, "pageCount": m.PageCount,
		"pagesProcessed": m.PagesProcessed, "shared": m.Shared,
	}
	if m.S3Key.Valid {
		result["s3Key"] = m.S3Key.String
	}
	if m.Hash.Valid {
		result["hash"] = m.Hash.String
	}
	return result
}
