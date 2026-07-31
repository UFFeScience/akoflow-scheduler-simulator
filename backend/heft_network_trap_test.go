package main

import "testing"

func TestHEFTNetworkTrapHidesSlowCrossTierLinksBehindFastMean(t *testing.T) {
	generated, err := generateExperimentSimulationForWorkflowAtRateAndDataScale(
		"hybrid_heft_network_trap", "montage_050d", 42, 1, true, minBeamWidth, 0, 10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(generated.Resources) != 14 {
		t.Fatalf("resource count: got %d, want 14", len(generated.Resources))
	}
	if got := generated.Matrices.BandwidthBW["hefttrap-fog-01"]["hefttrap-fog-02"]; got != 250 {
		t.Fatalf("fog internal bandwidth: got %v MB/s, want 250", got)
	}
	if got := generated.Matrices.BandwidthBW["hefttrap-fog-01"]["hefttrap-cloud-01"]; got != 0.625 {
		t.Fatalf("fog-cloud bandwidth: got %v MB/s, want 0.625", got)
	}
	if got := generated.Matrices.TransferDelay["hefttrap-fog-01"]["hefttrap-cloud-01"]; got != 0.2 {
		t.Fatalf("fog-cloud latency: got %v s, want 0.2", got)
	}
	globalBandwidth, _ := heftGlobalNetwork(generated)
	if globalBandwidth < 100 {
		t.Fatalf("global HEFT bandwidth should hide slow cross-tier links, got %v MB/s", globalBandwidth)
	}
}
