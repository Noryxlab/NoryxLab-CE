package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
)

// The platform's API, used from inside the platform.
//
// An agent's verbs are the platform's endpoints (ADR-046). Rather than
// describe each one a second time as a tool, the agent is given three things:
// an index of the operations the OpenAPI document declares, a description of
// any one of them, and a way to call one as its owner. The call is dispatched
// through the real router, so the handler that answers it is the handler a
// browser would reach, with its own access check and its own audit entry.
//
// Nothing here decides who may call what. The identity is placed in the
// request context by the caller, which is the only place a context value can
// come from, and the handlers do the rest as they always have.

// apiSurface is attached by the server once the router exists. The spec is
// parsed on first use and kept: 9,000 lines of YAML are read once, not per
// question.
type apiSurface struct {
	handler http.Handler
	spec    []byte

	once       sync.Once
	operations []apiOperation
	schemas    map[string]any
	parseErr   error
}

// apiOperation is one method on one path, as the document declares it.
type apiOperation struct {
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	Summary     string         `json:"summary,omitempty"`
	OperationID string         `json:"operationId,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Parameters  []apiParameter `json:"parameters,omitempty"`
	RequestBody any            `json:"requestBody,omitempty"`
	Response    any            `json:"response,omitempty"`
}

type apiParameter struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Required    bool   `json:"required,omitempty"`
	Description string `json:"description,omitempty"`
}

// AttachAPI gives the handlers their own router and the document that
// describes it. Called by the server after both exist.
func (h Handlers) AttachAPI(handler http.Handler, spec []byte) {
	if h.api == nil {
		return
	}
	h.api.handler = handler
	h.api.spec = spec
}

// apiOperations returns the index, parsing the document the first time.
func (h Handlers) apiOperations() ([]apiOperation, error) {
	if h.api == nil || len(h.api.spec) == 0 {
		return nil, fmt.Errorf("the API description is not available")
	}
	h.api.once.Do(func() {
		h.api.operations, h.api.schemas, h.api.parseErr = parseAPIDocument(h.api.spec)
	})
	return h.api.operations, h.api.parseErr
}

// The methods the document may declare under a path, in the order they are
// listed.
var apiMethods = []string{"get", "post", "put", "patch", "delete"}

// parseAPIDocument reads the operations out of an OpenAPI 3 document.
//
// It keeps the index light: summaries, parameters, and the request body and
// success response with their references resolved a few levels down. A model
// reading this needs to know what to send, not every alternative of every
// nested schema.
func parseAPIDocument(spec []byte) ([]apiOperation, map[string]any, error) {
	var document map[string]any
	if err := yaml.Unmarshal(spec, &document); err != nil {
		return nil, nil, fmt.Errorf("the API description could not be read: %w", err)
	}
	schemas := map[string]any{}
	if components, ok := document["components"].(map[string]any); ok {
		if declared, ok := components["schemas"].(map[string]any); ok {
			schemas = declared
		}
	}
	paths, _ := document["paths"].(map[string]any)
	operations := make([]apiOperation, 0, len(paths)*2)
	for path, entry := range paths {
		if !strings.HasPrefix(path, "/api/v1/") {
			continue
		}
		byMethod, _ := entry.(map[string]any)
		for _, method := range apiMethods {
			raw, ok := byMethod[method].(map[string]any)
			if !ok {
				continue
			}
			operation := apiOperation{Method: strings.ToUpper(method), Path: path}
			operation.Summary, _ = raw["summary"].(string)
			operation.OperationID, _ = raw["operationId"].(string)
			if tags, ok := raw["tags"].([]any); ok {
				for _, tag := range tags {
					if name, ok := tag.(string); ok {
						operation.Tags = append(operation.Tags, name)
					}
				}
			}
			if parameters, ok := raw["parameters"].([]any); ok {
				for _, item := range parameters {
					parameter, _ := item.(map[string]any)
					name, _ := parameter["name"].(string)
					in, _ := parameter["in"].(string)
					required, _ := parameter["required"].(bool)
					description, _ := parameter["description"].(string)
					if name != "" {
						operation.Parameters = append(operation.Parameters,
							apiParameter{Name: name, In: in, Required: required, Description: description})
					}
				}
			}
			if body, ok := raw["requestBody"].(map[string]any); ok {
				operation.RequestBody = resolveAPISchema(jsonContentSchema(body), schemas, 0)
			}
			if responses, ok := raw["responses"].(map[string]any); ok {
				for _, code := range []string{"200", "201", "202", "204"} {
					if response, ok := responses[code].(map[string]any); ok {
						operation.Response = resolveAPISchema(jsonContentSchema(response), schemas, 0)
						break
					}
				}
			}
			operations = append(operations, operation)
		}
	}
	sort.Slice(operations, func(a, b int) bool {
		if operations[a].Path != operations[b].Path {
			return operations[a].Path < operations[b].Path
		}
		return operations[a].Method < operations[b].Method
	})
	return operations, schemas, nil
}

// jsonContentSchema picks the JSON schema out of a request body or response,
// which is the only content type the API speaks.
func jsonContentSchema(entry map[string]any) any {
	content, _ := entry["content"].(map[string]any)
	for mediaType, declared := range content {
		if !strings.Contains(mediaType, "json") {
			continue
		}
		if media, ok := declared.(map[string]any); ok {
			return media["schema"]
		}
	}
	return nil
}

// resolveAPISchema inlines $ref a few levels deep, so a reader gets the
// fields rather than a name. The depth is capped because schemas refer to
// each other and some refer to themselves.
func resolveAPISchema(schema any, schemas map[string]any, depth int) any {
	const maxDepth = 4
	switch value := schema.(type) {
	case map[string]any:
		if ref, ok := value["$ref"].(string); ok {
			name := strings.TrimPrefix(ref, "#/components/schemas/")
			if depth >= maxDepth {
				return map[string]any{"$ref": name}
			}
			return resolveAPISchema(schemas[name], schemas, depth+1)
		}
		resolved := make(map[string]any, len(value))
		for key, nested := range value {
			resolved[key] = resolveAPISchema(nested, schemas, depth)
		}
		return resolved
	case []any:
		resolved := make([]any, len(value))
		for index, nested := range value {
			resolved[index] = resolveAPISchema(nested, schemas, depth)
		}
		return resolved
	default:
		return schema
	}
}

// findAPIOperation matches a concrete path against the declared templates:
// /api/v1/apps/abc matches /api/v1/apps/{appID}.
func findAPIOperation(operations []apiOperation, method, path string) (apiOperation, bool) {
	method = strings.ToUpper(strings.TrimSpace(method))
	path = strings.TrimSpace(path)
	for _, operation := range operations {
		if operation.Method == method && apiPathMatches(operation.Path, path) {
			return operation, true
		}
	}
	return apiOperation{}, false
}

func apiPathMatches(template, path string) bool {
	if template == path {
		return true
	}
	templateParts := strings.Split(strings.Trim(template, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	if len(templateParts) != len(pathParts) {
		return false
	}
	for index, part := range templateParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			continue
		}
		if part != pathParts[index] {
			return false
		}
	}
	return true
}

// apiCall is one request made from inside the process on behalf of a person.
type apiCall struct {
	Method string
	Path   string
	Query  map[string]string
	Body   json.RawMessage
}

// apiCallResult is what the platform answered, as data: a refusal is a fact
// the caller reports, not an exception it swallows.
type apiCallResult struct {
	Status int
	Body   any
}

// The body an in-process call may carry and the answer it may bring back.
// Both exist because the caller is a model: an agent that uploads a dataset
// through its tool loop, or reads 40,000 objects into its context, is doing
// the wrong thing with the wrong tool.
const (
	maxAPICallBody     = 64 << 10
	maxAPICallResponse = 128 << 10
)

// callAPI dispatches one request through the router as the given identity.
//
// The path must be an API path and never the assistant's own tool endpoint -
// a tool that could call the tool endpoint is a loop with a credential in it.
// Everything else is the router's business: an unknown path answers 404, a
// forbidden one 403, each from the handler that owns it.
func (h Handlers) callAPI(ctx context.Context, identity auth.Identity, call apiCall) (apiCallResult, error) {
	if h.api == nil || h.api.handler == nil {
		return apiCallResult{}, fmt.Errorf("the platform API is not attached")
	}
	method := strings.ToUpper(strings.TrimSpace(call.Method))
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return apiCallResult{}, fmt.Errorf("method %q is not one the API accepts", call.Method)
	}
	path := strings.TrimSpace(call.Path)
	if !strings.HasPrefix(path, "/api/v1/") || strings.Contains(path, "..") {
		return apiCallResult{}, fmt.Errorf("the path must be an API path under /api/v1/")
	}
	if strings.HasPrefix(path, "/api/v1/assistant/") {
		return apiCallResult{}, fmt.Errorf("the assistant's own endpoints are not callable this way")
	}
	if len(call.Body) > maxAPICallBody {
		return apiCallResult{}, fmt.Errorf("the request body is larger than %d KiB", maxAPICallBody>>10)
	}

	target := &url.URL{Path: path}
	if len(call.Query) > 0 {
		values := url.Values{}
		for key, value := range call.Query {
			values.Set(key, value)
		}
		target.RawQuery = values.Encode()
	}
	var body io.Reader
	if len(bytes.TrimSpace(call.Body)) > 0 {
		body = bytes.NewReader(call.Body)
	}
	request, err := http.NewRequestWithContext(auth.WithIdentity(ctx, identity), method, target.String(), body)
	if err != nil {
		return apiCallResult{}, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	// Named for the audit trail, which records where a request came from:
	// this one came from nowhere on the network, and saying so is better than
	// an empty address.
	request.RemoteAddr = "agent:" + identity.UserID()
	request.Header.Set("X-Forwarded-For", "agent")

	recorder := httptest.NewRecorder()
	h.api.handler.ServeHTTP(recorder, request)

	result := apiCallResult{Status: recorder.Code}
	answer := recorder.Body.Bytes()
	if len(answer) > maxAPICallResponse {
		return result, fmt.Errorf("the answer is larger than %d KiB; ask for less", maxAPICallResponse>>10)
	}
	if len(bytes.TrimSpace(answer)) == 0 {
		return result, nil
	}
	var decoded any
	if json.Unmarshal(answer, &decoded) == nil {
		result.Body = decoded
	} else {
		result.Body = string(answer)
	}
	return result, nil
}
