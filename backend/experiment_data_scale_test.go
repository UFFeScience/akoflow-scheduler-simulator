package main

import (
	"math"
	"testing"
)

func TestExperimentDataScaleMultipliesEveryDependency(t *testing.T) {
	base, err := generateExperimentSimulationForWorkflowAtRateAndDataScale(
		"cluster_homo", "montage_050d", 42, 1, true, minBeamWidth, 0, 1,
	)
	if err != nil {
		t.Fatal(err)
	}
	scaled, err := generateExperimentSimulationForWorkflowAtRateAndDataScale(
		"cluster_homo", "montage_050d", 42, 1, true, minBeamWidth, 0, 10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(base.Workflow.Dependencies) != len(scaled.Workflow.Dependencies) {
		t.Fatal("data scaling changed the workflow topology")
	}
	for index := range base.Workflow.Dependencies {
		want := base.Workflow.Dependencies[index].DataMB * 10
		got := scaled.Workflow.Dependencies[index].DataMB
		if math.Abs(got-want) > 1e-8 {
			t.Fatalf("dependency %d data scale: got %v, want %v", index, got, want)
		}
	}
}

func TestDisableContainerOverheadZerosEveryTaskResourcePair(t *testing.T) {
	generated, err := generateExperimentSimulationForWorkflowAtRateAndDataScale(
		"real_network_stress_cluster_hetero", imageDataflow8WorkflowID,
		42, 1, true, minBeamWidth, 0, 1,
	)
	if err != nil {
		t.Fatal(err)
	}
	disableContainerOverhead(&generated)
	for taskID, byResource := range generated.Matrices.ContainerOverhead {
		for resourceID, value := range byResource {
			if value != 0 {
				t.Fatalf("container overhead %s/%s: got %v, want 0", taskID, resourceID, value)
			}
		}
	}
}
