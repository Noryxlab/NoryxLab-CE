package handlers

import (
	"errors"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/repository"
)

func TestValidateRepositoryGitIdentity(t *testing.T) {
	tests := []struct {
		name      string
		author    string
		email     string
		expectErr bool
	}{
		{name: "empty fallback identity"},
		{name: "complete identity", author: "Git Author", email: "git@example.org"},
		{name: "missing email", author: "Git Author", expectErr: true},
		{name: "missing name", email: "git@example.org", expectErr: true},
		{name: "invalid email", author: "Git Author", email: "not-an-email", expectErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateRepositoryGitIdentity(test.author, test.email)
			if (err != nil) != test.expectErr {
				t.Fatalf("error=%v expectErr=%v", err, test.expectErr)
			}
		})
	}
}

func TestSetRepositoryValidation(t *testing.T) {
	item := repository.Repository{}
	setRepositoryValidation(&item, nil)
	if !item.Reachable || item.ValidationError != "" || item.LastValidatedAt == nil {
		t.Fatalf("unexpected successful validation state: %#v", item)
	}

	setRepositoryValidation(&item, errors.New("authentication failed"))
	if item.Reachable || item.ValidationError != "authentication failed" || item.LastValidatedAt == nil {
		t.Fatalf("unexpected failed validation state: %#v", item)
	}
}
