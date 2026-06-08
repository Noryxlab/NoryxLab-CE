package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/edition"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

func TestAdvancedAuditIsEnterpriseGated(t *testing.T) {
	audits := memory.NewAuditStore()
	request := httptest.NewRequest("GET", "/api/v1/datasets/example/objects/file.csv", nil)
	ce := Handlers{auditStore: audits}
	ce.emitAdvancedAudit(request, "stef", "dataset.object.download", "dataset", "example", "", "success", "", nil)

	items, err := audits.List(store.AuditFilter{})
	if err != nil || len(items) != 0 {
		t.Fatal("expected advanced audit event to remain disabled in CE")
	}

	ee := Handlers{
		auditStore: audits,
		editionHooks: edition.Hooks{
			Feature: edition.FeatureGateFromCSV(edition.FeatureAdvancedAudit),
		},
	}
	ee.emitAdvancedAudit(request, "stef", "dataset.object.download", "dataset", "example", "", "success", "", nil)
	items, err = audits.List(store.AuditFilter{})
	if err != nil || len(items) != 1 || items[0].Action != "dataset.object.download" {
		t.Fatal("expected advanced audit event when the Enterprise feature is enabled")
	}
}
