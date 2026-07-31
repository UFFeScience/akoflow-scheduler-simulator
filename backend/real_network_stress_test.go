package main

import "testing"

func TestRealNetworkStressFamilyReusesSevenRealMachineEnvironments(t *testing.T) {
	scenarios := map[string]int{
		"real_network_stress_cluster_homo":             4,
		"real_network_stress_cluster_hetero":           4,
		"real_network_stress_cloud_homo":               4,
		"real_network_stress_cloud_hetero":             4,
		"real_network_stress_hybrid_homo":              4,
		"real_network_stress_hybrid_hetero":            4,
		"real_network_stress_hybrid_raspberry_500mbps": 14,
	}
	for scenarioID, expectedResources := range scenarios {
		generated, err := generateExperimentSimulationForWorkflowAtRateAndDataScale(
			scenarioID, "montage_050d", 42, 1, true, minBeamWidth, 0, 10,
		)
		if err != nil {
			t.Fatalf("%s: %v", scenarioID, err)
		}
		if len(generated.Resources) != expectedResources {
			t.Fatalf("%s resources: got %d, want %d", scenarioID, len(generated.Resources), expectedResources)
		}
	}
}

func TestRealNetworkStressUsesPublishedInternalAndExternalLimits(t *testing.T) {
	cluster, err := generateExperimentSimulationForWorkflowAtRateAndDataScale(
		"real_network_stress_cluster_homo", "montage_050d", 42, 1, true, minBeamWidth, 0, 10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := cluster.Matrices.BandwidthBW["bora001"]["bora002"]; got != 25000 {
		t.Fatalf("HPC local bandwidth: got %v MB/s, want 25000", got)
	}
	if got := cluster.Matrices.BandwidthBW["bora001"]["bora003"]; got != 125 {
		t.Fatalf("HPC external bandwidth: got %v MB/s, want 125", got)
	}

	fog, err := generateExperimentSimulationForWorkflowAtRateAndDataScale(
		"real_network_stress_hybrid_raspberry_500mbps", "montage_050d", 42, 1, true, minBeamWidth, 0, 10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := fog.Matrices.BandwidthBW["rpi-edge-01"]["rpi-edge-02"]; got != 12.5 {
		t.Fatalf("Raspberry Ethernet bandwidth: got %v MB/s, want 12.5", got)
	}
	if got := fog.Matrices.BandwidthBW["rpi-edge-01"]["cloud-node-01"]; got != 4.375 {
		t.Fatalf("Fog-cloud bandwidth: got %v MB/s, want 4.375", got)
	}
}
