//go:build !enterprise

package handlers

import "net/http"

// The audit trail is an Enterprise capability. Community code still calls
// these from every mutating handler, so the signatures stay part of the core
// and the recording is what the edition supplies. Discarding is deliberate: a
// Community build that silently kept a partial trail would be worse than one
// that keeps none, because an operator would believe they had one.

func (h Handlers) emitAudit(_ *http.Request, _, _, _, _, _, _, _ string, _ map[string]any) {}

func (h Handlers) emitSystemAudit(_, _, _, _ string, _ map[string]any) {}

func (h Handlers) emitAdvancedAudit(_ *http.Request, _, _, _, _, _, _, _ string, _ map[string]any) {
}
