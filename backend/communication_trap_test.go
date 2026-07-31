package main

import (
	"math"
	"testing"
)

func TestCommunicationTrapScenario(t *testing.T) {
	generated, err := generateExperimentSimulationForWorkflowAtRate(
		"hybrid_communication_trap", "montage_050d", 42, 1, true, minBeamWidth, 0,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(generated.Resources) != 14 {
		t.Fatalf("expected 14 resources, got %d", len(generated.Resources))
	}
	for _, resource := range generated.Resources {
		if communicationTrapTier(resource.ID) == "hpc" && resource.PricePerHourUSD != 0 {
			t.Fatalf("%s HPC price: got %v, want zero direct charge", resource.ID, resource.PricePerHourUSD)
		}
	}
	assertLink := func(left, right string, wantBandwidth, wantLatency float64) {
		t.Helper()
		if got := generated.Matrices.BandwidthBW[left][right]; math.Abs(got-wantBandwidth) > 1e-9 {
			t.Fatalf("%s -> %s bandwidth: got %v, want %v", left, right, got, wantBandwidth)
		}
		if got := generated.Matrices.TransferDelay[left][right]; math.Abs(got-wantLatency) > 1e-9 {
			t.Fatalf("%s -> %s latency: got %v, want %v", left, right, got, wantLatency)
		}
	}
	assertLink("trap-fog-01", "trap-fog-02", 93.75, 0.010)
	assertLink("trap-fog-01", "trap-hpc-01", 12.5, 0.030)
	assertLink("trap-hpc-01", "trap-cloud-01", 25, 0.080)
	assertLink("trap-fog-01", "trap-cloud-01", 6.25, 0.130)

	baseline, err := generateExperimentSimulationForWorkflowAtRate(
		"hybrid_raspberry_500mbps", "montage_050d", 42, 1, true, minBeamWidth, 0,
	)
	if err != nil {
		t.Fatal(err)
	}
	if generated.Workflow.Dependencies[0].DataMB != round(baseline.Workflow.Dependencies[0].DataMB*25, 9) {
		t.Fatalf("communication-trap dependency scaling was not applied")
	}
}
