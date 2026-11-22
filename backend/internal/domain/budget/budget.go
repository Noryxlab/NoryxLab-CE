// Package budget is what a project may consume in a month, and what happens
// as it gets there.
//
// The unit is vCPU-hours, not money, and that is a decision rather than a
// shortcut. Domino budgets in currency because it bills a cloud account; this
// platform runs on hardware somebody already owns, so a price per hour would
// be a number the product invented. vCPU-hours are sampled every five minutes
// by the sampler that has been running since 2026-09-06 - the only figure
// here that is true without a conversion.
//
// An installation that wants currency sets a price per vCPU-hour, and the
// same budget reads in euros with the conversion visible rather than built in.
package budget

import "time"

// Budget is one project's monthly allowance.
type Budget struct {
	ProjectID string `json:"projectId"`
	// VCPUHours is the allowance for a calendar month. Zero means no budget,
	// which is the default: a budget nobody set must never refuse anything.
	VCPUHours float64   `json:"vcpuHours"`
	UpdatedBy string    `json:"updatedBy,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Thresholds are the two points at which somebody is told, in percent.
//
// Two, not five: an alert that arrives often is an alert nobody reads. The
// first is early enough to change course, the second is the fact itself.
var Thresholds = []int{75, 100}

// Reached returns the highest threshold this consumption has crossed, or zero.
func Reached(consumed, allowance float64) int {
	if allowance <= 0 || consumed <= 0 {
		return 0
	}
	percent := consumed / allowance * 100
	highest := 0
	for _, threshold := range Thresholds {
		if percent >= float64(threshold) {
			highest = threshold
		}
	}
	return highest
}

// State is what a screen shows, and what the watcher compares against what it
// has already announced.
type State struct {
	ProjectID   string  `json:"projectId"`
	ProjectName string  `json:"projectName,omitempty"`
	VCPUHours   float64 `json:"vcpuHours"`
	Consumed    float64 `json:"consumed"`
	// Percent is reported even with no budget set: consumption is worth
	// seeing before anybody decides what the limit should be.
	Percent float64 `json:"percent"`
	// Period is the first day of the month this covers.
	Period time.Time `json:"period"`
}
