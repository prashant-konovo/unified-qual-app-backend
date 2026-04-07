package shared

import (
	"github.com/InCrowd/unified-qual-api/internal/service"
)

// Handler holds the remaining (non-auth) handler methods.
// It embeds *Services so all promoted fields are accessible.
type Handler struct{ *service.Services }
