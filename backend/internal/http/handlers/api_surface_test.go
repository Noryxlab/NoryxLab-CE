package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
)

const sampleAPIDocument = `
openapi: 3.0.3
paths:
  /api/v1/apps/{appID}/restart:
    post:
      tags: [Apps]
      summary: Restart an application
      operationId: restartApp
      parameters:
        - name: appID
          in: path
          required: true
      responses:
        '200':
          description: Restarted
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/App'
  /api/v1/workflows:
    post:
      summary: Write a workflow down
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Workflow'
      responses:
        '201':
          description: Created
  /healthz:
    get:
      summary: Health check
components:
  schemas:
    App:
      type: object
      properties:
        id: { type: string }
    Workflow:
      type: object
      properties:
        name: { type: string }
        steps:
          type: array
          items:
            $ref: '#/components/schemas/Workflow'
`

// The index is the document's operations under /api/v1, with references
// turned into fields - and a schema that refers to itself stops somewhere.
func TestTheAPIIndexIsReadFromTheDocument(t *testing.T) {
	operations, _, err := parseAPIDocument([]byte(sampleAPIDocument))
	if err != nil {
		t.Fatal(err)
	}
	if len(operations) != 2 {
		t.Fatalf("%d operations, expected 2 (healthz is not an API path): %+v", len(operations), operations)
	}
	restart, found := findAPIOperation(operations, "post", "/api/v1/apps/abc/restart")
	if !found || restart.OperationID != "restartApp" || len(restart.Parameters) != 1 {
		t.Fatalf("the concrete path did not match its template: %+v", restart)
	}
	response, _ := restart.Response.(map[string]any)
	if _, ok := response["properties"]; !ok {
		t.Fatalf("the response reference was not resolved: %+v", restart.Response)
	}
	workflow, _ := findAPIOperation(operations, "POST", "/api/v1/workflows")
	encoded, _ := json.Marshal(workflow.RequestBody)
	if !strings.Contains(string(encoded), `"$ref":"Workflow"`) {
		t.Fatalf("a self-referencing schema should stop at a name:\n%s", encoded)
	}
}

// A call goes through the router as the identity the caller placed in the
// context, and nothing else: no header, no token, no session.
func TestAnInternalCallReachesTheHandlerAsItsOwner(t *testing.T) {
	var seenUser, seenMethod string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/apps/{appID}/restart", func(w http.ResponseWriter, r *http.Request) {
		identity, _ := auth.IdentityFrom(r.Context())
		seenUser, seenMethod = identity.UserID(), r.Method
		writeJSON(w, http.StatusOK, map[string]any{"id": r.PathValue("appID"), "status": "restarted"})
	})
	h := Handlers{api: &apiSurface{}}
	h.AttachAPI(mux, []byte(sampleAPIDocument))

	owner := auth.Identity{Username: "amandine", Roles: map[string]struct{}{}}
	result, err := h.callAPI(context.Background(), owner, apiCall{Method: "post", Path: "/api/v1/apps/abc/restart"})
	if err != nil {
		t.Fatal(err)
	}
	if seenUser != "amandine" || seenMethod != http.MethodPost {
		t.Fatalf("the handler saw %q doing %s", seenUser, seenMethod)
	}
	body, _ := result.Body.(map[string]any)
	if result.Status != http.StatusOK || body["id"] != "abc" {
		t.Fatalf("answer = %d %+v", result.Status, result.Body)
	}
}

// What the router refuses comes back as an answer; what this layer refuses
// never reaches the router.
func TestAnInternalCallStaysOnTheAPI(t *testing.T) {
	reached := false
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { reached = true })
	h := Handlers{api: &apiSurface{}}
	h.AttachAPI(mux, []byte(sampleAPIDocument))
	owner := auth.Identity{Username: "amandine", Roles: map[string]struct{}{}}

	for _, call := range []apiCall{
		{Method: "GET", Path: "/swagger"},
		{Method: "GET", Path: "/api/v1/assistant/tools"},
		{Method: "TRACE", Path: "/api/v1/projects"},
		{Method: "GET", Path: "/api/v1/../admin"},
	} {
		if _, err := h.callAPI(context.Background(), owner, call); err == nil {
			t.Fatalf("%s %s was dispatched", call.Method, call.Path)
		}
	}
	if reached {
		t.Fatal("a refused call reached the router")
	}
}

// A request from the wire never carries a context identity, and a request
// built here never asks for a credential.
func TestAContextIdentityIsTakenAsResolved(t *testing.T) {
	h := Handlers{authMode: "oidc"}
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	if _, ok := h.requireIdentity(httptest.NewRecorder(), request); ok {
		t.Fatal("a bare request was identified")
	}
	request = request.WithContext(auth.WithIdentity(request.Context(), auth.Identity{Username: "amandine"}))
	identity, ok := h.requireIdentity(httptest.NewRecorder(), request)
	if !ok || identity.UserID() != "amandine" {
		t.Fatalf("the context identity was not honoured: %v %+v", ok, identity)
	}
}
