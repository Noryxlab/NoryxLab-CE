package handlers

import (
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/usage"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Consumption is arithmetic, and arithmetic is worth checking: this is the
// number a customer is billed on, or argues about.
func TestTwoCoresForThreeHoursIsSixVCPUHours(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	samples := []usage.Sample{}
	// Three hours of samples, five minutes apart, at two cores.
	for minute := 0; minute <= 180; minute += 5 {
		samples = append(samples, usage.Sample{
			ProjectID: "p1", At: start.Add(time.Duration(minute) * time.Minute),
			VCPU: 2, MemoryGiB: 8,
		})
	}
	total := usage.Accumulate("p1", samples, 5*time.Minute, 30*time.Minute)

	// 37 samples, each crediting five minutes: three hours and five minutes.
	if total.VCPUHours < 6 || total.VCPUHours > 6.2 {
		t.Errorf("expected about six vCPU-hours, got %.3f", total.VCPUHours)
	}
	if total.PeakVCPU != 2 {
		t.Errorf("peak = %v, want 2", total.PeakVCPU)
	}
	if total.Samples != len(samples) {
		t.Errorf("the total must say how many samples it rests on, got %d", total.Samples)
	}
}

// A gap means the platform was not measuring. Crediting it would invoice
// somebody for an outage.
func TestAGapIsNotBilled(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	samples := []usage.Sample{
		{ProjectID: "p1", At: start, VCPU: 4},
		// The sampler was down for six hours.
		{ProjectID: "p1", At: start.Add(6 * time.Hour), VCPU: 4},
		{ProjectID: "p1", At: start.Add(6*time.Hour + 5*time.Minute), VCPU: 4},
	}
	total := usage.Accumulate("p1", samples, 5*time.Minute, 30*time.Minute)

	// Three samples of five minutes each at four cores: one hour, not 24.
	if total.VCPUHours > 1.1 {
		t.Fatalf("a six-hour gap was billed: %.2f vCPU-hours", total.VCPUHours)
	}
}

// The peak is what capacity planning needs: an average hides the hour that
// filled the cluster.
func TestThePeakSurvivesAnAverage(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	samples := []usage.Sample{
		{ProjectID: "p1", At: start, VCPU: 1, MemoryGiB: 2},
		{ProjectID: "p1", At: start.Add(5 * time.Minute), VCPU: 16, MemoryGiB: 64},
		{ProjectID: "p1", At: start.Add(10 * time.Minute), VCPU: 1, MemoryGiB: 2},
	}
	total := usage.Accumulate("p1", samples, 5*time.Minute, 30*time.Minute)
	if total.PeakVCPU != 16 || total.PeakMemoryGiB != 64 {
		t.Fatalf("the peak must survive: %+v", total)
	}
}

func TestNoSamplesIsZeroRatherThanAnError(t *testing.T) {
	total := usage.Accumulate("p1", nil, 5*time.Minute, 30*time.Minute)
	if total.VCPUHours != 0 || total.Samples != 0 {
		t.Fatalf("an empty window must total zero, got %+v", total)
	}
}

// The sampler writes one instant per sweep, and skips projects holding
// nothing: writing zeroes for every idle project every five minutes fills a
// table with the absence of information.
func TestTheSamplerRecordsWhatIsRunningAndSkipsWhatIsNot(t *testing.T) {
	h, _, item := launchFixture(t, "")
	h.usageStore = memory.NewUsageStore()

	idle := project.NewOwned("owner", "Idle project", "")
	if err := h.projectStore.Create(idle); err != nil {
		t.Fatal(err)
	}
	running := workspace.New("jupyter", item.ID, "w1", "image", "pod", "svc", "2", "8Gi", "", "token")
	running.Status = "running"
	if err := h.workspaceStore.Create(running); err != nil {
		t.Fatal(err)
	}

	h.sampleUsage()

	samples, err := h.usageStore.ListByProject(item.ID, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 || samples[0].VCPU != 2 || samples[0].Workspaces != 1 {
		t.Fatalf("the running project must be sampled once with what it holds, got %+v", samples)
	}
	idleSamples, err := h.usageStore.ListByProject(idle.ID, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(idleSamples) != 0 {
		t.Errorf("a project holding nothing must not be recorded, got %+v", idleSamples)
	}
}
