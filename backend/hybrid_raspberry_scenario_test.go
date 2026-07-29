package main

import (
	"math"
	"testing"
)

func TestHybridRaspberryScenario(t *testing.T) {
	resources, err := experimentScenarioResources("hybrid_raspberry_500mbps")
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 14 {
		t.Fatalf("expected 14 resources, got %d", len(resources))
	}
	raspberries := 0
	for _, resource := range resources {
		if len(resource.ID) < 8 || resource.ID[:8] != "rpi-edge" {
			continue
		}
		raspberries++
		if resource.Cores != 2 || resource.Memory != 1 {
			t.Fatalf("%s does not match the requested Raspberry Pi capacity", resource.ID)
		}
		if math.Abs(resource.Speedup-0.023) > 1e-9 {
			t.Fatalf("%s speedup: got %v, want 0.023", resource.ID, resource.Speedup)
		}
		if resource.Bandwidth != 20 || resource.NetworkLatencyMS != 10 {
			t.Fatalf("%s network: got %.2f MB/s and %.2f ms", resource.ID, resource.Bandwidth, resource.NetworkLatencyMS)
		}
	}
	if raspberries != 10 {
		t.Fatalf("expected 10 Raspberry Pi resources, got %d", raspberries)
	}
}

func TestHybridRaspberryTransferIncludesLatency(t *testing.T) {
	generated, err := generateExperimentSimulationForWorkflowAtRate(
		"hybrid_raspberry_500mbps", "montage_050d", 42, 1, true, minBeamWidth, 0,
	)
	if err != nil {
		t.Fatal(err)
	}
	got := generated.Matrices.TransferDelay["rpi-edge-01"]["cloud-node-01"]
	want := (10.0 + 100.0) / 2000.0
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("transfer latency: got %v s, want %v s", got, want)
	}
	if generated.Matrices.BandwidthBW["fog-node-01"]["cloud-node-01"] != 500 {
		t.Fatalf("fog/cloud backbone must be 500 Mbps in the experiment model")
	}
}

func TestExperimentClusterBandwidthIsCappedAt500Mbps(t *testing.T) {
	for _, scenario := range []string{"cluster_homo", "cluster_hetero", "hybrid_homo", "hybrid_hetero"} {
		resources, err := experimentScenarioResources(scenario)
		if err != nil {
			t.Fatal(err)
		}
		for _, resource := range resources {
			if resource.Kind == "cluster" && resource.Bandwidth != 500 {
				t.Fatalf("%s/%s bandwidth: got %.2f, want 500", scenario, resource.ID, resource.Bandwidth)
			}
		}
	}
}
