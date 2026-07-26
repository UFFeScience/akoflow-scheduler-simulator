package main

import (
	"encoding/csv"
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
	heftMakespans := []float64{}
	heftCosts := []float64{}
	for _, row := range records[1:] {
		if row[header["algorithm"]] == "heft_classic" {
			makespan, _ := strconv.ParseFloat(row[header["makespan"]], 64)
			cost, _ := strconv.ParseFloat(row[header["budget_used"]], 64)
			heftMakespans = append(heftMakespans, makespan)
			heftCosts = append(heftCosts, cost)
		}
	}
	wantDeadline := mean(heftMakespans)
	wantBudget := mean(heftCosts)
	for _, row := range records[1:] {
		deadline, _ := strconv.ParseFloat(row[header["deadline_limit"]], 64)
		budget, _ := strconv.ParseFloat(row[header["budget_limit"]], 64)
		if math.Abs(deadline-wantDeadline) > 1e-6 {
			t.Fatalf("deadline must equal global HEFT mean: got %v, want %v", deadline, wantDeadline)
		}
		if math.Abs(budget-wantBudget) > 1e-6 {
			t.Fatalf("budget must equal global HEFT mean: got %v, want %v", budget, wantBudget)
		}
	}
	for _, name := range []string{"summary.csv", "manifest.json"} {
		if _, err := os.Stat(filepath.Join(output, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
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
