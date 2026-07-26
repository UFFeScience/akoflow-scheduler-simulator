package main

import (
	"os"
	"strconv"
	"testing"
)

// Opt-in profiling harness for the 6,448-task workflow. It is intentionally
// skipped in normal test runs.
func TestProfileLargePRISMCC(t *testing.T) {
	if os.Getenv("PROFILE_LARGE_PRISM_CC") != "1" {
		t.Skip("set PROFILE_LARGE_PRISM_CC=1 to run the large profiling harness")
	}
	beamWidth := 11
	if raw := os.Getenv("PROFILE_BEAM_WIDTH"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatal(err)
		}
		beamWidth = parsed
	}
	generated, err := generateExperimentSimulationForWorkflow(
		"hybrid_hetero", montageDSS20WorkflowID, 42, 1, false, beamWidth,
	)
	if err != nil {
		t.Fatal(err)
	}
	generated.Experimental.Algorithm = "prism_cc_time"
	generated.Experimental.PriorityPolicy = "upward_rank"
	generated.SLA.WeightTime = 1
	generated.SLA.WeightCost = 0
	if _, err := beamSearch(generated, beamWidth); err != nil {
		t.Fatal(err)
	}
}
