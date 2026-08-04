package main

import (
	"math"
	"testing"
)

func TestNetworkCriticalScenarioMatrices(t *testing.T) {
	tests := []struct {
		scenario, left, right string
		bandwidth, latency    float64
	}{
		{"network_hpc_local", "hpc-local-01", "hpc-local-02", 25000, 0.001},
		{"network_hpc_multisite", "hpc-a-01", "hpc-b-01", 62.5, 0.020},
		{"network_cloud_multiregion", "cloud-a-01", "cloud-a-02", 1250, 0.002},
		{"network_cloud_multiregion", "cloud-a-01", "cloud-b-01", 62.5, 0.100},
		{"network_hpc_cloud", "hpc-hybrid-01", "cloud-hybrid-01", 62.5, 0.080},
		{"network_edge_cloud", "edge-data-01", "cloud-edge-01", 62.5, 0.120},
		{"network_fog_hpc_cloud", "edge-fog-01", "hpc-fog-01", 62.5, 0.040},
		{"network_wfcommons_overlay", "compute-3", "compute-5", 62.5, 0.020},
	}
	for _, test := range tests {
		generated, err := generateExperimentSimulationForWorkflowAtRateAndDataScale(
			test.scenario, "montage_050d", 42, 1, true, minBeamWidth, 0, 1,
		)
		if err != nil {
			t.Fatalf("%s: %v", test.scenario, err)
		}
		if got := generated.Matrices.BandwidthBW[test.left][test.right]; math.Abs(got-test.bandwidth) > 1e-9 {
			t.Fatalf("%s bandwidth %s->%s: got %v, want %v", test.scenario, test.left, test.right, got, test.bandwidth)
		}
		if got := generated.Matrices.TransferDelay[test.left][test.right]; math.Abs(got-test.latency) > 1e-9 {
			t.Fatalf("%s latency %s->%s: got %v, want %v", test.scenario, test.left, test.right, got, test.latency)
		}
	}
}

func TestNetworkCriticalTransferUsesFileSizeBandwidthAndLatency(t *testing.T) {
	seconds := 10000.0/62.5 + 0.120
	if math.Abs(seconds-160.120) > 1e-9 {
		t.Fatalf("10 GB over 500 Mbps with 120 ms: got %v, want 160.120", seconds)
	}
}
