package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsAuditedMutation(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   bool
	}{
		{http.MethodPost, "/api/v1/projects", true},
		{http.MethodPut, "/api/v1/datasets/id/ownership", true},
		{http.MethodDelete, "/api/v1/secrets/name", true},
		{http.MethodGet, "/api/v1/projects", false},
		{http.MethodPost, "/workspaces/id/files", false},
	}
	for _, test := range tests {
		request := httptest.NewRequest(test.method, test.path, nil)
		if got := isAuditedMutation(request); got != test.want {
			t.Fatalf("%s %s: got %v, want %v", test.method, test.path, got, test.want)
		}
	}
}

func TestMutationAuditResource(t *testing.T) {
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/project-1/datasets/dataset-1", nil)
	request.SetPathValue("projectID", "project-1")
	request.SetPathValue("datasetID", "dataset-1")

	resourceType, resourceID, projectID := mutationAuditResource(request)
	if resourceType != "project" || resourceID != "project-1" || projectID != "project-1" {
		t.Fatalf("unexpected resource: %q %q %q", resourceType, resourceID, projectID)
	}
}
