package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

const akoflowYAML = `
name: wf-synthetic-dag
spec:
  runtime: hpcplafrim
  image: /home/wferreir/wf-synthetic-tar/sifs/akoflow-wf-montage_050d.sif
  activities:
    - name: mprojectid0000001
      runtime: hpcplafrim
      run: mv -fvu /akoflow-wfa-shared/* . && mProject -X a.fits pa.fits region.hdr
      memoryLimit: 256M
      cpuLimit: 1
    - name: mprojectid0000002
      runtime: hpcplafrim
      run: mv -fvu /akoflow-wfa-shared/* . && mProject -X b.fits pb.fits region.hdr
      memoryLimit: 256M
      cpuLimit: 1
    - name: mdifffitid0000005
      runtime: hpcplafrim
      run: mDiffFit -d -s fit.txt pa.fits pb.fits diff.fits region.hdr
      memoryLimit: 256M
      cpuLimit: 1
      dependsOn:
        - mprojectid0000001
        - mprojectid0000002
`

func defaultGenerated(t *testing.T) GeneratedSimulation {
	t.Helper()
	req := defaultRequest()
	req.TaskCount = 14
	req.EdgeDensity = 0.3
	generated, err := generateSimulation(req)
	if err != nil {
		t.Fatal(err)
	}
	return generated
}

func TestGeneratedWorkflowIsDAG(t *testing.T) {
	generated := defaultGenerated(t)
	positions := map[string]int{}
	for i, task := range generated.Workflow.Tasks {
		positions[task.ID] = i
	}
	for _, dep := range generated.Workflow.Dependencies {
		if positions[dep.Source] >= positions[dep.Target] {
			t.Fatalf("dependency is not ordered: %+v", dep)
		}
	}
}

func TestSeededGenerationIsDeterministic(t *testing.T) {
	req := defaultRequest()
	req.Seed = 99
	req.TaskCount = 10
	first, err := generateSimulation(req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := generateSimulation(req)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	if string(firstJSON) != string(secondJSON) {
		t.Fatal("generation is not deterministic")
	}
}

func TestAkoflowYAMLActivitiesBuildWorkflowDAG(t *testing.T) {
	req := defaultRequest()
	req.Seed = 12
	req.WorkflowYAML = ptr(akoflowYAML)
	generated, err := generateSimulation(req)
	if err != nil {
		t.Fatal(err)
	}
	result, err := scheduleWorkflow(generated)
	if err != nil {
		t.Fatal(err)
	}
	gotIDs := []string{}
	for _, task := range generated.Workflow.Tasks {
		gotIDs = append(gotIDs, task.ID)
	}
	wantIDs := []string{"mprojectid0000001", "mprojectid0000002", "mdifffitid0000005"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("tasks = %#v", gotIDs)
	}
	if !reflect.DeepEqual(generated.Workflow.PredecessorSets["mdifffitid0000005"], wantIDs[:2]) {
		t.Fatalf("predecessor set = %#v", generated.Workflow.PredecessorSets["mdifffitid0000005"])
	}
	if generated.Workflow.Tasks[0].WorkflowStage != "mProject" || generated.Workflow.Tasks[2].WorkflowStage != "mDiffFit" {
		t.Fatalf("unexpected stages: %s %s", generated.Workflow.Tasks[0].WorkflowStage, generated.Workflow.Tasks[2].WorkflowStage)
	}
	if generated.Workflow.Tasks[0].CPU != 1 || generated.Workflow.Tasks[0].Memory != 0.25 {
		t.Fatalf("unexpected limits: cpu=%v memory=%v", generated.Workflow.Tasks[0].CPU, generated.Workflow.Tasks[0].Memory)
	}
	if len(result.Assignments) != len(generated.Workflow.Tasks) {
		t.Fatal("not every YAML task was assigned")
	}
}

func TestResourceSpecsAndCloudSemantics(t *testing.T) {
	req := defaultRequest()
	req.Seed = 7
	req.ResourceSpecs = []ResourceSpec{
		{ID: "c1", Name: "cluster-large", Kind: "cluster", Cores: 3, Memory: 64, Bandwidth: 1200, Location: "on-prem"},
		{ID: "v1", Name: "cloud-fast", Kind: "cloud", Cores: 4, Memory: 96, Bandwidth: 500, BootOverhead: 15, Location: "eu-west"},
	}
	generated, err := generateSimulation(req)
	if err != nil {
		t.Fatal(err)
	}
	resources := resourceMap(generated.Resources)
	if resources["c1"].PricePerCPUSecond != 0 || resources["c1"].PricePerGBSecond != 0 || resources["c1"].FinancialNetworkPrice != 0 {
		t.Fatal("cluster resource should have owned-capacity pricing")
	}
	if resources["v1"].Status != "cold" || resources["v1"].BootOverhead != 15 {
		t.Fatalf("cloud status/boot mismatch: %+v", resources["v1"])
	}
	if len(resources["c1"].Cores) != 3 || resources["c1"].CPU != 3 || resources["v1"].CPU != 4 {
		t.Fatal("resource core capacity mismatch")
	}
	if generated.Matrices.BandwidthBW["c1"]["v1"] != 500 {
		t.Fatalf("bandwidth mismatch: %v", generated.Matrices.BandwidthBW["c1"]["v1"])
	}
}

func TestSchedulerAssignsEveryTaskAndRespectsConstraints(t *testing.T) {
	generated := defaultGenerated(t)
	result, err := scheduleWorkflow(generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assignments) != len(generated.Workflow.Tasks) {
		t.Fatalf("assignments=%d tasks=%d", len(result.Assignments), len(generated.Workflow.Tasks))
	}
	resources := resourceMap(result.Resources)
	tasks := taskMap(result.Workflow.Tasks)
	seen := map[string]bool{}
	assignmentByTask := map[string]Assignment{}
	previousOnCore := map[string]float64{}
	for _, assignment := range result.Assignments {
		if seen[assignment.TaskID] {
			t.Fatalf("duplicate assignment for %s", assignment.TaskID)
		}
		seen[assignment.TaskID] = true
		assignmentByTask[assignment.TaskID] = assignment
		if assignment.StartTime < previousOnCore[assignment.CoreID] {
			t.Fatalf("core availability violated by %+v", assignment)
		}
		previousOnCore[assignment.CoreID] = assignment.FinishTime
		task := tasks[assignment.TaskID]
		resource := resources[assignment.ResourceID]
		if task.CPU > resource.CPU || task.Memory > resource.Memory {
			t.Fatalf("infeasible assignment: task=%+v resource=%+v", task, resource)
		}
	}
	for _, assignment := range result.Assignments {
		for _, dep := range generated.Workflow.Dependencies {
			if dep.Target != assignment.TaskID {
				continue
			}
			predecessor := assignmentByTask[dep.Source]
			transfer := 0.0
			if predecessor.ResourceID != assignment.ResourceID {
				transfer = dep.DataMB / generated.Matrices.BandwidthBW[predecessor.ResourceID][assignment.ResourceID]
			}
			if assignment.StartTime < predecessor.FinishTime+transfer {
				t.Fatalf("predecessor timing violated for %s", assignment.TaskID)
			}
		}
	}
}

func TestCostInterferenceAndDeviationAreConsistent(t *testing.T) {
	generated := defaultGenerated(t)
	result, err := scheduleWorkflow(generated)
	if err != nil {
		t.Fatal(err)
	}
	maxFinish := 0.0
	for _, finish := range result.TimingVariables.FT {
		maxFinish = maxf(maxFinish, finish)
	}
	if result.TimingVariables.Makespan != maxFinish {
		t.Fatalf("makespan mismatch: %v != %v", result.TimingVariables.Makespan, maxFinish)
	}
	sumFC := 0.0
	for _, value := range result.CostVariables.FC {
		sumFC += value
	}
	if result.CostVariables.BUsed != round(sumFC, 4) {
		t.Fatalf("budget mismatch: %v", result.CostVariables.BUsed)
	}
	totalInterference := 0.0
	for _, assignment := range result.Assignments {
		base := generated.Matrices.ET0[assignment.TaskID][assignment.ResourceID]
		totalInterference += maxf(0, assignment.EffectiveRuntime-base)
		variance := round(result.DeviationVariables.ETObs[assignment.TaskID]-base, 3)
		if result.DeviationVariables.Var[assignment.TaskID] != variance {
			t.Fatalf("variance mismatch for %s", assignment.TaskID)
		}
	}
	if result.InterferenceVariables.TotalInterferenceTime != round(totalInterference, 3) {
		t.Fatalf("interference mismatch: %v", result.InterferenceVariables.TotalInterferenceTime)
	}
}

func TestBeamOptimizerReturnsOptionsAndRecommendedSelection(t *testing.T) {
	req := defaultRequest()
	req.TaskCount = 8
	req.EdgeDensity = 0.3
	req.BudgetLimit = ptr(9999.0)
	req.DeadlineLimit = ptr(9999.0)
	req.OptionCount = 5
	generated, err := generateSimulation(req)
	if err != nil {
		t.Fatal(err)
	}
	response, err := optimizeSchedule(generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Options) == 0 || len(response.Options) > req.OptionCount {
		t.Fatalf("unexpected option count: %d", len(response.Options))
	}
	if response.SelectedOptionID == nil || *response.SelectedOptionID != response.Options[0].ID || !response.Options[0].Recommended {
		t.Fatal("first option should be selected and recommended")
	}
	signatures := map[string]bool{}
	for _, option := range response.Options {
		if !option.Feasible || option.BudgetViolation != 0 || option.DeadlineViolation != 0 {
			t.Fatalf("unexpected infeasible option: %+v", option)
		}
		if option.MachineDistribution == nil || len(option.MachineDistribution) == 0 {
			t.Fatal("missing machine distribution")
		}
		if signatures[option.MachineSignature] {
			t.Fatalf("duplicate option signature: %s", option.MachineSignature)
		}
		signatures[option.MachineSignature] = true
	}
}

func TestValidationRejectsOptionCountAboveMaximum(t *testing.T) {
	req := defaultRequest()
	req.OptionCount = 101
	if err := validateRequest(req); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestScheduleResponseJSONContainsFrontendFields(t *testing.T) {
	generated := defaultGenerated(t)
	response, err := optimizeSchedule(generated)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"selected_option_id", "constraints", "options"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("missing %s", key)
		}
	}
	options := decoded["options"].([]any)
	first := options[0].(map[string]any)
	for _, key := range []string{"machine_distribution", "weighted_score", "result"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("missing option field %s", key)
		}
	}
	result := first["result"].(map[string]any)
	for _, key := range []string{"scheduler_steps", "scheduler_variables", "timing_variables", "cost_variables", "interference_variables", "deviation_variables"} {
		if _, ok := result[key]; !ok {
			t.Fatalf("missing result field %s", key)
		}
	}
}

func ptr[T any](value T) *T {
	return &value
}
