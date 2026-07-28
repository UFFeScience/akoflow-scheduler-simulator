package main

import (
	"encoding/csv"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

func TestMontageExperimentUsesExactWorkflowAndRuntimes(t *testing.T) {
	generated, err := generateExperimentSimulation("cloud_homo", 42, 1, false, minBeamWidth)
	if err != nil {
		t.Fatal(err)
	}
	if len(generated.Workflow.Tasks) != 58 {
		t.Fatalf("expected 58 Montage tasks, got %d", len(generated.Workflow.Tasks))
	}
	rows, err := readMontageRuntimes()
	if err != nil {
		t.Fatal(err)
	}
	runtimeByID := map[string]float64{}
	for _, row := range rows {
		runtimeByID[row.ActivityID] = row.ET0Seconds
	}
	for _, task := range generated.Workflow.Tasks {
		if task.BaseRuntime != runtimeByID[task.ID] {
			t.Fatalf("runtime mismatch for %s: got %v, want %v", task.ID, task.BaseRuntime, runtimeByID[task.ID])
		}
	}
	if len(generated.Workflow.Dependencies) == 0 {
		t.Fatal("expected dependencies imported from the Montage YAML")
	}
}

func TestExperimentScenariosHaveFourMachinesAndUpdatedSpeedups(t *testing.T) {
	if err := validateExperimentScenarios(); err != nil {
		t.Fatal(err)
	}
	expected := map[string]float64{
		"bora001": 7.09, "diablo01": 5.68, "h3-standard-88-1": 20.00, "h4d-standard-192-1": 39.27,
	}
	expectedPrices := map[string]float64{
		"bora001": 0, "diablo01": 0, "h3-standard-88-1": 4.923600, "h4d-standard-192-1": 7.853760,
	}
	rows, err := readExperimentMachines()
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]float64{}
	for _, row := range rows {
		if value, ok := expected[row.MachineID]; ok {
			found[row.MachineID] = row.Speedup
			if row.Speedup != value {
				t.Fatalf("speedup mismatch for %s: got %v, want %v", row.MachineID, row.Speedup, value)
			}
			if row.PricePerHourUSD != expectedPrices[row.MachineID] {
				t.Fatalf("price mismatch for %s: got %v, want %v", row.MachineID, row.PricePerHourUSD, expectedPrices[row.MachineID])
			}
		}
	}
	if len(found) != len(expected) {
		t.Fatalf("did not load all representative machines: %v", found)
	}
}

func TestHomogeneousScenarioUsesIdenticalET0ForEveryMachine(t *testing.T) {
	generated, err := generateExperimentSimulation("cluster_homo", 42, 1, false, minBeamWidth)
	if err != nil {
		t.Fatal(err)
	}
	if len(generated.Resources) == 0 {
		t.Fatal("expected resources in homogeneous scenario")
	}
	speedup := generated.Resources[0].Speedup
	for _, resource := range generated.Resources[1:] {
		if resource.Speedup != speedup {
			t.Fatalf("scenario is not homogeneous: %s speedup=%v, want %v", resource.ID, resource.Speedup, speedup)
		}
	}
	for _, task := range generated.Workflow.Tasks {
		want := round(task.BaseRuntime/speedup, 6)
		for _, resource := range generated.Resources {
			got := generated.Matrices.ET0[task.ID][resource.ID]
			if got != want {
				t.Fatalf("ET0 mismatch for task %s on %s: got %v, want %v", task.ID, resource.ID, got, want)
			}
		}
	}
}

func TestET0IsDerivedOnlyFromRuntimeAndResourceSpeedup(t *testing.T) {
	generated, err := generateExperimentSimulation("cluster_hetero", 42, 1, false, minBeamWidth)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range generated.Workflow.Tasks {
		for _, resource := range generated.Resources {
			want := round(task.BaseRuntime/resource.Speedup, 6)
			got := generated.Matrices.ET0[task.ID][resource.ID]
			if got != want {
				t.Fatalf("ET0 mismatch for task %s on %s: got %v, want %v", task.ID, resource.ID, got, want)
			}
		}
	}
}

func TestControlledInterferenceSelectionIsPairedAndReproducible(t *testing.T) {
	first, err := generateExperimentSimulation("cluster_homo", 42, 7, false, minBeamWidth)
	if err != nil {
		t.Fatal(err)
	}
	second, err := generateExperimentSimulation("cloud_hetero", 42, 7, false, minBeamWidth)
	if err != nil {
		t.Fatal(err)
	}
	third, err := generateExperimentSimulation("cloud_hetero", 42, 8, false, minBeamWidth)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Experimental.InterferenceActivityIDs) != 29 {
		t.Fatalf("expected 29 selected activities, got %d", len(first.Experimental.InterferenceActivityIDs))
	}
	if !reflect.DeepEqual(first.Experimental.InterferenceActivityIDs, second.Experimental.InterferenceActivityIDs) {
		t.Fatal("same interference seed must select the same activities in every scenario")
	}
	if reflect.DeepEqual(first.Experimental.InterferenceActivityIDs, third.Experimental.InterferenceActivityIDs) {
		t.Fatal("different interference seeds should select a different activity sample")
	}
}

func TestControlledInterferenceAccumulatesTwentyPercentPerOverlappingTask(t *testing.T) {
	generated, err := generateExperimentSimulation("cluster_homo", 42, 1, false, minBeamWidth)
	if err != nil {
		t.Fatal(err)
	}
	selected := generated.Experimental.InterferenceActivityIDs
	resourceID := generated.Resources[0].ID
	scheduled := []Assignment{
		{TaskID: selected[0], ResourceID: resourceID, StartTime: 0, FinishTime: 100},
		{TaskID: selected[1], ResourceID: resourceID, StartTime: 0, FinishTime: 100},
		{TaskID: selected[2], ResourceID: resourceID, StartTime: 0, FinishTime: 100},
	}
	phi, pairs := candidatePairwiseInterference(generated, selected[3], resourceID, scheduled, 10, 20)
	if phi != 0.6 || len(pairs) != 3 {
		t.Fatalf("expected cumulative phi=0.6 from 3 pairs, got phi=%v pairs=%d", phi, len(pairs))
	}
	phi, _ = candidatePairwiseInterference(generated, selected[3], generated.Resources[1].ID, scheduled, 10, 20)
	if phi != 0 {
		t.Fatalf("different machines must not interfere, got %v", phi)
	}
}

func TestClassicHEFTProducesDependencyRespectingNonColocatedSchedule(t *testing.T) {
	generated, err := generateExperimentSimulation("hybrid_hetero", 42, 0, true, minBeamWidth)
	if err != nil {
		t.Fatal(err)
	}
	result, err := scheduleHEFTClassic(generated)
	if err != nil {
		t.Fatal(err)
	}
	assignments := map[string]Assignment{}
	for _, assignment := range result.Assignments {
		assignments[assignment.TaskID] = assignment
	}
	for _, dependency := range result.Workflow.Dependencies {
		source := assignments[dependency.Source]
		target := assignments[dependency.Target]
		if target.StartTime < source.FinishTime {
			t.Fatalf("dependency violation %s -> %s: %v < %v", dependency.Source, dependency.Target, target.StartTime, source.FinishTime)
		}
	}
	for i, left := range result.Assignments {
		for _, right := range result.Assignments[i+1:] {
			if left.ResourceID == right.ResourceID &&
				maxf(left.StartTime, right.StartTime) < minf(left.FinishTime, right.FinishTime) {
				t.Fatalf("classic HEFT colocated %s and %s on %s", left.TaskID, right.TaskID, left.ResourceID)
			}
		}
	}
	if result.InterferenceVariables.TotalInterferenceTime != 0 {
		t.Fatalf("classic HEFT must not activate interference, got %v", result.InterferenceVariables.TotalInterferenceTime)
	}
}

func TestHEFTColocationImplementationIsPreserved(t *testing.T) {
	generated, err := generateExperimentSimulation("hybrid_hetero", 42, 1, false, minBeamWidth)
	if err != nil {
		t.Fatal(err)
	}
	result, err := scheduleHEFTColocation(generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assignments) != 58 {
		t.Fatalf("expected preserved HEFT-colocation to schedule 58 tasks, got %d", len(result.Assignments))
	}
}

func TestClassicHEFTIsIndependentOfInterferenceSeed(t *testing.T) {
	firstGenerated, err := generateExperimentSimulation("cloud_hetero", 42, 1, false, minBeamWidth)
	if err != nil {
		t.Fatal(err)
	}
	secondGenerated, err := generateExperimentSimulation("cloud_hetero", 42, 2, false, minBeamWidth)
	if err != nil {
		t.Fatal(err)
	}
	first, err := scheduleHEFTClassic(firstGenerated)
	if err != nil {
		t.Fatal(err)
	}
	second, err := scheduleHEFTClassic(secondGenerated)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Assignments, second.Assignments) {
		t.Fatal("classic HEFT schedule must not change with the interference seed")
	}
}

func TestPRISMCCUpwardRankOrderRespectsDependencies(t *testing.T) {
	generated, err := generateExperimentSimulation("hybrid_hetero", 42, 1, false, minBeamWidth)
	if err != nil {
		t.Fatal(err)
	}
	generated.Experimental.PriorityPolicy = "upward_rank"
	order, err := prismCCPriorityOrder(generated)
	if err != nil {
		t.Fatal(err)
	}
	position := map[string]int{}
	for index, taskID := range order {
		position[taskID] = index
	}
	for _, dependency := range generated.Workflow.Dependencies {
		if position[dependency.Source] >= position[dependency.Target] {
			t.Fatalf("upward-rank order violates %s -> %s", dependency.Source, dependency.Target)
		}
	}
	topological, err := topologicalOrder(generated)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(order, topological) {
		t.Fatal("expected upward-rank order to differ from the lexicographic topological order")
	}
}

func TestMontageDSS20WorkflowLoadsExactWfCommonsDAGAndNormalizedRuntimes(t *testing.T) {
	generated, err := generateExperimentSimulationForWorkflow(
		"cloud_hetero", montageDSS20WorkflowID, 42, 7, false, minBeamWidth,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(generated.Workflow.Tasks) != 6448 {
		t.Fatalf("expected 6448 DSS 20d tasks, got %d", len(generated.Workflow.Tasks))
	}
	if len(generated.Workflow.Dependencies) != 18924 {
		t.Fatalf("expected 18924 DSS 20d dependencies, got %d", len(generated.Workflow.Dependencies))
	}
	if len(generated.Experimental.InterferenceActivityIDs) != 3224 {
		t.Fatalf("expected 3224 selected interference tasks, got %d", len(generated.Experimental.InterferenceActivityIDs))
	}
	roots, leaves := 0, 0
	for _, task := range generated.Workflow.Tasks {
		if len(task.Predecessors) == 0 {
			roots++
		}
		if len(task.Successors) == 0 {
			leaves++
		}
		if task.BaseRuntime <= 0 {
			t.Fatalf("task %s has invalid normalized runtime %v", task.ID, task.BaseRuntime)
		}
	}
	if roots != 192 || leaves != 4 {
		t.Fatalf("unexpected DSS 20d topology: roots=%d leaves=%d", roots, leaves)
	}
	for _, dependency := range generated.Workflow.Dependencies {
		if dependency.DataMB <= 0 {
			t.Fatalf("dependency %s -> %s has invalid data size %v", dependency.Source, dependency.Target, dependency.DataMB)
		}
	}
	if len(generated.Matrices.InterferenceIN["h3-standard-88-1"]["cpu"]) != 0 {
		t.Fatal("large workflow interference matrix must remain sparse")
	}
}

func TestClassicHEFTSchedulesMontageDSS20WithoutColocation(t *testing.T) {
	generated, err := generateExperimentSimulationForWorkflow(
		"cloud_hetero", montageDSS20WorkflowID, 42, 1, false, minBeamWidth,
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := scheduleHEFTClassic(generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assignments) != 6448 {
		t.Fatalf("expected 6448 classic HEFT assignments, got %d", len(result.Assignments))
	}
	if result.InterferenceVariables.TotalInterferenceTime != 0 {
		t.Fatalf("classic HEFT must not activate DSS 20d interference, got %v", result.InterferenceVariables.TotalInterferenceTime)
	}
}

func TestExperimentalRunnerWritesOnePairedRepetition(t *testing.T) {
	output := t.TempDir()
	if err := runExperimentalProtocol(ExperimentRunOptions{
		OutputDirectory: output, Repetitions: 1, StructuralSeed: 42, BeamWidth: minBeamWidth,
	}); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(output, "raw_results.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 19 {
		t.Fatalf("expected header plus 18 paired executions, got %d rows", len(records))
	}
	header := map[string]int{}
	for index, name := range records[0] {
		header[name] = index
	}
	if _, ok := header["recommendations_json"]; !ok {
		t.Fatal("raw_results.csv must contain recommendations_json")
	}
	seenRecommendationAlgorithms := map[string]bool{}
	for _, row := range records[1:] {
		algorithm := row[header["algorithm"]]
		if algorithm == "heft_classic" {
			if row[header["recommendations_json"]] != "[]" {
				t.Fatal("HEFT rows must have an empty recommendation list")
			}
			continue
		}
		var recommendations []ExperimentRecommendationRecord
		if err := json.Unmarshal([]byte(row[header["recommendations_json"]]), &recommendations); err != nil {
			t.Fatal(err)
		}
		if len(recommendations) == 0 {
			t.Fatalf("%s row must include recommendations", algorithm)
		}
		seenRecommendationAlgorithms[algorithm] = true
	}
	if !seenRecommendationAlgorithms["prism_cc_time"] || !seenRecommendationAlgorithms["prism_cc_cost"] {
		t.Fatalf("expected Time and Cost recommendations, got %v", seenRecommendationAlgorithms)
	}
	heftMakespans := map[string][]float64{}
	heftCosts := map[string][]float64{}
	for _, row := range records[1:] {
		if row[header["algorithm"]] == "heft_classic" {
			scenarioID := row[header["scenario_id"]]
			makespan, _ := strconv.ParseFloat(row[header["makespan"]], 64)
			cost, _ := strconv.ParseFloat(row[header["budget_used"]], 64)
			heftMakespans[scenarioID] = append(heftMakespans[scenarioID], makespan)
			heftCosts[scenarioID] = append(heftCosts[scenarioID], cost)
		}
	}
	for _, row := range records[1:] {
		scenarioID := row[header["scenario_id"]]
		wantDeadline := round(mean(heftMakespans[scenarioID])*experimentSLAMargin, 6)
		wantBudget := round(mean(heftCosts[scenarioID])*experimentSLAMargin, 6)
		deadline, _ := strconv.ParseFloat(row[header["deadline_limit"]], 64)
		budget, _ := strconv.ParseFloat(row[header["budget_limit"]], 64)
		if math.Abs(deadline-wantDeadline) > 1e-6 {
			t.Fatalf("%s deadline must equal HEFT mean times margin: got %v, want %v", scenarioID, deadline, wantDeadline)
		}
		if math.Abs(budget-wantBudget) > 1e-6 {
			t.Fatalf("%s budget must equal HEFT mean times margin: got %v, want %v", scenarioID, budget, wantBudget)
		}
	}
	var manifest ExperimentManifest
	data, err := os.ReadFile(filepath.Join(output, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SLAMargin != experimentSLAMargin {
		t.Fatalf("unexpected SLA margin: got %v, want %v", manifest.SLAMargin, experimentSLAMargin)
	}
	if len(manifest.ScenarioSLAs) != len(experimentScenarioIDs) {
		t.Fatalf("expected one SLA per scenario, got %v", manifest.ScenarioSLAs)
	}
	for _, name := range []string{"manifest.json", "summary.csv"} {
		if _, err := os.Stat(filepath.Join(output, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	for _, name := range []string{"recommendations.csv"} {
		if _, err := os.Stat(filepath.Join(output, name)); !os.IsNotExist(err) {
			t.Fatalf("%s must not be generated separately", name)
		}
	}
}

func TestExperimentalRunnerSupportsColocatedHEFTBaseline(t *testing.T) {
	output := t.TempDir()
	if err := runExperimentalProtocol(ExperimentRunOptions{
		OutputDirectory: output, Repetitions: 1, StructuralSeed: 42,
		BeamWidth: minBeamWidth, HEFTMode: "colocation",
	}); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(output, "raw_results.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	header := map[string]int{}
	for index, name := range records[0] {
		header[name] = index
	}
	baselineRows := 0
	for _, row := range records[1:] {
		if row[header["algorithm"]] == "heft_colocation" {
			baselineRows++
		}
	}
	if baselineRows != len(experimentScenarioIDs) {
		t.Fatalf("expected %d colocated HEFT rows, got %d", len(experimentScenarioIDs), baselineRows)
	}
	var manifest ExperimentManifest
	data, err := os.ReadFile(filepath.Join(output, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.HEFTMode != "colocation" {
		t.Fatalf("unexpected HEFT mode %q", manifest.HEFTMode)
	}
	if !reflect.DeepEqual(manifest.Algorithms, []string{"prism_cc_time", "prism_cc_cost", "heft_colocation"}) {
		t.Fatalf("unexpected algorithms: %v", manifest.Algorithms)
	}
}

func TestC3DReferenceLimitsAndWholeMachineBilling(t *testing.T) {
	if c3dStandard16ReferenceMakespanSeconds != 2815 {
		t.Fatalf("unexpected C3D deadline: %v", c3dStandard16ReferenceMakespanSeconds)
	}
	if round(c3dStandard16ReferenceBudgetUSD, 6) != 0.567992 {
		t.Fatalf("unexpected C3D budget: %v", c3dStandard16ReferenceBudgetUSD)
	}
	resource := Resource{ID: "c3d-standard-16", PricePerHourUSD: c3dStandard16ReferencePricePerHourUSD}
	assignments := []Assignment{{ResourceID: resource.ID, StartTime: 0, FinishTime: 2815}}
	if round(machineActiveCost(assignments, resource), 6) != 0.567992 {
		t.Fatalf("whole-machine billing mismatch: %v", machineActiveCost(assignments, resource))
	}
}
