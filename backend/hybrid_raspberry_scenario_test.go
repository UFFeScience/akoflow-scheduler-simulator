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
	hpcNodes := 0
	cloudNodes := 0
	raspberrySpeedups := map[float64]bool{}
	for _, resource := range resources {
		if len(resource.ID) >= 8 && resource.ID[:8] == "hpc-node" {
			hpcNodes++
		}
		if len(resource.ID) >= 10 && resource.ID[:10] == "cloud-node" {
			cloudNodes++
		}
		if len(resource.ID) < 8 || resource.ID[:8] != "rpi-edge" {
			continue
		}
		raspberries++
		if resource.Cores != 2 || resource.Memory != 1 {
			t.Fatalf("%s does not match the requested Raspberry Pi capacity", resource.ID)
		}
		raspberrySpeedups[resource.Speedup] = true
		if resource.Speedup < 0.019 || resource.Speedup > 0.027 {
			t.Fatalf("%s speedup outside the heterogeneous fog range: %v", resource.ID, resource.Speedup)
		}
		if resource.Bandwidth < 1.25 || resource.Bandwidth > 2.5 ||
			resource.NetworkLatencyMS < 5 || resource.NetworkLatencyMS > 15 {
			t.Fatalf("%s network: got %.2f MB/s and %.2f ms", resource.ID, resource.Bandwidth, resource.NetworkLatencyMS)
		}
	}
	if raspberries != 10 {
		t.Fatalf("expected 10 Raspberry Pi resources, got %d", raspberries)
	}
	if hpcNodes != 2 || cloudNodes != 2 {
		t.Fatalf("expected 2 HPC and 2 cloud resources, got %d and %d", hpcNodes, cloudNodes)
	}
	if len(raspberrySpeedups) < 5 {
		t.Fatalf("fog layer is not sufficiently heterogeneous: %v", raspberrySpeedups)
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
	want := (15.0 + 80.0) / 2000.0
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("transfer latency: got %v s, want %v s", got, want)
	}
	if generated.Matrices.BandwidthBW["hpc-node-01"]["cloud-node-01"] != 62.5 {
		t.Fatalf("HPC/cloud backbone must be 500 Mbps in the experiment model")
	}
}

func TestLegacyExperimentMachineBandwidthIsCappedAt500Mbps(t *testing.T) {
	for _, scenario := range []string{
		"cluster_homo", "cluster_hetero", "cloud_homo", "cloud_hetero",
		"hybrid_homo", "hybrid_hetero",
	} {
		resources, err := experimentScenarioResources(scenario)
		if err != nil {
			t.Fatal(err)
		}
		for _, resource := range resources {
			if resource.Bandwidth != 62.5 {
				t.Fatalf("%s/%s bandwidth: got %.2f MB/s, want 62.5 MB/s (500 Mbps)", scenario, resource.ID, resource.Bandwidth)
			}
		}
	}
}

func TestExperimentBandwidthConversionDividesBitsByEight(t *testing.T) {
	if got := gigabitsPerSecondToMegabytesPerSecond(200); got != 25000 {
		t.Fatalf("200 Gbps: got %.2f MB/s, want 25000 MB/s", got)
	}
	if got := gigabitsPerSecondToMegabytesPerSecond(0.5); got != 62.5 {
		t.Fatalf("500 Mbps: got %.2f MB/s, want 62.5 MB/s", got)
	}
	if got := megabitsPerSecondToMegabytesPerSecond(10); got != 1.25 {
		t.Fatalf("10 Mbps: got %.2f MB/s, want 1.25 MB/s", got)
	}
}
