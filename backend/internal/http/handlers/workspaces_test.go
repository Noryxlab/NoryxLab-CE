package handlers

import (
	"strings"
	"testing"
)

func TestWorkspaceBootstrapMarksReaderDatasetsReadOnly(t *testing.T) {
	script := workspaceBootstrapScript(
		"vscode",
		"workspace-id",
		"token",
		false,
		"/home/noryx/.noryx-profile",
		"/mnt",
		nil,
		[]workspaceAttachedDataset{
			{Name: "reader-dataset", Bucket: "reader", Endpoint: "s3.example.test", ReadOnly: true},
			{Name: "writer-dataset", Bucket: "writer", Endpoint: "s3.example.test"},
		},
		"",
		"",
		"",
		false,
	)

	if !strings.Contains(script, "'name': \"reader-dataset\"") || !strings.Contains(script, "'read_only': True") {
		t.Fatal("reader dataset is not marked read-only in workspace bootstrap")
	}
	if !strings.Contains(script, "'name': \"writer-dataset\"") || !strings.Contains(script, "'read_only': False") {
		t.Fatal("writer dataset is not marked writable in workspace bootstrap")
	}
	if !strings.Contains(script, "if ds.get('read_only', False):") {
		t.Fatal("workspace dataset write-back does not skip read-only datasets")
	}
	if !strings.Contains(script, "ThreadPoolExecutor") {
		t.Fatal("workspace initial dataset synchronization is not concurrent")
	}
}
