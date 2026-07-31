package main

import (
	"math"
	"testing"
)

func TestHEFTGlobalNetworkAveragesAllDistinctResourcePairs(t *testing.T) {
	generated := GeneratedSimulation{
		Resources: []Resource{{ID: "a"}, {ID: "b"}, {ID: "c"}},
		Matrices: Matrices{
			BandwidthBW: map[string]map[string]float64{
				"a": {"a": 0, "b": 10, "c": 20},
				"b": {"a": 30, "b": 0, "c": 40},
				"c": {"a": 50, "b": 60, "c": 0},
			},
			TransferDelay: map[string]map[string]float64{
				"a": {"a": 0, "b": 0.01, "c": 0.02},
				"b": {"a": 0.03, "b": 0, "c": 0.04},
				"c": {"a": 0.05, "b": 0.06, "c": 0},
			},
		},
	}

	bandwidth, latency := heftGlobalNetwork(generated)
	if math.Abs(bandwidth-35) > 1e-9 {
		t.Fatalf("global bandwidth: got %v, want 35", bandwidth)
	}
	if math.Abs(latency-0.035) > 1e-9 {
		t.Fatalf("global latency: got %v, want 0.035", latency)
	}
}

func TestHEFTGlobalPredecessorTimingUsesOneNetworkForEveryRemoteMachine(t *testing.T) {
	deps := []Dependency{{Source: "parent", Target: "child", DataMB: 100}}
	assignments := map[string]Assignment{
		"parent": {TaskID: "parent", ResourceID: "a", FinishTime: 7},
	}

	floor, total := heftGlobalPredecessorTiming(deps, assignments, "b", 50, 0.1)
	if math.Abs(total-2.1) > 1e-9 || math.Abs(floor-9.1) > 1e-9 {
		t.Fatalf("remote timing: floor=%v total=%v, want 9.1 and 2.1", floor, total)
	}

	floor, total = heftGlobalPredecessorTiming(deps, assignments, "a", 50, 0.1)
	if floor != 7 || total != 0 {
		t.Fatalf("same-machine timing: floor=%v total=%v, want 7 and 0", floor, total)
	}
}

func TestHEFTObservedRunPreservesGlobalPlanPlacements(t *testing.T) {
	req := defaultRequest()
	req.Seed = 912
	req.TaskCount = 30
	req.EdgeDensity = 0.3
	generated, err := generateSimulation(req)
	if err != nil {
		t.Fatal(err)
	}

	planned, err := scheduleHEFTColocationWithGlobalNetwork(generated)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := scheduleHEFTColocation(generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Assignments) != len(observed.Assignments) {
		t.Fatalf("assignment count changed: planned=%d observed=%d", len(planned.Assignments), len(observed.Assignments))
	}
	for index := range planned.Assignments {
		want, got := planned.Assignments[index], observed.Assignments[index]
		if got.TaskID != want.TaskID || got.ResourceID != want.ResourceID {
			t.Fatalf(
				"placement %d changed: planned=%s/%s observed=%s/%s",
				index, want.TaskID, want.ResourceID, got.TaskID, got.ResourceID,
			)
		}
	}
}

func TestHEFTColocationKeepsBootedMachinesRunning(t *testing.T) {
	generated, err := generateExperimentSimulationForWorkflowAtRateAndDataScale(
		"real_network_stress_cloud_homo", imageDataflow8WorkflowID,
		42, 1, true, minBeamWidth, 0, 1,
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := scheduleHEFTColocation(generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.MachineStopIntervals) != 0 {
		t.Fatalf("HEFT must not stop machines, got %#v", result.MachineStopIntervals)
	}
	bootsByResource := map[string]int{}
	for _, assignment := range result.Assignments {
		if assignment.BootOverhead > 0 {
			bootsByResource[assignment.ResourceID]++
		}
	}
	for resourceID, boots := range bootsByResource {
		if boots != 1 {
			t.Fatalf("resource %s booted %d times, want exactly one initial boot", resourceID, boots)
		}
	}
}
