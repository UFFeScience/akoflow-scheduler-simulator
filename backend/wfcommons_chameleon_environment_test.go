package main

import (
	"math"
	"testing"
)

func TestWfCommonsChameleonEnvironmentMatchesObservedMachines(t *testing.T) {
	generated, err := generateExperimentSimulationForWorkflowAtRateAndDataScale(
		"wfcommons_chameleon_dss20", montageDSS20WorkflowID,
		42, 1, true, minBeamWidth, 0, 1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(generated.Resources) != 5 {
		t.Fatalf("resource count: got %d, want 5", len(generated.Resources))
	}
	expected := map[string]float64{
		"compute-3": 998.4 / 422.4,
		"compute-4": 460.8 / 422.4,
		"compute-5": 460.8 / 422.4,
		"compute-6": 568.32 / 422.4,
		"compute-7": 460.8 / 422.4,
	}
	for _, resource := range generated.Resources {
		if len(resource.Cores) != 48 {
			t.Fatalf("%s cores: got %d, want 48", resource.ID, len(resource.Cores))
		}
		if math.Abs(resource.Memory-131.80) > 1e-6 {
			t.Fatalf("%s memory: got %v, want 131.80 GB after simulator rounding", resource.ID, resource.Memory)
		}
		if math.Abs(resource.Speedup-expected[resource.ID]) > 1e-6 {
			t.Fatalf("%s speedup: got %v, want %v", resource.ID, resource.Speedup, expected[resource.ID])
		}
	}
	if got := generated.Matrices.BandwidthBW["compute-3"]["compute-4"]; got != 125 {
		t.Fatalf("bandwidth: got %v MB/s, want 125 MB/s (1 Gbps)", got)
	}
	if got := generated.Matrices.TransferDelay["compute-3"]["compute-4"]; math.Abs(got-0.0005) > 1e-9 {
		t.Fatalf("latency: got %v s, want 0.0005 s", got)
	}
}
