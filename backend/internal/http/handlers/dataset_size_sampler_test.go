package handlers

import (
	"strings"
	"testing"
)

// A partial walk is not a size.
//
// The on-demand measurement stops after a fixed number of objects and says so
// with a flag. Persisting that number as the dataset's size would publish a
// floor as a total - the platform under-reporting with confidence, which is
// the failure this whole figure has had twice already.
func TestATruncatedWalkIsRecordedAsAFailureRatherThanASize(t *testing.T) {
	source := readSourceFile(t, "dataset_size_sampler.go")
	branch := source[strings.Index(source, "case usage.Truncated:"):]
	branch = branch[:strings.Index(branch, "default:")]
	if !strings.Contains(branch, "entry.Failure") {
		t.Error("a truncated listing is not recorded as a failure")
	}
	if strings.Contains(branch, "entry.Bytes") {
		t.Error("a truncated listing writes a byte count, which reads as a total")
	}
}

// The sweep must not walk every bucket on every restart.
//
// A deployment restarts the pod two or three times in a minute. Measuring
// hundreds of gigabytes each time would turn a rollout into an object-store
// bill, and the figures are a night old by design anyway.
func TestStartupOnlyMeasuresWhenTheFiguresAreStale(t *testing.T) {
	source := readSourceFile(t, "dataset_size_sampler.go")
	start := strings.Index(source, "func (h Handlers) StartDatasetSizeSampler")
	body := source[start : strings.Index(source[start:], "func (h Handlers) datasetSizesAreStale")+start]
	if !strings.Contains(body, "h.datasetSizesAreStale()") {
		t.Error("the sampler measures at startup unconditionally")
	}
}

// One listing, not two.
//
// The sampler reuses the on-demand measurement rather than walking the bucket
// its own way. Two implementations of the same listing drift, and the one that
// drifts is always the one nobody watches.
func TestTheSamplerReusesTheOnDemandMeasurement(t *testing.T) {
	source := readSourceFile(t, "dataset_size_sampler.go")
	if !strings.Contains(source, "h.measureDataset(ctx, item)") {
		t.Error("the sampler no longer reuses the on-demand measurement")
	}
	if strings.Contains(source, "ListObjects(") {
		t.Error("the sampler walks the bucket itself, making a second implementation of the same listing")
	}
}
