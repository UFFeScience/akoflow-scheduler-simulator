package main

import "testing"

func TestPRISMCostScoreIncludesFinancialNetworkCost(t *testing.T) {
	if got := prismIncrementalFinancialCost(2.5, 7.5); got != 10 {
		t.Fatalf("incremental financial cost=%v, want compute+network=10", got)
	}
}

func TestPartialParetoArchiveRemovesDominatedState(t *testing.T) {
	states := []beamState{
		{Assignments: []Assignment{{TaskID: "a", ResourceID: "r1"}}, ScheduledTaskHash: 1, PartialMakespan: 10, PartialBudgetUsed: 10},
		{Assignments: []Assignment{{TaskID: "a", ResourceID: "r2"}}, ScheduledTaskHash: 1, PartialMakespan: 12, PartialBudgetUsed: 12},
		{Assignments: []Assignment{{TaskID: "a", ResourceID: "r3"}}, ScheduledTaskHash: 1, PartialMakespan: 8, PartialBudgetUsed: 14},
	}
	archive := partialParetoArchive(states, GeneratedSimulation{})
	if len(archive) != 2 {
		t.Fatalf("Pareto archive retained %d states, want 2", len(archive))
	}
}

func TestDynamicContainerCacheMakesSecondUseWarm(t *testing.T) {
	task := Task{ID: "a", Image: "workflow:v1"}
	generated := GeneratedSimulation{Matrices: Matrices{ContainerOverhead: map[string]map[string]float64{
		task.ID: {"r1": 7},
	}}}
	cold := beamState{CachedImages: map[string]bool{}}
	warm := beamState{CachedImages: map[string]bool{resourceImageCacheKey("r1", task.Image): true}}
	if got := dynamicContainerOverhead(generated, cold, task, "r1"); got != 7 {
		t.Fatalf("cold overhead=%v, want 7", got)
	}
	if got := dynamicContainerOverhead(generated, warm, task, "r1"); got != 0 {
		t.Fatalf("warm overhead=%v, want 0", got)
	}
}

func TestProfilePCCPenalizesSameProfileMore(t *testing.T) {
	left := Task{ID: "left", CPU: 1, BaseRuntime: 10}
	right := Task{ID: "right", CPU: 1, BaseRuntime: 10}
	ctx := optimizerContext{Tasks: map[string]Task{left.ID: left, right.ID: right}}
	adjusted, profile := explicitProfilePCC(ctx, left, 0.2, []PairwiseInterference{{OtherTaskID: right.ID}})
	if profile != "cpu" || adjusted != 0.25 {
		t.Fatalf("profile=%s adjusted=%v, want cpu/0.25", profile, adjusted)
	}
}

func TestCapacityConstrainedStartWaitsForCPUAndMemory(t *testing.T) {
	resource := Resource{ID: "r1", CPU: 2, Memory: 4, Cores: []Core{{ID: "c1"}, {ID: "c2"}}}
	runningTask := Task{ID: "running", CPU: 2, Memory: 3}
	candidateTask := Task{ID: "candidate", CPU: 1, Memory: 2}
	ctx := optimizerContext{Tasks: map[string]Task{runningTask.ID: runningTask, candidateTask.ID: candidateTask}}
	state := beamState{Assignments: []Assignment{{
		TaskID: runningTask.ID, ResourceID: resource.ID, StartTime: 0, FinishTime: 10,
	}}}
	if got := capacityConstrainedStart(ctx, state, candidateTask, resource, 0, 5); got != 10 {
		t.Fatalf("capacity start=%v, want 10", got)
	}
}

func TestPRISMUpwardRankIncludesExpectedInterference(t *testing.T) {
	req := defaultRequest()
	req.Seed = 991
	req.TaskCount = 8
	generated, err := generateSimulation(req)
	if err != nil {
		t.Fatal(err)
	}
	generated.Experimental = &ExperimentMetadata{InterferenceDisabled: true}
	without, err := prismCommunicationInterferenceRanks(generated)
	if err != nil {
		t.Fatal(err)
	}
	generated.Experimental.InterferenceDisabled = false
	generated.Experimental.InterferenceRate = 0.5
	with, err := prismCommunicationInterferenceRanks(generated)
	if err != nil {
		t.Fatal(err)
	}
	increased := false
	for taskID := range without {
		increased = increased || with[taskID] > without[taskID]
	}
	if !increased {
		t.Fatal("communication/interference rank did not react to interference")
	}
}

func TestIndependentFrontierKeepsTenStatesAndCanonicalAnchor(t *testing.T) {
	states := make([]beamState, 0, 20)
	for index := 0; index < 20; index++ {
		state := beamState{
			Assignments:     []Assignment{{TaskID: "a", ResourceID: string(rune('a' + index))}},
			PartialMakespan: float64(index + 1), PartialBudgetUsed: float64(20 - index),
			OrderDeviated: index != 19,
		}
		state.FrontierScores[5] = float64(index)
		states = append(states, state)
	}
	selected := selectIndependentFrontierStates(states, 10, 5, GeneratedSimulation{})
	if len(selected) != 10 {
		t.Fatalf("frontier retained %d states, want 10", len(selected))
	}
	foundCanonical := false
	for _, state := range selected {
		foundCanonical = foundCanonical || !state.OrderDeviated
	}
	if !foundCanonical {
		t.Fatal("independent frontier lost canonical HEFT anchor")
	}
}

func TestAdaptiveLookaheadUsesNoDepthForClearDecision(t *testing.T) {
	task := Task{ID: "task"}
	ctx := optimizerContext{LookaheadDepth: map[string]int{task.ID: 3}}
	if got := adaptiveDecisionLookaheadDepth(ctx, task, []float64{10, 20}, []float64{1, 2}); got != 0 {
		t.Fatalf("clear decision depth=%d, want 0", got)
	}
}

func TestAdaptiveLookaheadKeepsStructuralDepthForAmbiguousDecision(t *testing.T) {
	task := Task{ID: "task"}
	ctx := optimizerContext{LookaheadDepth: map[string]int{task.ID: 3}}
	if got := adaptiveDecisionLookaheadDepth(ctx, task, []float64{10, 10.2}, []float64{2, 1}); got != 3 {
		t.Fatalf("ambiguous decision depth=%d, want 3", got)
	}
}

func TestAdaptiveLookaheadCriticalTaskUsesDepthThree(t *testing.T) {
	task := Task{ID: "critical"}
	successor := Task{ID: "successor"}
	generated := GeneratedSimulation{Workflow: Workflow{
		Tasks: []Task{task, successor}, Dependencies: []Dependency{{Source: task.ID, Target: successor.ID, DataMB: 1}},
	}}
	ctx := optimizerContext{
		Tasks:         map[string]Task{task.ID: task, successor.ID: successor},
		DepsBySource:  map[string][]Dependency{task.ID: {{Source: task.ID, Target: successor.ID, DataMB: 1}}},
		DepsByTarget:  map[string][]Dependency{successor.ID: {{Source: task.ID, Target: successor.ID, DataMB: 1}}},
		PriorityRanks: map[string]float64{task.ID: 100, successor.ID: 1}, MaxPriorityRank: 100,
	}
	if got := adaptiveTaskLookaheadDepths(generated, ctx)[task.ID]; got != 3 {
		t.Fatalf("critical task depth=%d, want 3", got)
	}
}

func TestAdaptiveLookaheadHeavyForkReachesJoin(t *testing.T) {
	tasks := []Task{{ID: "fork"}, {ID: "left"}, {ID: "right"}, {ID: "join"}}
	deps := []Dependency{
		{Source: "fork", Target: "left", DataMB: 100}, {Source: "fork", Target: "right", DataMB: 100},
		{Source: "left", Target: "join", DataMB: 1}, {Source: "right", Target: "join", DataMB: 1},
	}
	ctx := optimizerContext{
		Tasks: taskMap(tasks), DepsBySource: dependenciesBySource(deps), DepsByTarget: dependenciesByTarget(deps),
		PriorityRanks: map[string]float64{"fork": 1}, MaxPriorityRank: 100,
	}
	depth := adaptiveTaskLookaheadDepths(GeneratedSimulation{Workflow: Workflow{Tasks: tasks, Dependencies: deps}}, ctx)["fork"]
	if depth != 2 {
		t.Fatalf("heavy fork depth=%d, want distance 2 to join", depth)
	}
}
