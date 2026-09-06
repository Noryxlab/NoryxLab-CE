// Package usage records what each project actually consumed.
//
// The platform could say what a project is allowed to run and what it is
// running now. It could not say what it ran last month, which is the question
// behind every conversation about cost, capacity and who pays for what -
// Databricks and Domino both sell on that answer.
//
// Consumption is sampled rather than derived from lifecycles. A workspace that
// is stopped by a node failure leaves no end timestamp, and integrating over
// records with missing ends produces a number that is confidently wrong. A
// sample says only "at this instant, this project held this much", and a
// missed sample costs one interval rather than a whole run.
package usage

import "time"

// Sample is what one project held at one instant.
type Sample struct {
	ProjectID  string    `json:"projectId"`
	At         time.Time `json:"at"`
	VCPU       float64   `json:"vcpu"`
	MemoryGiB  float64   `json:"memoryGib"`
	Workspaces int       `json:"workspaces"`
	Jobs       int       `json:"jobs"`
}

// Total is consumption over a window, in the units a bill is written in.
type Total struct {
	ProjectID string    `json:"projectId"`
	From      time.Time `json:"from"`
	To        time.Time `json:"to"`
	// VCPUHours is the integral of vCPU over time: two cores for three hours
	// is six. It is the number that means something across machines of
	// different sizes.
	VCPUHours      float64 `json:"vcpuHours"`
	MemoryGiBHours float64 `json:"memoryGibHours"`
	// Samples says how many measurements this total rests on, so a number
	// built from three samples cannot be mistaken for one built from a month
	// of them.
	Samples int `json:"samples"`
	// PeakVCPU is what capacity planning needs: an average hides the hour that
	// filled the cluster.
	PeakVCPU      float64 `json:"peakVcpu"`
	PeakMemoryGiB float64 `json:"peakMemoryGib"`
}

// Accumulate turns samples into a total. Each sample is credited with the
// interval it represents, capped so a gap - a restart, a missed sweep - cannot
// be billed as if the platform had been running all along.
func Accumulate(projectID string, samples []Sample, interval, maxGap time.Duration) Total {
	total := Total{ProjectID: projectID}
	if len(samples) == 0 {
		return total
	}
	total.From = samples[0].At
	total.To = samples[len(samples)-1].At

	previous := time.Time{}
	for _, sample := range samples {
		span := interval
		if !previous.IsZero() {
			actual := sample.At.Sub(previous)
			// A gap longer than maxGap means the sampler was not running.
			// Crediting it would invent consumption nobody had.
			if actual > 0 && actual <= maxGap {
				span = actual
			}
		}
		hours := span.Hours()
		total.VCPUHours += sample.VCPU * hours
		total.MemoryGiBHours += sample.MemoryGiB * hours
		if sample.VCPU > total.PeakVCPU {
			total.PeakVCPU = sample.VCPU
		}
		if sample.MemoryGiB > total.PeakMemoryGiB {
			total.PeakMemoryGiB = sample.MemoryGiB
		}
		total.Samples++
		previous = sample.At
	}
	return total
}
