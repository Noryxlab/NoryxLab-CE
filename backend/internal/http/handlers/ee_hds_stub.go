//go:build !enterprise

package handlers

// A Community build cannot manage regulated health datasets. Creation is
// refused and any HDS dataset already recorded stays hidden, so downgrading
// an installation cannot silently expose regulated data through an edition
// that has none of the controls for it.
func (h Handlers) hdsDatasetsAvailable() bool { return false }
