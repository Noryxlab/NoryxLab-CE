package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

func TestWorkspaceBootstrapDoesNotSynchronizeDirectDatasetMounts(t *testing.T) {
	script := workspaceBootstrapScript(
		"vscode",
		"workspace-id",
		"token",
		false,
		"/home/noryx/.noryx-profile",
		"/mnt",
		nil,
		2,
	)

	if strings.Contains(script, "from minio import Minio") || strings.Contains(script, "initial_sync") {
		t.Fatal("direct dataset mounts must not trigger local S3 synchronization")
	}
	if !strings.Contains(script, "repos=0 datasets=2") {
		t.Fatal("bootstrap must report the number of direct dataset mounts")
	}
}

func TestWorkspaceAccessURLNeverContainsInternalToken(t *testing.T) {
	for _, kind := range []string{"jupyter", "vscode"} {
		accessURL := workspaceAccessURL(kind, "workspace-id")
		if strings.Contains(accessURL, "token=") {
			t.Fatalf("%s access URL exposes internal workspace token: %s", kind, accessURL)
		}
	}
}

func TestWorkspaceProxyRejectsSharedTokenWithoutSSO(t *testing.T) {
	workspaces := memory.NewWorkspaceStore()
	record := workspace.New(
		"jupyter", "project-id", "private-workspace", "image", "pod", "service",
		"500m", "512Mi", "/workspaces/workspace-id/lab?reset", "legacy-shared-token",
	)
	if err := workspaces.Create(record); err != nil {
		t.Fatal(err)
	}

	h := Handlers{workspaceStore: workspaces}
	request := httptest.NewRequest(http.MethodGet, "/workspaces/"+record.ID+"/lab?token=legacy-shared-token", nil)
	request.SetPathValue("workspaceID", record.ID)
	request.SetPathValue("path", "lab")
	response := httptest.NewRecorder()

	h.ProxyWorkspace(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("shared workspace URL must require SSO, got HTTP %d", response.Code)
	}
}
