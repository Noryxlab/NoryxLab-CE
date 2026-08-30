//go:build !enterprise

package http

import (
	"net/http"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/http/handlers"
)

// The Community build registers no Enterprise routes. This is the whole
// point of the build tag: the handlers are not linked into the binary, so
// there is nothing to route to and no flag that could change that.
func registerEnterpriseRoutes(_ *http.ServeMux, _ handlers.Handlers) {}
