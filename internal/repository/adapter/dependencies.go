package adapter

import (
	"fmt"
	"strings"

	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// ---------------------------------------------------------------------------
// Brand
// ---------------------------------------------------------------------------

// Brand represents the business brand / service category.
// LS (Life Sciences) maps to the IRIS database.
// MRA (Market Research & Analysis) maps to the QS database.
type Brand string

const (
	BrandLS  Brand = "LS"
	BrandMRA Brand = "MRA"
)

// BrandFromSource converts a data-source string ("iris", "qs") to a Brand.
// Returns an error if the source is unknown.
func BrandFromSource(source string) (Brand, error) {
	switch strings.ToLower(source) {
	case "iris", "ls":
		return BrandLS, nil
	case "qs", "mra":
		return BrandMRA, nil
	default:
		return "", fmt.Errorf("unknown brand source: %q", source)
	}
}

// String returns the brand as a string.
func (b Brand) String() string {
	return string(b)
}

// IsLS returns true if the brand is Life Sciences (IRIS).
func (b Brand) IsLS() bool {
	return b == BrandLS
}

// IsMRA returns true if the brand is Market Research & Analysis (QS).
func (b Brand) IsMRA() bool {
	return b == BrandMRA
}

// ErrUnsupportedBrand is returned when an adapter method receives an unknown brand.
var ErrUnsupportedBrand = fmt.Errorf("unsupported brand")

// ---------------------------------------------------------------------------
// Adapters container
// ---------------------------------------------------------------------------

// Adapters holds all brand-aware repository adapter instances.
// Add new adapters here as they are created.
type Adapters struct {
	Project *ProjectAdapter
	// User     *UserAdapter       // TODO: add when needed
	// Moderator *ModeratorAdapter // TODO: add when needed
	// TimeSlot *TimeSlotAdapter   // TODO: add when needed
	// Survey   *SurveyAdapter     // TODO: add when needed
}

// NewAdapters creates all adapter instances from the IRIS and QS repository containers.
func NewAdapters(irisRepos *iris.Repositories, qsRepos *qs.Repositories) *Adapters {
	return &Adapters{
		Project: NewProjectAdapter(irisRepos.Project, qsRepos.Project),
	}
}
