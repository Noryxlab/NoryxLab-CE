package handlers

import (
	"strings"

	"github.com/google/uuid"
)

func shortID() string {
	id := strings.ReplaceAll(uuid.NewString(), "-", "")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
