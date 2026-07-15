package main

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
)

type beamState struct {
	Assignments       []Assignment
	AssignmentByTask  map[string]Assignment
	CoreAvail         map[string]float64
	NodeHasBooted     map[string]bool
	NodeReadyTime     map[string]float64
	NodeLastActive    map[string]float64
	StopIntervals     []MachineStopInterval
	StepTrace         *scheduleStepTrace
	PartialBudgetUsed float64
	PartialMakespan   float64
	PartialScore      float64
}

type scheduleStepTrace struct {
	Step ScheduleStep
	Prev *scheduleStepTrace
	Len  int
}

type optimizerContext struct {
	Tasks        map[string]Task
	Resources    map[string]Resource
	DepsByTarget map[string][]Dependency
}

type beamFrontier struct {
	WeightTime float64
	WeightCost float64
}

const beamFrontierCount = 11

func optimizeSchedule(generated GeneratedSimulation) (ScheduleOptimizationResponse, error) {
	optionCount := max(1, min(generated.SLA.OptionCount, maxScheduleOptions))
	beamWidth := normalizedBeamWidth(generated.SLA.BeamWidth)
	finalStates, err := beamSearch(generated, beamWidth)
	if err != nil {
		return ScheduleOptimizationResponse{}, err
	}
	options := buildOptions(generated, finalStates, optionCount, generated.SLA.BudgetLimit, generated.SLA.DeadlineLimit)
	var selected *string
	if len(options) > 0 {
		selected = &options[0].ID
	}
	return ScheduleOptimizationResponse{SelectedOptionID: selected, Constraints: ScheduleConstraints{BudgetLimit: generated.SLA.BudgetLimit, DeadlineLimit: generated.SLA.DeadlineLimit, OptionCount: optionCount, BeamWidth: beamWidth}, Options: options}, nil
}

func normalizedBeamWidth(value int) int {
	if value <= 0 {
		return defaultBeamWidth
	}
	return max(minBeamWidth, min(value, maxBeamWidth))
}

func beamSearch(generated GeneratedSimulation, beamWidth int) ([]beamState, error) {
	coreAvail := map[string]float64{}
	for _, resource := range generated.Resources {
		for _, core := range resource.Cores {
			coreAvail[core.ID] = 0
		}
	}
	hasBooted, ready, last := initialNodeState(generated.Resources)
	initialBeam := []beamState{{Assignments: []Assignment{}, AssignmentByTask: map[string]Assignment{}, CoreAvail: coreAvail, NodeHasBooted: hasBooted, NodeReadyTime: ready, NodeLastActive: last, StopIntervals: []MachineStopInterval{}}}
	ctx := optimizerContext{Tasks: taskMap(generated.Workflow.Tasks), Resources: resourceMap(generated.Resources), DepsByTarget: dependenciesByTarget(generated.Workflow.Dependencies)}
	order, err := topologicalOrder(generated)
	if err != nil {
		return nil, err
	}
	frontiers := beamFrontiers()
	widths := beamFrontierWidths(beamWidth, len(frontiers))
	beams := make([][]beamState, len(frontiers))
	for index := range frontiers {
		beams[index] = append([]beamState{}, initialBeam...)
	}
	for stepIndex, taskID := range order {
		anyExpanded := false
		for index, frontier := range frontiers {
			expanded := expandStatesParallel(generated, ctx, beams[index], stepIndex+1, taskID, frontier)
			if len(expanded) == 0 {
				beams[index] = []beamState{}
				continue
			}
			anyExpanded = true
			beams[index] = selectBeamStates(expanded, widths[index], generated)
		}
		if !anyExpanded {
			return nil, fmt.Errorf("No feasible resource for task %s", taskID)
		}
	}
	finalStates := []beamState{}
	for index := range frontiers {
		finalStates = append(finalStates, beams[index]...)
	}
	return dedupeStates(finalStates), nil
}

func beamFrontiers() []beamFrontier {
	return []beamFrontier{
		{WeightTime: 0, WeightCost: 1},
		{WeightTime: 0.1, WeightCost: 0.9},
		{WeightTime: 0.2, WeightCost: 0.8},
		{WeightTime: 0.3, WeightCost: 0.7},
		{WeightTime: 0.4, WeightCost: 0.6},
		{WeightTime: 0.5, WeightCost: 0.5},
		{WeightTime: 0.6, WeightCost: 0.4},
		{WeightTime: 0.7, WeightCost: 0.3},
		{WeightTime: 0.8, WeightCost: 0.2},
		{WeightTime: 0.9, WeightCost: 0.1},
		{WeightTime: 1, WeightCost: 0},
	}
}

func beamFrontierWidths(total int, frontierCount int) []int {
	widths := make([]int, frontierCount)
	if frontierCount == 0 {
		return widths
	}
	base := max(1, total/frontierCount)
	remainder := max(0, total-base*frontierCount)
	for index := range widths {
		widths[index] = base
		if index < remainder {
			widths[index]++
		}
	}
	return widths
}

func expandStatesParallel(generated GeneratedSimulation, ctx optimizerContext, states []beamState, stepIndex int, taskID string, frontier beamFrontier) []beamState {
	if len(states) == 0 {
		return []beamState{}
	}
	if len(states) == 1 {
		return expandState(generated, ctx, states[0], stepIndex, taskID, frontier)
	}
	workers := min(len(states), runtime.GOMAXPROCS(0))
	results := make([][]beamState, len(states))
	jobs := make(chan int)
	var wg sync.WaitGroup
	wg.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func() {
			defer wg.Done()
			for index := range jobs {
				results[index] = expandState(generated, ctx, states[index], stepIndex, taskID, frontier)
			}
		}()
	}
	for index := range states {
		jobs <- index
	}
	close(jobs)
	wg.Wait()

	total := 0
	for _, stateResults := range results {
		total += len(stateResults)
	}
	out := make([]beamState, 0, total)
	for _, stateResults := range results {
		out = append(out, stateResults...)
	}
	return out
}

func expandState(generated GeneratedSimulation, ctx optimizerContext, state beamState, stepIndex int, taskID string, frontier beamFrontier) []beamState {
	task := ctx.Tasks[taskID]
	type candidateRow struct {
		assignment      Assignment
		candidate       CandidateEvaluation
		finish, rawCost float64
		phi, incBudget  float64
	}
	rows := []candidateRow{}
	for _, resource := range generated.Resources {
		if task.CPU > resource.CPU || task.Memory > resource.Memory {
			continue
		}
		predecessorFloor, transferTotal := predecessorTiming(ctx.DepsByTarget[task.ID], state.AssignmentByTask, generated, resource.ID)
		networkCost := 0.0
		for _, dep := range ctx.DepsByTarget[task.ID] {
			predecessor := state.AssignmentByTask[dep.Source]
			networkCost += dep.DataMB * generated.Matrices.FinancialNetworkCost[predecessor.ResourceID][resource.ID]
		}
		for _, core := range resource.Cores {
			readyFloor := maxf(predecessorFloor, state.CoreAvail[core.ID], state.NodeReadyTime[resource.ID])
			stopBoot := resource.Kind == "cloud" && state.NodeHasBooted[resource.ID] && resource.BootOverhead > 0 && readyFloor-state.NodeLastActive[resource.ID] >= resource.BootOverhead
			coldBoot := !state.NodeHasBooted[resource.ID]
			boot := 0.0
			if coldBoot || stopBoot {
				boot = resource.BootOverhead
			}
			container := generated.Matrices.ContainerOverhead[task.ID][resource.ID]
			bootReady := readyFloor
			if coldBoot {
				bootReady += boot
			}
			start := round(bootReady+container, 3)
			baseRuntime := generated.Matrices.ET0[task.ID][resource.ID]
			phi, pairwise := candidatePairwiseInterference(generated, task.ID, resource.ID, state.Assignments, start, start+baseRuntime)
			effective := round(baseRuntime*(1+phi), 3)
			finish := round(start+effective, 3)
			rawCost := effective * (task.CPU*resource.PricePerCPUSecond + task.Memory*resource.PricePerGBSecond)
			score := ScoreBreakdown{}
			assignment := Assignment{TaskID: task.ID, ResourceID: resource.ID, CoreID: core.ID, StartTime: start, FinishTime: finish, EffectiveRuntime: effective, TransferDelay: round(transferTotal, 3), BootOverhead: boot, ContainerOverhead: container, PhiN: phi, PredecessorFinishFloor: round(predecessorFloor, 3), Score: score}
			candidate := CandidateEvaluation{TaskID: task.ID, ResourceID: resource.ID, CoreID: core.ID, StartTime: start, FinishTime: finish, BaseRuntime: baseRuntime, EffectiveRuntime: effective, InterferenceTime: round(effective-baseRuntime, 3), TransferDelay: round(transferTotal, 3), BootOverhead: boot, ContainerOverhead: container, PredecessorFinishFloor: round(predecessorFloor, 3), RawCost: round(rawCost, 4), PhiN: phi, PairwiseInterference: pairwise, Score: score}
			rows = append(rows, candidateRow{assignment: assignment, candidate: candidate, finish: finish, rawCost: rawCost, phi: phi, incBudget: rawCost + networkCost})
		}
	}
	if len(rows) == 0 {
		return []beamState{}
	}
	maxFinish, maxRawCost := 0.0, 0.0
	for _, row := range rows {
		maxFinish = maxf(maxFinish, row.finish)
		maxRawCost = maxf(maxRawCost, row.rawCost)
	}
	for i := range rows {
		timeScore := rows[i].finish / maxf(maxFinish, 0.001)
		costScore := 0.0
		if maxRawCost != 0 {
			costScore = rows[i].rawCost / maxRawCost
		}
		score := ScoreBreakdown{
			TimeScore: round(timeScore, 5), CostScore: round(costScore, 5),
			InterferenceScore: round(rows[i].phi, 5),
			TotalScore:        round(0.5*timeScore+0.5*costScore, 5),
		}
		rows[i].assignment.Score = score
		rows[i].candidate.Score = score
	}
	rankedRows := append([]candidateRow{}, rows...)
	sort.SliceStable(rankedRows, func(i, j int) bool {
		a, b := beamFrontierScore(rankedRows[i].assignment.Score, frontier), beamFrontierScore(rankedRows[j].assignment.Score, frontier)
		if a != b {
			return a < b
		}
		if rankedRows[i].assignment.FinishTime != rankedRows[j].assignment.FinishTime {
			return rankedRows[i].assignment.FinishTime < rankedRows[j].assignment.FinishTime
		}
		if rankedRows[i].incBudget != rankedRows[j].incBudget {
			return rankedRows[i].incBudget < rankedRows[j].incBudget
		}
		return rankedRows[i].assignment.ResourceID+"|"+rankedRows[i].assignment.CoreID < rankedRows[j].assignment.ResourceID+"|"+rankedRows[j].assignment.CoreID
	})
	frontierRankBySlot := map[string]int{}
	for i, row := range rankedRows {
		frontierRankBySlot[row.assignment.ResourceID+"|"+row.assignment.CoreID] = i + 1
	}
	rankedCandidates := []CandidateEvaluation{}
	for _, row := range rows {
		rankedCandidates = append(rankedCandidates, row.candidate)
	}
	sortCandidates(rankedCandidates)
	displayRankBySlot := map[string]int{}
	for i, candidate := range rankedCandidates {
		displayRankBySlot[candidate.ResourceID+"|"+candidate.CoreID] = i + 1
	}
	nextStates := []beamState{}
	for _, row := range rows {
		selectedCandidates := []CandidateEvaluation{}
		for _, ranked := range rankedCandidates {
			item := ranked
			item.Rank = displayRankBySlot[item.ResourceID+"|"+item.CoreID]
			item.Selected = item.ResourceID == row.assignment.ResourceID && item.CoreID == row.assignment.CoreID
			selectedCandidates = append(selectedCandidates, item)
		}
		selectedRank := frontierRankBySlot[row.assignment.ResourceID+"|"+row.assignment.CoreID]
		step := ScheduleStep{Step: stepIndex, TaskID: task.ID, SelectedResourceID: row.assignment.ResourceID, SelectedCoreID: row.assignment.CoreID, SelectedTotalScore: row.assignment.Score.TotalScore, Candidates: selectedCandidates}
		coreAvail := copyFloatMap(state.CoreAvail)
		coreAvail[row.assignment.CoreID] = row.assignment.FinishTime
		nodeHasBooted := copyBoolMap(state.NodeHasBooted)
		nodeReady := copyFloatMap(state.NodeReadyTime)
		nodeLast := copyFloatMap(state.NodeLastActive)
		intervals := append([]MachineStopInterval{}, state.StopIntervals...)
		updateNodeState(row.assignment, ctx.Resources[row.assignment.ResourceID], nodeHasBooted, nodeReady, nodeLast, &intervals)
		partialBudget := round(state.PartialBudgetUsed+row.incBudget, 4)
		partialMakespan := round(maxf(state.PartialMakespan, row.assignment.FinishTime), 3)
		partialScore := round(state.PartialScore+beamFrontierScore(row.assignment.Score, frontier)+float64(selectedRank)*0.0001, 6)
		assignments := append(append([]Assignment{}, state.Assignments...), row.assignment)
		byTask := map[string]Assignment{}
		for key, value := range state.AssignmentByTask {
			byTask[key] = value
		}
		byTask[row.assignment.TaskID] = row.assignment
		nextStates = append(nextStates, beamState{Assignments: assignments, AssignmentByTask: byTask, CoreAvail: coreAvail, NodeHasBooted: nodeHasBooted, NodeReadyTime: nodeReady, NodeLastActive: nodeLast, StopIntervals: intervals, StepTrace: appendStepTrace(state.StepTrace, step), PartialBudgetUsed: partialBudget, PartialMakespan: partialMakespan, PartialScore: partialScore})
	}
	return nextStates
}

func beamFrontierScore(score ScoreBreakdown, frontier beamFrontier) float64 {
	return frontier.WeightTime*score.TimeScore + frontier.WeightCost*score.CostScore
}

func appendStepTrace(prev *scheduleStepTrace, step ScheduleStep) *scheduleStepTrace {
	length := 1
	if prev != nil {
		length = prev.Len + 1
	}
	return &scheduleStepTrace{Step: step, Prev: prev, Len: length}
}

func traceSteps(trace *scheduleStepTrace) []ScheduleStep {
	if trace == nil {
		return []ScheduleStep{}
	}
	steps := make([]ScheduleStep, trace.Len)
	for item := trace; item != nil; item = item.Prev {
		steps[item.Len-1] = item.Step
	}
	return steps
}

func copyFloatMap(in map[string]float64) map[string]float64 {
	out := map[string]float64{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyBoolMap(in map[string]bool) map[string]bool {
	out := map[string]bool{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func selectBeamStates(states []beamState, width int, generated GeneratedSimulation) []beamState {
	if width <= 0 || len(states) == 0 {
		return []beamState{}
	}
	unique := dedupeStates(states)
	unique = preferPartiallyFeasibleStates(unique, generated)
	if len(unique) <= width {
		return unique
	}
	sort.SliceStable(unique, func(i, j int) bool {
		return beamStateLess(unique[i], unique[j])
	})
	return append([]beamState{}, unique[:width]...)
}

func preferPartiallyFeasibleStates(states []beamState, generated GeneratedSimulation) []beamState {
	if len(states) == 0 || (generated.SLA.BudgetLimit == nil && generated.SLA.DeadlineLimit == nil) {
		return states
	}
	feasible := make([]beamState, 0, len(states))
	for _, state := range states {
		if isPartiallyFeasibleState(state, generated) {
			feasible = append(feasible, state)
		}
	}
	if len(feasible) == 0 {
		return states
	}
	return feasible
}

func isPartiallyFeasibleState(state beamState, generated GeneratedSimulation) bool {
	if generated.SLA.BudgetLimit != nil && state.PartialBudgetUsed > *generated.SLA.BudgetLimit {
		return false
	}
	if generated.SLA.DeadlineLimit != nil && state.PartialMakespan > *generated.SLA.DeadlineLimit {
		return false
	}
	return true
}

func beamStateLess(a, b beamState) bool {
	if a.PartialScore != b.PartialScore {
		return a.PartialScore < b.PartialScore
	}
	if a.PartialMakespan != b.PartialMakespan {
		return a.PartialMakespan < b.PartialMakespan
	}
	if a.PartialBudgetUsed != b.PartialBudgetUsed {
		return a.PartialBudgetUsed < b.PartialBudgetUsed
	}
	return stateSignature(a) < stateSignature(b)
}

func dedupeStates(states []beamState) []beamState {
	sort.Slice(states, func(i, j int) bool {
		a, b := states[i], states[j]
		if a.PartialScore != b.PartialScore {
			return a.PartialScore < b.PartialScore
		}
		if a.PartialMakespan != b.PartialMakespan {
			return a.PartialMakespan < b.PartialMakespan
		}
		return a.PartialBudgetUsed < b.PartialBudgetUsed
	})
	seen := map[string]bool{}
	out := []beamState{}
	for _, state := range states {
		sig := stateSignature(state)
		if seen[sig] {
			continue
		}
		seen[sig] = true
		out = append(out, state)
	}
	return out
}

func stateSignature(state beamState) string {
	parts := []string{}
	for _, assignment := range state.Assignments {
		parts = append(parts, assignment.TaskID+":"+assignment.ResourceID)
	}
	return strings.Join(parts, "|")
}

func buildOptions(generated GeneratedSimulation, states []beamState, optionCount int, budgetLimit, deadlineLimit *float64) []ScheduleOption {
	unique := dedupeStates(states)
	built := buildOptionsParallel(generated, unique, budgetLimit, deadlineLimit)
	annotateOptionScores(built, generated)
	ranked := rankOptions(built, optionCount, generated.SLA.WeightTime, generated.SLA.WeightCost)
	if len(ranked) > optionCount {
		ranked = ranked[:optionCount]
	}
	for i := range ranked {
		ranked[i].Rank = i + 1
		ranked[i].ID = fmt.Sprintf("option-%d", i+1)
		ranked[i].Recommended = i == 0
	}
	return ranked
}

func buildOptionsParallel(generated GeneratedSimulation, states []beamState, budgetLimit, deadlineLimit *float64) []ScheduleOption {
	if len(states) == 0 {
		return []ScheduleOption{}
	}
	if len(states) == 1 {
		return []ScheduleOption{buildOption(generated, states[0], budgetLimit, deadlineLimit)}
	}
	workers := min(len(states), runtime.GOMAXPROCS(0))
	built := make([]ScheduleOption, len(states))
	jobs := make(chan int)
	var wg sync.WaitGroup
	wg.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func() {
			defer wg.Done()
			for index := range jobs {
				built[index] = buildOption(generated, states[index], budgetLimit, deadlineLimit)
			}
		}()
	}
	for index := range states {
		jobs <- index
	}
	close(jobs)
	wg.Wait()
	return built
}

func buildOption(generated GeneratedSimulation, state beamState, budgetLimit, deadlineLimit *float64) ScheduleOption {
	copyGenerated := cloneGeneratedForOption(generated)
	result := buildResult(copyGenerated, append([]Assignment{}, state.Assignments...), append([]MachineStopInterval{}, state.StopIntervals...), traceSteps(state.StepTrace))
	budgetUsed, makespan := result.CostVariables.BUsed, result.TimingVariables.Makespan
	budgetViolation, deadlineViolation := 0.0, 0.0
	if budgetLimit != nil {
		budgetViolation = round(maxf(0, budgetUsed-*budgetLimit), 4)
	}
	if deadlineLimit != nil {
		deadlineViolation = round(maxf(0, makespan-*deadlineLimit), 3)
	}
	distribution := machineDistribution(state)
	return ScheduleOption{ID: "pending", Feasible: budgetViolation == 0 && deadlineViolation == 0, BudgetUsed: budgetUsed, BudgetLimit: budgetLimit, BudgetViolation: budgetViolation, Makespan: makespan, DeadlineLimit: deadlineLimit, DeadlineViolation: deadlineViolation, MachineSignature: stateSignature(state), MachineDistribution: distribution, Result: result}
}

func cloneGeneratedForOption(in GeneratedSimulation) GeneratedSimulation {
	out := in
	out.Matrices = in.Matrices
	out.Matrices.ETStar = copyNestedFloatMap(in.Matrices.ETStar)
	return out
}

func copyNestedFloatMap(in map[string]map[string]float64) map[string]map[string]float64 {
	out := make(map[string]map[string]float64, len(in))
	for key, values := range in {
		copied := make(map[string]float64, len(values))
		for innerKey, value := range values {
			copied[innerKey] = value
		}
		out[key] = copied
	}
	return out
}

func machineDistribution(state beamState) map[string]int {
	out := map[string]int{}
	for _, assignment := range state.Assignments {
		out[assignment.ResourceID]++
	}
	return out
}

func annotateOptionScores(options []ScheduleOption, generated GeneratedSimulation) {
	maxBudget, maxMakespan := 1.0, 1.0
	for _, option := range options {
		maxBudget = maxf(maxBudget, option.BudgetUsed)
		maxMakespan = maxf(maxMakespan, option.Makespan)
	}
	for i := range options {
		timeScore := options[i].Makespan / maxMakespan
		costScore := options[i].BudgetUsed / maxBudget
		timeContribution := generated.SLA.WeightTime * timeScore
		costContribution := generated.SLA.WeightCost * costScore
		options[i].WeightedScore = round(timeContribution+costContribution, 6)
		optionTotal := timeScore + costScore
		if optionTotal > 0 {
			options[i].WeightedTimePercent = round(timeScore/optionTotal*100, 1)
			options[i].WeightedCostPercent = round(costScore/optionTotal*100, 1)
		}
		options[i].DiversityScore = round(distributionDiversity(options[i].MachineDistribution), 6)
	}
}

func distributionDiversity(distribution map[string]int) float64 {
	if len(distribution) == 0 {
		return 0
	}
	total, maxValue := 0, 0
	for _, value := range distribution {
		total += value
		if value > maxValue {
			maxValue = value
		}
	}
	return float64(len(distribution)) - float64(maxValue)/float64(max(1, total))
}

func rankOptions(options []ScheduleOption, optionCount int, weightTime, weightCost float64) []ScheduleOption {
	if len(options) == 0 {
		return []ScheduleOption{}
	}
	ranked := append([]ScheduleOption{}, options...)
	sort.SliceStable(ranked, func(i, j int) bool {
		return optionRankLess(ranked[i], ranked[j], weightTime, weightCost)
	})
	if len(ranked) > optionCount {
		return append([]ScheduleOption{}, ranked[:optionCount]...)
	}
	return ranked
}

func optionRankLess(a, b ScheduleOption, weightTime, weightCost float64) bool {
	if a.Feasible != b.Feasible {
		return a.Feasible
	}
	aBudgetRatio, bBudgetRatio := optionBudgetViolationRatio(a), optionBudgetViolationRatio(b)
	aDeadlineRatio, bDeadlineRatio := optionDeadlineViolationRatio(a), optionDeadlineViolationRatio(b)
	if !a.Feasible && !b.Feasible {
		if weightTime > weightCost {
			if aDeadlineRatio != bDeadlineRatio {
				return aDeadlineRatio < bDeadlineRatio
			}
			if aBudgetRatio != bBudgetRatio {
				return aBudgetRatio < bBudgetRatio
			}
		} else if weightCost > weightTime {
			if aBudgetRatio != bBudgetRatio {
				return aBudgetRatio < bBudgetRatio
			}
			if aDeadlineRatio != bDeadlineRatio {
				return aDeadlineRatio < bDeadlineRatio
			}
		} else if aBudgetRatio+aDeadlineRatio != bBudgetRatio+bDeadlineRatio {
			return aBudgetRatio+aDeadlineRatio < bBudgetRatio+bDeadlineRatio
		}
	}
	if aBudgetRatio+aDeadlineRatio != bBudgetRatio+bDeadlineRatio {
		return aBudgetRatio+aDeadlineRatio < bBudgetRatio+bDeadlineRatio
	}
	if a.WeightedScore != b.WeightedScore {
		return a.WeightedScore < b.WeightedScore
	}
	if a.Makespan != b.Makespan {
		return a.Makespan < b.Makespan
	}
	if a.BudgetUsed != b.BudgetUsed {
		return a.BudgetUsed < b.BudgetUsed
	}
	return a.MachineSignature < b.MachineSignature
}

func optionBudgetViolationRatio(option ScheduleOption) float64 {
	if option.BudgetLimit == nil || *option.BudgetLimit <= 0 {
		return 0
	}
	return option.BudgetViolation / *option.BudgetLimit
}

func optionDeadlineViolationRatio(option ScheduleOption) float64 {
	if option.DeadlineLimit == nil || *option.DeadlineLimit <= 0 {
		return 0
	}
	return option.DeadlineViolation / *option.DeadlineLimit
}
