package main

import "testing"

func TestImageDataflowWorkflowMatchesTheEightTaskDiagram(t *testing.T) {
	generated, err := generateExperimentSimulationForWorkflowAtRateAndDataScale(
		"real_network_stress_hybrid_hetero", imageDataflow8WorkflowID,
		42, 1, true, minBeamWidth, 0, 1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(generated.Workflow.Tasks) != 8 {
		t.Fatalf("task count: got %d, want 8", len(generated.Workflow.Tasks))
	}
	expectedData := map[string]float64{
		"t0\x00t1": 10000,
		"t1\x00t2": 10000,
		"t2\x00t6": 40000,
		"t3\x00t4": 10000,
		"t4\x00t6": 10000,
		"t5\x00t7": 20000,
		"t7\x00t6": 10000,
	}
	if len(generated.Workflow.Dependencies) != len(expectedData) {
		t.Fatalf("dependency count: got %d, want %d", len(generated.Workflow.Dependencies), len(expectedData))
	}
	for _, dependency := range generated.Workflow.Dependencies {
		key := dependency.Source + "\x00" + dependency.Target
		if got, want := dependency.DataMB, expectedData[key]; got != want {
			t.Fatalf("%s -> %s data: got %v MB, want %v MB", dependency.Source, dependency.Target, got, want)
		}
	}
}
