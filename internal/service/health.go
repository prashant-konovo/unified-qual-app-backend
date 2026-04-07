package service

import (
	"context"

	"github.com/InCrowd/unified-qual-api/internal/database"
)

// HealthService provides application health checks.
type HealthService struct {
	db *database.DBPair
}

// NewHealthService creates a HealthService.
func NewHealthService(db *database.DBPair) *HealthService {
	return &HealthService{db: db}
}

// Check returns per-database connectivity status.
func (s *HealthService) Check(ctx context.Context) map[string]string {
	if s.db == nil {
		return map[string]string{
			"incrowdDB":   "not_configured",
			"incrowdRODB": "not_configured",
			"qstoolDB":    "not_configured",
		}
	}
	return s.db.HealthCheck(ctx)
}
