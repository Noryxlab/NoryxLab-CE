package k8s

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

type Runtime struct {
	httpClient        *http.Client
	apiURL            string
	token             string
	controlNamespace  string
	workloadNamespace string
}

func NewFromInCluster(controlNamespace, workloadNamespace string) (*Runtime, error) {
	controlNamespace = strings.TrimSpace(controlNamespace)
	if controlNamespace == "" {
		return nil, fmt.Errorf("control namespace is required")
	}
	workloadNamespace = strings.TrimSpace(workloadNamespace)
	if workloadNamespace == "" {
		workloadNamespace = controlNamespace
	}

	tokenBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return nil, fmt.Errorf("read serviceaccount token: %w", err)
	}

	caPEM, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		return nil, fmt.Errorf("read serviceaccount ca: %w", err)
	}

	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return nil, fmt.Errorf("kubernetes service env not set")
	}

	pool, err := newCertPool(caPEM)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool},
		},
	}

	return &Runtime{
		httpClient:        client,
		apiURL:            fmt.Sprintf("https://%s:%s", host, port),
		token:             strings.TrimSpace(string(tokenBytes)),
		controlNamespace:  controlNamespace,
		workloadNamespace: workloadNamespace,
	}, nil
}

func (r *Runtime) CreatePod(spec noryxruntime.PodSpec) error {
	ports := make([]map[string]any, 0, len(spec.Ports))
	for _, p := range spec.Ports {
		ports = append(ports, map[string]any{"containerPort": p})
	}

	resources := map[string]any{}
	requests := map[string]string{}
	limits := map[string]string{}
	if spec.CPURequest != "" {
		requests["cpu"] = spec.CPURequest
	}
	if spec.MemRequest != "" {
		requests["memory"] = spec.MemRequest
	}
	if spec.CPULimit != "" {
		limits["cpu"] = spec.CPULimit
	}
	if spec.MemLimit != "" {
		limits["memory"] = spec.MemLimit
	}
	if len(requests) > 0 {
		resources["requests"] = requests
	}
	if len(limits) > 0 {
		resources["limits"] = limits
	}

	container := map[string]any{
		"name":    "main",
		"image":   spec.Image,
		"command": spec.Command,
		"args":    spec.Args,
		"env":     spec.Env,
	}
	if len(ports) > 0 {
		container["ports"] = ports
	}
	if len(resources) > 0 {
		container["resources"] = resources
	}

	payload := map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":   spec.PodName,
			"labels": spec.Labels,
		},
		"spec": map[string]any{
			"containers":    []map[string]any{container},
			"restartPolicy": "Never",
		},
	}

	if spec.PullSecret != "" {
		payload["spec"].(map[string]any)["imagePullSecrets"] = []map[string]string{{"name": spec.PullSecret}}
	}

	_, err := r.post(fmt.Sprintf("/api/v1/namespaces/%s/pods", r.workloadNamespace), payload)
	return err
}

func (r *Runtime) DeletePod(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("pod name is required")
	}
	return r.delete(fmt.Sprintf("/api/v1/namespaces/%s/pods/%s", r.workloadNamespace, name))
}

func (r *Runtime) CreateService(spec noryxruntime.ServiceSpec) error {
	payload := map[string]any{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata": map[string]any{
			"name": spec.Name,
		},
		"spec": map[string]any{
			"selector": spec.Selector,
			"ports": []map[string]any{
				{
					"port":       spec.Port,
					"targetPort": spec.Port,
				},
			},
		},
	}

	_, err := r.post(fmt.Sprintf("/api/v1/namespaces/%s/services", r.workloadNamespace), payload)
	return err
}

func (r *Runtime) DeleteService(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("service name is required")
	}
	return r.delete(fmt.Sprintf("/api/v1/namespaces/%s/services/%s", r.workloadNamespace, name))
}

func (r *Runtime) CreateBuild(spec noryxruntime.BuildSpec) error {
	contextArg := spec.ContextGitURL
	if spec.GitRef != "" {
		contextArg = fmt.Sprintf("%s#refs/heads/%s", spec.ContextGitURL, spec.GitRef)
	}
	if strings.HasPrefix(spec.GitRef, "refs/") {
		contextArg = fmt.Sprintf("%s#%s", spec.ContextGitURL, spec.GitRef)
	}

	args := []string{
		"--context=" + contextArg,
		"--dockerfile=" + spec.DockerfilePath,
		"--destination=" + spec.DestinationImage,
		"--insecure",
		"--skip-tls-verify",
	}
	if spec.ContextPath != "" {
		args = append(args, "--context-sub-path="+spec.ContextPath)
	}

	container := map[string]any{
		"name":  "kaniko",
		"image": "gcr.io/kaniko-project/executor:v1.23.2",
		"args":  args,
	}

	podSpec := map[string]any{
		"restartPolicy": "Never",
		"containers":    []map[string]any{container},
	}

	if spec.PullSecret != "" {
		podSpec["imagePullSecrets"] = []map[string]string{{"name": spec.PullSecret}}
	}

	if spec.RegistrySecretName != "" {
		podSpec["volumes"] = []map[string]any{
			{
				"name": "docker-config",
				"secret": map[string]any{
					"secretName": spec.RegistrySecretName,
					"items": []map[string]string{
						{
							"key":  ".dockerconfigjson",
							"path": "config.json",
						},
					},
				},
			},
		}
		container["volumeMounts"] = []map[string]any{
			{
				"name":      "docker-config",
				"mountPath": "/kaniko/.docker",
			},
		}
	}

	payload := map[string]any{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": map[string]any{
			"name":   spec.JobName,
			"labels": spec.Labels,
		},
		"spec": map[string]any{
			"ttlSecondsAfterFinished": int64(86400),
			"backoffLimit":            int64(0),
			"template": map[string]any{
				"metadata": map[string]any{
					"labels": spec.Labels,
				},
				"spec": podSpec,
			},
		},
	}

	_, err := r.post(fmt.Sprintf("/apis/batch/v1/namespaces/%s/jobs", r.workloadNamespace), payload)
	return err
}

func (r *Runtime) ListDeployments() ([]noryxruntime.DeploymentStatus, error) {
	body, err := r.get(fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments", r.controlNamespace))
	if err != nil {
		return nil, err
	}

	var response struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct {
				Replicas int `json:"replicas"`
			} `json:"spec"`
			Status struct {
				ReadyReplicas     int `json:"readyReplicas"`
				AvailableReplicas int `json:"availableReplicas"`
				UpdatedReplicas   int `json:"updatedReplicas"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	out := make([]noryxruntime.DeploymentStatus, 0, len(response.Items))
	for _, item := range response.Items {
		out = append(out, noryxruntime.DeploymentStatus{
			Name:              item.Metadata.Name,
			Replicas:          item.Spec.Replicas,
			ReadyReplicas:     item.Status.ReadyReplicas,
			AvailableReplicas: item.Status.AvailableReplicas,
			UpdatedReplicas:   item.Status.UpdatedReplicas,
		})
	}
	return out, nil
}

func (r *Runtime) ListServices() ([]noryxruntime.ServiceStatus, error) {
	body, err := r.get(fmt.Sprintf("/api/v1/namespaces/%s/services", r.controlNamespace))
	if err != nil {
		return nil, err
	}

	var response struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct {
				Type string `json:"type"`
			} `json:"spec"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	out := make([]noryxruntime.ServiceStatus, 0, len(response.Items))
	for _, item := range response.Items {
		out = append(out, noryxruntime.ServiceStatus{
			Name: item.Metadata.Name,
			Type: item.Spec.Type,
		})
	}
	return out, nil
}

func (r *Runtime) IsServiceReady(serviceName string) (bool, error) {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return false, fmt.Errorf("service name is required")
	}

	body, err := r.get(fmt.Sprintf("/api/v1/namespaces/%s/endpoints/%s", r.workloadNamespace, serviceName))
	if err != nil {
		return false, err
	}

	var response struct {
		Subsets []struct {
			Addresses []any `json:"addresses"`
			Ports     []any `json:"ports"`
		} `json:"subsets"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return false, err
	}

	for _, subset := range response.Subsets {
		if len(subset.Addresses) > 0 && len(subset.Ports) > 0 {
			return true, nil
		}
	}
	return false, nil
}

func (r *Runtime) ListWorkspaces() ([]noryxruntime.WorkspaceRuntimeInfo, error) {
	selector := url.QueryEscape("app.kubernetes.io/name=noryx-workspace")
	body, err := r.get(fmt.Sprintf("/api/v1/namespaces/%s/pods?labelSelector=%s", r.workloadNamespace, selector))
	if err != nil {
		return nil, err
	}

	var response struct {
		Items []struct {
			Metadata struct {
				Name   string            `json:"name"`
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				Containers []struct {
					Image string `json:"image"`
					Args  []string `json:"args"`
				} `json:"containers"`
			} `json:"spec"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	out := make([]noryxruntime.WorkspaceRuntimeInfo, 0, len(response.Items))
	for _, item := range response.Items {
		workspaceID := strings.TrimSpace(item.Metadata.Labels["noryx.io/workspace-id"])
		projectID := strings.TrimSpace(item.Metadata.Labels["noryx.io/project-id"])
		if workspaceID == "" || projectID == "" {
			continue
		}
		image := ""
		accessToken := ""
		if len(item.Spec.Containers) > 0 {
			image = strings.TrimSpace(item.Spec.Containers[0].Image)
			for _, arg := range item.Spec.Containers[0].Args {
				if strings.HasPrefix(arg, "--ServerApp.token=") {
					accessToken = strings.TrimPrefix(arg, "--ServerApp.token=")
					break
				}
			}
		}
		podName := strings.TrimSpace(item.Metadata.Name)
		if podName == "" {
			continue
		}
		out = append(out, noryxruntime.WorkspaceRuntimeInfo{
			WorkspaceID: workspaceID,
			ProjectID:   projectID,
			PodName:     podName,
			ServiceName: podName,
			Image:       image,
			AccessToken: strings.TrimSpace(accessToken),
		})
	}

	return out, nil
}

func (r *Runtime) post(path string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, r.apiURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("kubernetes api %s failed: status=%d body=%s", path, resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

func (r *Runtime) get(path string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, r.apiURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("Accept", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("kubernetes api %s failed: status=%d body=%s", path, resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

func (r *Runtime) delete(path string) error {
	req, err := http.NewRequest(http.MethodDelete, r.apiURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("Accept", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("kubernetes api %s failed: status=%d body=%s", path, resp.StatusCode, string(respBody))
	}
	return nil
}
