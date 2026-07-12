package main

import (
	"encoding/json"
	"reflect"
	"sort"
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

func TestBeamOptimizerIsDeterministic(t *testing.T) {
	req := defaultRequest()
	req.Seed = 123
	req.TaskCount = 12
	req.EdgeDensity = 0.35
	req.BudgetLimit = ptr(9999.0)
	req.DeadlineLimit = ptr(9999.0)
	req.OptionCount = 20
	generated, err := generateSimulation(req)
	if err != nil {
		t.Fatal(err)
	}
	first, err := optimizeSchedule(generated)
	if err != nil {
		t.Fatal(err)
	}
	second, err := optimizeSchedule(generated)
	if err != nil {
		t.Fatal(err)
	}
	if (first.SelectedOptionID == nil) != (second.SelectedOptionID == nil) {
		t.Fatal("selected option nil mismatch")
	}
	if first.SelectedOptionID != nil && *first.SelectedOptionID != *second.SelectedOptionID {
		t.Fatalf("selected option mismatch: %s != %s", *first.SelectedOptionID, *second.SelectedOptionID)
	}
	if len(first.Options) != len(second.Options) {
		t.Fatalf("option count mismatch: %d != %d", len(first.Options), len(second.Options))
	}
	for i := range first.Options {
		a, b := first.Options[i], second.Options[i]
		if a.ID != b.ID || a.Rank != b.Rank || a.MachineSignature != b.MachineSignature || a.Makespan != b.Makespan || a.BudgetUsed != b.BudgetUsed {
			t.Fatalf("option %d mismatch: %+v != %+v", i, a, b)
		}
	}
}

func TestBeamSearchStateSetDoesNotDependOnUserWeights(t *testing.T) {
	req := defaultRequest()
	req.Seed = 123
	req.TaskCount = 10
	req.EdgeDensity = 0.35
	req.BeamWidth = 120
	generated, err := generateSimulation(req)
	if err != nil {
		t.Fatal(err)
	}
	timeFirst := generated
	timeFirst.SLA.WeightTime = 1
	timeFirst.SLA.WeightCost = 0
	costFirst := generated
	costFirst.SLA.WeightTime = 0
	costFirst.SLA.WeightCost = 1
	timeStates, err := beamSearch(timeFirst, normalizedBeamWidth(timeFirst.SLA.BeamWidth))
	if err != nil {
		t.Fatal(err)
	}
	costStates, err := beamSearch(costFirst, normalizedBeamWidth(costFirst.SLA.BeamWidth))
	if err != nil {
		t.Fatal(err)
	}
	timeSignatures := stateSignatures(timeStates)
	costSignatures := stateSignatures(costStates)
	if len(timeSignatures) != len(costSignatures) {
		t.Fatalf("state count should not depend on user weights: %d != %d", len(timeSignatures), len(costSignatures))
	}
	for i := range timeSignatures {
		if timeSignatures[i] != costSignatures[i] {
			t.Fatalf("state signature %d should not depend on user weights: %s != %s", i, timeSignatures[i], costSignatures[i])
		}
	}
}

func TestBeamExpansionPartialScoreDoesNotDependOnUserWeights(t *testing.T) {
	req := defaultRequest()
	req.Seed = 321
	req.TaskCount = 6
	req.EdgeDensity = 0.25
	generated, err := generateSimulation(req)
	if err != nil {
		t.Fatal(err)
	}
	ctx := optimizerContext{Tasks: taskMap(generated.Workflow.Tasks), Resources: resourceMap(generated.Resources), DepsByTarget: dependenciesByTarget(generated.Workflow.Dependencies)}
	order, err := topologicalOrder(generated)
	if err != nil {
		t.Fatal(err)
	}
	coreAvail := map[string]float64{}
	for _, resource := range generated.Resources {
		for _, core := range resource.Cores {
			coreAvail[core.ID] = 0
		}
	}
	hasBooted, ready, last := initialNodeState(generated.Resources)
	initial := beamState{Assignments: []Assignment{}, AssignmentByTask: map[string]Assignment{}, CoreAvail: coreAvail, NodeHasBooted: hasBooted, NodeReadyTime: ready, NodeLastActive: last, StopIntervals: []MachineStopInterval{}}
	frontier := beamFrontier{WeightTime: 0.3, WeightCost: 0.7}

	timeFirst := generated
	timeFirst.SLA.WeightTime = 1
	timeFirst.SLA.WeightCost = 0
	costFirst := generated
	costFirst.SLA.WeightTime = 0
	costFirst.SLA.WeightCost = 1

	timeExpanded := expandState(timeFirst, ctx, initial, 1, order[0], frontier)
	costExpanded := expandState(costFirst, ctx, initial, 1, order[0], frontier)
	if len(timeExpanded) != len(costExpanded) {
		t.Fatalf("expanded count should not depend on user weights: %d != %d", len(timeExpanded), len(costExpanded))
	}
	for i := range timeExpanded {
		if timeExpanded[i].PartialScore != costExpanded[i].PartialScore {
			t.Fatalf("partial score %d should not depend on user weights: %v != %v", i, timeExpanded[i].PartialScore, costExpanded[i].PartialScore)
		}
	}
}

func TestBeamOptimizerRecommendationIsNotDominatedByReturnedOption(t *testing.T) {
	req := defaultRequest()
	req.Seed = 321
	req.TaskCount = 16
	req.EdgeDensity = 0.35
	req.BudgetLimit = ptr(30.0)
	req.OptionCount = 30
	generated, err := generateSimulation(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, deadline := range []float64{180, 110} {
		generated.SLA.BudgetLimit = ptr(30.0)
		generated.SLA.DeadlineLimit = ptr(deadline)
		response, err := optimizeSchedule(generated)
		if err != nil {
			t.Fatal(err)
		}
		if len(response.Options) == 0 || response.SelectedOptionID == nil {
			t.Fatalf("missing recommendation for deadline %v", deadline)
		}
		recommended := response.Options[0]
		if recommended.ID != *response.SelectedOptionID || !recommended.Recommended {
			t.Fatalf("first option should be recommended for deadline %v", deadline)
		}
		for _, option := range response.Options[1:] {
			dominates := option.Makespan <= recommended.Makespan && option.BudgetUsed <= recommended.BudgetUsed && (option.Makespan < recommended.Makespan || option.BudgetUsed < recommended.BudgetUsed)
			if dominates {
				t.Fatalf("deadline %v recommended dominated option: rec makespan=%v budget=%v, better makespan=%v budget=%v", deadline, recommended.Makespan, recommended.BudgetUsed, option.Makespan, option.BudgetUsed)
			}
		}
	}
}

func TestDeadlineConstraintSurvivesCostOnlyObjective(t *testing.T) {
	req := defaultRequest()
	req.Seed = 321
	req.TaskCount = 16
	req.EdgeDensity = 0.35
	req.OptionCount = 100
	req.WeightTime = 1
	req.WeightCost = 0
	generated, err := generateSimulation(req)
	if err != nil {
		t.Fatal(err)
	}
	timeResponse, err := optimizeSchedule(generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(timeResponse.Options) == 0 {
		t.Fatal("missing time-prioritized options")
	}
	deadline := timeResponse.Options[0].Makespan
	generated.SLA.WeightTime = 0
	generated.SLA.WeightCost = 1
	generated.SLA.DeadlineLimit = ptr(deadline)
	costResponse, err := optimizeSchedule(generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(costResponse.Options) == 0 {
		t.Fatal("missing cost-prioritized options")
	}
	if !costResponse.Options[0].Feasible {
		t.Fatalf("cost-only recommendation should preserve deadline feasibility: deadline=%v recommended makespan=%v violation=%v", deadline, costResponse.Options[0].Makespan, costResponse.Options[0].DeadlineViolation)
	}
}

func TestOptionRankPrioritizesConstraintSatisfaction(t *testing.T) {
	budgetLimit, deadlineLimit := 20.0, 180.0
	budgetCompliant := ScheduleOption{
		Feasible:          false,
		BudgetUsed:        19.67,
		BudgetLimit:       &budgetLimit,
		BudgetViolation:   0,
		Makespan:          180.57,
		DeadlineLimit:     &deadlineLimit,
		DeadlineViolation: 0.57,
		WeightedScore:     1.357,
	}
	budgetViolating := ScheduleOption{
		Feasible:          false,
		BudgetUsed:        25.863,
		BudgetLimit:       &budgetLimit,
		BudgetViolation:   5.863,
		Makespan:          178.882,
		DeadlineLimit:     &deadlineLimit,
		DeadlineViolation: 0,
		WeightedScore:     0.922,
	}
	if !optionRankLess(budgetCompliant, budgetViolating, 0, 1) {
		t.Fatal("budget-compliant option should rank before a budget-violating option")
	}
	feasible := budgetCompliant
	feasible.Feasible = true
	feasible.Makespan = 179.9
	feasible.DeadlineViolation = 0
	if !optionRankLess(feasible, budgetCompliant, 0, 1) {
		t.Fatal("feasible option should rank before infeasible option")
	}
}

func TestOptionRankPrioritizesDeadlineViolationForTimeObjective(t *testing.T) {
	budgetLimit, deadlineLimit := 20.0, 180.0
	lowerDeadlineViolation := ScheduleOption{BudgetLimit: &budgetLimit, BudgetViolation: 10, DeadlineLimit: &deadlineLimit, DeadlineViolation: 1, WeightedScore: 10}
	higherDeadlineViolation := ScheduleOption{BudgetLimit: &budgetLimit, BudgetViolation: 0, DeadlineLimit: &deadlineLimit, DeadlineViolation: 2, WeightedScore: 1}
	if !optionRankLess(lowerDeadlineViolation, higherDeadlineViolation, 1, 0) {
		t.Fatal("time objective should prioritize lower deadline violation")
	}
}

func TestOptionRankPrioritizesBudgetViolationForCostObjective(t *testing.T) {
	budgetLimit, deadlineLimit := 20.0, 180.0
	lowerBudgetViolation := ScheduleOption{BudgetLimit: &budgetLimit, BudgetViolation: 1, DeadlineLimit: &deadlineLimit, DeadlineViolation: 10, WeightedScore: 10}
	higherBudgetViolation := ScheduleOption{BudgetLimit: &budgetLimit, BudgetViolation: 2, DeadlineLimit: &deadlineLimit, DeadlineViolation: 0, WeightedScore: 1}
	if !optionRankLess(lowerBudgetViolation, higherBudgetViolation, 0, 1) {
		t.Fatal("cost objective should prioritize lower budget violation")
	}
}

func TestOptionRankUsesCombinedViolationForBalancedObjective(t *testing.T) {
	budgetLimit, deadlineLimit := 20.0, 180.0
	lowerCombinedViolation := ScheduleOption{BudgetLimit: &budgetLimit, BudgetViolation: 1, DeadlineLimit: &deadlineLimit, DeadlineViolation: 1, WeightedScore: 10}
	higherCombinedViolation := ScheduleOption{BudgetLimit: &budgetLimit, BudgetViolation: 3, DeadlineLimit: &deadlineLimit, DeadlineViolation: 3, WeightedScore: 1}
	if !optionRankLess(lowerCombinedViolation, higherCombinedViolation, 0.5, 0.5) {
		t.Fatal("balanced objective should prioritize lower combined normalized violation")
	}
}

func TestBeamSelectionUsesFrontierSpecificScores(t *testing.T) {
	costFrontier := beamFrontier{WeightTime: 0, WeightCost: 1}
	timeFrontier := beamFrontier{WeightTime: 1, WeightCost: 0}
	cheapSlow := ScoreBreakdown{TimeScore: 1, CostScore: 0.2}
	fastExpensive := ScoreBreakdown{TimeScore: 0.2, CostScore: 1}
	if beamFrontierScore(cheapSlow, costFrontier) >= beamFrontierScore(fastExpensive, costFrontier) {
		t.Fatal("cost frontier should prefer lower cost score")
	}
	if beamFrontierScore(fastExpensive, timeFrontier) >= beamFrontierScore(cheapSlow, timeFrontier) {
		t.Fatal("time frontier should prefer lower time score")
	}
}

func TestBeamSearchUsesFiveObjectiveFrontiers(t *testing.T) {
	frontiers := beamFrontiers()
	if len(frontiers) != 5 {
		t.Fatalf("expected five objective frontiers, got %d", len(frontiers))
	}
	expected := []beamFrontier{
		{WeightTime: 0, WeightCost: 1},
		{WeightTime: 0.3, WeightCost: 0.7},
		{WeightTime: 0.5, WeightCost: 0.5},
		{WeightTime: 0.7, WeightCost: 0.3},
		{WeightTime: 1, WeightCost: 0},
	}
	for index, frontier := range frontiers {
		if frontier != expected[index] {
			t.Fatalf("frontier %d mismatch: %+v != %+v", index, frontier, expected[index])
		}
	}
	widths := beamFrontierWidths(120, len(frontiers))
	total := 0
	for index, width := range widths {
		if width == 0 {
			t.Fatalf("frontier %d should receive beam width", index)
		}
		total += width
	}
	if total != 120 {
		t.Fatalf("expected widths to sum to 120, got %d", total)
	}
}

func TestBeamSelectionPrefersPartiallyFeasibleStates(t *testing.T) {
	budgetLimit, deadlineLimit := 20.0, 180.0
	generated := GeneratedSimulation{SLA: SLA{BudgetLimit: &budgetLimit, DeadlineLimit: &deadlineLimit}}
	states := []beamState{
		{Assignments: []Assignment{{TaskID: "t1", ResourceID: "feasible", CoreID: "c1"}}, PartialBudgetUsed: 19.9, PartialMakespan: 179.9, PartialScore: 10},
		{Assignments: []Assignment{{TaskID: "t1", ResourceID: "deadline", CoreID: "c2"}}, PartialBudgetUsed: 1, PartialMakespan: 180.1, PartialScore: 1},
		{Assignments: []Assignment{{TaskID: "t1", ResourceID: "budget", CoreID: "c3"}}, PartialBudgetUsed: 20.1, PartialMakespan: 1, PartialScore: 1},
	}
	selected := selectBeamStates(states, 10, generated)
	if len(selected) != 1 {
		t.Fatalf("expected only partially feasible states to remain, got %d", len(selected))
	}
	if selected[0].Assignments[0].ResourceID != "feasible" {
		t.Fatalf("expected feasible state to remain, got %s", selected[0].Assignments[0].ResourceID)
	}
}

func TestBeamSelectionKeepsFallbackWhenNoPartialFeasibleStateExists(t *testing.T) {
	budgetLimit, deadlineLimit := 20.0, 180.0
	generated := GeneratedSimulation{SLA: SLA{BudgetLimit: &budgetLimit, DeadlineLimit: &deadlineLimit}}
	states := []beamState{
		{Assignments: []Assignment{{TaskID: "t1", ResourceID: "best", CoreID: "c1"}}, PartialBudgetUsed: 20.1, PartialMakespan: 179.9, PartialScore: 1},
		{Assignments: []Assignment{{TaskID: "t1", ResourceID: "worse", CoreID: "c2"}}, PartialBudgetUsed: 19.9, PartialMakespan: 180.1, PartialScore: 2},
	}
	selected := selectBeamStates(states, 1, generated)
	if len(selected) != 1 {
		t.Fatalf("expected one fallback state, got %d", len(selected))
	}
	if selected[0].Assignments[0].ResourceID != "best" {
		t.Fatalf("expected best fallback state to remain, got %s", selected[0].Assignments[0].ResourceID)
	}
}

func TestBeamSelectionPrunesPerFrontierWithoutChangingBeamSplit(t *testing.T) {
	budgetLimit, deadlineLimit := 20.0, 180.0
	generated := GeneratedSimulation{SLA: SLA{BudgetLimit: &budgetLimit, DeadlineLimit: &deadlineLimit}}
	frontiers := beamFrontiers()
	widths := beamFrontierWidths(120, len(frontiers))
	if len(frontiers) != 5 || len(widths) != 5 {
		t.Fatalf("expected five frontiers and widths, got %d/%d", len(frontiers), len(widths))
	}
	states := []beamState{
		{Assignments: []Assignment{{TaskID: "t1", ResourceID: "feasible", CoreID: "c1"}}, PartialBudgetUsed: 19.9, PartialMakespan: 179.9, PartialScore: 10},
		{Assignments: []Assignment{{TaskID: "t1", ResourceID: "infeasible", CoreID: "c2"}}, PartialBudgetUsed: 20.1, PartialMakespan: 180.1, PartialScore: 1},
	}
	for index, width := range widths {
		selected := selectBeamStates(states, width, generated)
		if len(selected) != 1 {
			t.Fatalf("frontier %d should keep only partial feasible states, got %d", index, len(selected))
		}
		if selected[0].Assignments[0].ResourceID != "feasible" {
			t.Fatalf("frontier %d expected feasible state, got %s", index, selected[0].Assignments[0].ResourceID)
		}
	}
	total := 0
	for _, width := range widths {
		total += width
	}
	if total != 120 {
		t.Fatalf("expected widths to remain split across beam, got %d", total)
	}
}

func TestValidationRejectsOptionCountAboveMaximum(t *testing.T) {
	req := defaultRequest()
	req.OptionCount = maxScheduleOptions + 1
	if err := validateRequest(req); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidationAcceptsMaximumOptionCount(t *testing.T) {
	req := defaultRequest()
	req.OptionCount = maxScheduleOptions
	if err := validateRequest(req); err != nil {
		t.Fatal(err)
	}
}

func TestValidationRejectsBeamWidthOutsideRange(t *testing.T) {
	req := defaultRequest()
	req.BeamWidth = minBeamWidth - 1
	if err := validateRequest(req); err == nil {
		t.Fatal("expected validation error below minimum beam_width")
	}
	req.BeamWidth = maxBeamWidth + 1
	if err := validateRequest(req); err == nil {
		t.Fatal("expected validation error above maximum beam_width")
	}
}

func TestValidationAcceptsBeamWidthRange(t *testing.T) {
	req := defaultRequest()
	req.BeamWidth = minBeamWidth
	if err := validateRequest(req); err != nil {
		t.Fatal(err)
	}
	req.BeamWidth = maxBeamWidth
	if err := validateRequest(req); err != nil {
		t.Fatal(err)
	}
}

func TestScheduleResponseIncludesBeamWidth(t *testing.T) {
	generated := defaultGenerated(t)
	generated.SLA.BeamWidth = 250
	response, err := optimizeSchedule(generated)
	if err != nil {
		t.Fatal(err)
	}
	if response.Constraints.BeamWidth != 250 {
		t.Fatalf("expected beam width 250, got %d", response.Constraints.BeamWidth)
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
	constraints := decoded["constraints"].(map[string]any)
	if _, ok := constraints["beam_width"]; !ok {
		t.Fatal("missing constraints.beam_width")
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

func BenchmarkOptimizeScheduleOptionCount100(b *testing.B) {
	req := defaultRequest()
	req.Seed = 321
	req.TaskCount = 16
	req.EdgeDensity = 0.35
	req.BudgetLimit = ptr(9999.0)
	req.DeadlineLimit = ptr(9999.0)
	req.OptionCount = 100
	generated, err := generateSimulation(req)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := optimizeSchedule(generated); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOptimizeScheduleOptionCount100Activities59(b *testing.B) {
	req := defaultRequest()
	req.Seed = 321
	req.TaskCount = 59
	req.EdgeDensity = 0.35
	req.BudgetLimit = ptr(9999.0)
	req.DeadlineLimit = ptr(9999.0)
	req.OptionCount = 100
	generated, err := generateSimulation(req)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := optimizeSchedule(generated); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOptimizeScheduleOptionCount1000(b *testing.B) {
	req := defaultRequest()
	req.Seed = 321
	req.TaskCount = 3
	req.EdgeDensity = 0.2
	req.BudgetLimit = ptr(9999.0)
	req.DeadlineLimit = ptr(9999.0)
	req.OptionCount = 1000
	generated, err := generateSimulation(req)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := optimizeSchedule(generated); err != nil {
			b.Fatal(err)
		}
	}
}

func ptr[T any](value T) *T {
	return &value
}

func stateSignatures(states []beamState) []string {
	signatures := make([]string, len(states))
	for i, state := range states {
		signatures[i] = stateSignature(state)
	}
	sort.Strings(signatures)
	return signatures
}
