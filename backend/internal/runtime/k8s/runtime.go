package k8s

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

type Runtime struct {
	httpClient *http.Client
	apiURL     string
	token      string
	namespace  string
}

func NewFromInCluster(namespace string) (*Runtime, error) {
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
		httpClient: client,
		apiURL:     fmt.Sprintf("https://%s:%s", host, port),
		token:      strings.TrimSpace(string(tokenBytes)),
		namespace:  namespace,
	}, nil
}

func (r *Runtime) CreatePod(spec noryxruntime.PodSpec) error {
	payload := map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":   spec.PodName,
			"labels": spec.Labels,
		},
		"spec": map[string]any{
			"containers": []map[string]any{
				{
					"name":    "main",
					"image":   spec.Image,
					"command": spec.Command,
					"args":    spec.Args,
					"env":     spec.Env,
				},
			},
			"restartPolicy": "Never",
		},
	}

	if spec.PullSecret != "" {
		payload["spec"].(map[string]any)["imagePullSecrets"] = []map[string]string{{"name": spec.PullSecret}}
	}

	_, err := r.post(fmt.Sprintf("/api/v1/namespaces/%s/pods", r.namespace), payload)
	return err
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

	_, err := r.post(fmt.Sprintf("/apis/batch/v1/namespaces/%s/jobs", r.namespace), payload)
	return err
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
