package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

type beamState struct {
	Assignments       []Assignment
	AssignmentByTask  map[string]Assignment
	CoreAvail         map[string]float64
	NodeHasBooted     map[string]bool
	NodeReadyTime     map[string]float64
	NodeLastActive    map[string]float64
	StopIntervals     []MachineStopInterval
	SchedulerSteps    []ScheduleStep
	PartialBudgetUsed float64
	PartialMakespan   float64
	PartialScore      float64
}

func optimizeSchedule(generated GeneratedSimulation) (ScheduleOptimizationResponse, error) {
	optionCount := max(1, min(generated.SLA.OptionCount, 100))
	finalStates, err := beamSearch(generated, optionCount)
	if err != nil {
		return ScheduleOptimizationResponse{}, err
	}
	options := buildOptions(generated, finalStates, optionCount, generated.SLA.BudgetLimit, generated.SLA.DeadlineLimit)
	var selected *string
	if len(options) > 0 {
		selected = &options[0].ID
	}
	return ScheduleOptimizationResponse{SelectedOptionID: selected, Constraints: ScheduleConstraints{BudgetLimit: generated.SLA.BudgetLimit, DeadlineLimit: generated.SLA.DeadlineLimit, OptionCount: optionCount}, Options: options}, nil
}

func beamSearch(generated GeneratedSimulation, optionCount int) ([]beamState, error) {
	coreAvail := map[string]float64{}
	for _, resource := range generated.Resources {
		for _, core := range resource.Cores {
			coreAvail[core.ID] = 0
		}
	}
	hasBooted, ready, last := initialNodeState(generated.Resources)
	beam := []beamState{{Assignments: []Assignment{}, AssignmentByTask: map[string]Assignment{}, CoreAvail: coreAvail, NodeHasBooted: hasBooted, NodeReadyTime: ready, NodeLastActive: last, StopIntervals: []MachineStopInterval{}, SchedulerSteps: []ScheduleStep{}}}
	overflow := []beamState{}
	beamWidth := min(600, max(120, optionCount*6))
	overflowWidth := min(240, max(60, optionCount*3))
	order, err := topologicalOrder(generated)
	if err != nil {
		return nil, err
	}
	for stepIndex, taskID := range order {
		expanded, overflowExpanded := []beamState{}, []beamState{}
		for _, state := range append(append([]beamState{}, beam...), overflow...) {
			nextStates := expandState(generated, state, stepIndex+1, taskID)
			for _, next := range nextStates {
				if generated.SLA.BudgetLimit != nil && next.PartialBudgetUsed > *generated.SLA.BudgetLimit {
					overflowExpanded = append(overflowExpanded, next)
				} else {
					expanded = append(expanded, next)
				}
			}
		}
		if len(expanded) == 0 && len(overflowExpanded) == 0 {
			return nil, fmt.Errorf("No feasible resource for task %s", taskID)
		}
		beam = selectBeamStates(expanded, beamWidth, generated)
		overflow = selectBeamStates(overflowExpanded, overflowWidth, generated)
		if len(beam) == 0 {
			overflow = selectBeamStates(overflow, overflowWidth, generated)
		}
	}
	return append(beam, overflow...), nil
}

func expandState(generated GeneratedSimulation, state beamState, stepIndex int, taskID string) []beamState {
	tasks, resources := taskMap(generated.Workflow.Tasks), resourceMap(generated.Resources)
	depsByTarget := dependenciesByTarget(generated.Workflow.Dependencies)
	task := tasks[taskID]
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
		predecessorFloor, transferTotal := predecessorTiming(depsByTarget[task.ID], state.AssignmentByTask, generated, resource.ID)
		networkCost := 0.0
		for _, dep := range depsByTarget[task.ID] {
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
			TotalScore:        round(generated.SLA.WeightTime*timeScore+generated.SLA.WeightCost*costScore, 5),
		}
		rows[i].assignment.Score = score
		rows[i].candidate.Score = score
	}
	rankedCandidates := []CandidateEvaluation{}
	for _, row := range rows {
		rankedCandidates = append(rankedCandidates, row.candidate)
	}
	sortCandidates(rankedCandidates)
	rankBySlot := map[string]int{}
	for i, candidate := range rankedCandidates {
		rankBySlot[candidate.ResourceID+"|"+candidate.CoreID] = i + 1
	}
	nextStates := []beamState{}
	for _, row := range rows {
		selectedCandidates := []CandidateEvaluation{}
		for _, ranked := range rankedCandidates {
			item := ranked
			item.Rank = rankBySlot[item.ResourceID+"|"+item.CoreID]
			item.Selected = item.ResourceID == row.assignment.ResourceID && item.CoreID == row.assignment.CoreID
			selectedCandidates = append(selectedCandidates, item)
		}
		selectedRank := rankBySlot[row.assignment.ResourceID+"|"+row.assignment.CoreID]
		step := ScheduleStep{Step: stepIndex, TaskID: task.ID, SelectedResourceID: row.assignment.ResourceID, SelectedCoreID: row.assignment.CoreID, SelectedTotalScore: row.assignment.Score.TotalScore, Candidates: selectedCandidates}
		coreAvail := copyFloatMap(state.CoreAvail)
		coreAvail[row.assignment.CoreID] = row.assignment.FinishTime
		nodeHasBooted := copyBoolMap(state.NodeHasBooted)
		nodeReady := copyFloatMap(state.NodeReadyTime)
		nodeLast := copyFloatMap(state.NodeLastActive)
		intervals := append([]MachineStopInterval{}, state.StopIntervals...)
		updateNodeState(row.assignment, resources[row.assignment.ResourceID], nodeHasBooted, nodeReady, nodeLast, &intervals)
		partialBudget := round(state.PartialBudgetUsed+row.incBudget, 4)
		partialMakespan := round(maxf(state.PartialMakespan, row.assignment.FinishTime), 3)
		partialScore := round(state.PartialScore+row.assignment.Score.TotalScore+limitRatio(partialMakespan, generated.SLA.DeadlineLimit)+limitRatio(partialBudget, generated.SLA.BudgetLimit)+float64(selectedRank)*0.0001, 6)
		assignments := append(append([]Assignment{}, state.Assignments...), row.assignment)
		byTask := map[string]Assignment{}
		for key, value := range state.AssignmentByTask {
			byTask[key] = value
		}
		byTask[row.assignment.TaskID] = row.assignment
		nextStates = append(nextStates, beamState{Assignments: assignments, AssignmentByTask: byTask, CoreAvail: coreAvail, NodeHasBooted: nodeHasBooted, NodeReadyTime: nodeReady, NodeLastActive: nodeLast, StopIntervals: intervals, SchedulerSteps: append(append([]ScheduleStep{}, state.SchedulerSteps...), step), PartialBudgetUsed: partialBudget, PartialMakespan: partialMakespan, PartialScore: partialScore})
	}
	return nextStates
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

func limitRatio(value float64, limit *float64) float64 {
	if limit == nil || *limit <= 0 {
		return 0
	}
	return maxf(0, value/(*limit)-1)
}

func selectBeamStates(states []beamState, width int, generated GeneratedSimulation) []beamState {
	if width <= 0 || len(states) == 0 {
		return []beamState{}
	}
	unique := dedupeStates(states)
	if len(unique) <= width {
		return unique
	}
	sort.Slice(unique, func(i, j int) bool {
		a, b := unique[i], unique[j]
		if a.PartialScore != b.PartialScore {
			return a.PartialScore < b.PartialScore
		}
		if a.PartialMakespan != b.PartialMakespan {
			return a.PartialMakespan < b.PartialMakespan
		}
		return a.PartialBudgetUsed < b.PartialBudgetUsed
	})
	return append([]beamState{}, unique[:width]...)
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
	built := []ScheduleOption{}
	for _, state := range unique {
		copyGenerated := cloneGenerated(generated)
		result := buildResult(copyGenerated, append([]Assignment{}, state.Assignments...), append([]MachineStopInterval{}, state.StopIntervals...), append([]ScheduleStep{}, state.SchedulerSteps...))
		budgetUsed, makespan := result.CostVariables.BUsed, result.TimingVariables.Makespan
		budgetViolation, deadlineViolation := 0.0, 0.0
		if budgetLimit != nil {
			budgetViolation = round(maxf(0, budgetUsed-*budgetLimit), 4)
		}
		if deadlineLimit != nil {
			deadlineViolation = round(maxf(0, makespan-*deadlineLimit), 3)
		}
		distribution := machineDistribution(state)
		built = append(built, ScheduleOption{ID: "pending", Feasible: budgetViolation == 0 && deadlineViolation == 0, BudgetUsed: budgetUsed, BudgetLimit: budgetLimit, BudgetViolation: budgetViolation, Makespan: makespan, DeadlineLimit: deadlineLimit, DeadlineViolation: deadlineViolation, MachineSignature: stateSignature(state), MachineDistribution: distribution, Result: result})
	}
	feasible := []ScheduleOption{}
	for _, option := range built {
		if option.Feasible {
			feasible = append(feasible, option)
		}
	}
	source := built
	if len(feasible) > 0 {
		source = feasible
	}
	annotateOptionScores(source, generated)
	ranked := rankOptions(source, optionCount)
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

func cloneGenerated(in GeneratedSimulation) GeneratedSimulation {
	data, _ := json.Marshal(in)
	var out GeneratedSimulation
	_ = json.Unmarshal(data, &out)
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
		penalty := options[i].BudgetViolation + options[i].DeadlineViolation
		options[i].WeightedScore = round(generated.SLA.WeightTime*timeScore+generated.SLA.WeightCost*costScore+penalty, 6)
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

func rankOptions(options []ScheduleOption, optionCount int) []ScheduleOption {
	if len(options) == 0 {
		return []ScheduleOption{}
	}
	sort.Slice(options, func(i, j int) bool {
		a, b := options[i], options[j]
		if a.WeightedScore != b.WeightedScore {
			return a.WeightedScore < b.WeightedScore
		}
		if a.Makespan != b.Makespan {
			return a.Makespan < b.Makespan
		}
		return a.BudgetUsed < b.BudgetUsed
	})
	selected := []ScheduleOption{options[0]}
	remaining := append([]ScheduleOption{}, options[1:]...)
	for len(remaining) > 0 && len(selected) < optionCount {
		bestIndex := 0
		bestKey := math.Inf(-1)
		for i, option := range remaining {
			distance := optionDistanceToSelected(option, selected)
			key := distance*1000000 - option.WeightedScore*1000 + option.DiversityScore - option.Makespan*0.0001 - option.BudgetUsed*0.0001
			if key > bestKey {
				bestKey = key
				bestIndex = i
			}
		}
		selected = append(selected, remaining[bestIndex])
		remaining = append(remaining[:bestIndex], remaining[bestIndex+1:]...)
	}
	return selected
}

func optionDistanceToSelected(candidate ScheduleOption, selected []ScheduleOption) float64 {
	candidateSlots := splitSignature(candidate.MachineSignature)
	best := math.Inf(1)
	for _, option := range selected {
		best = minf(best, symmetricDistance(candidateSlots, splitSignature(option.MachineSignature)))
	}
	if math.IsInf(best, 1) {
		return float64(len(candidateSlots))
	}
	return best
}

func splitSignature(signature string) map[string]bool {
	out := map[string]bool{}
	if signature == "" {
		return out
	}
	for _, part := range strings.Split(signature, "|") {
		out[part] = true
	}
	return out
}

func symmetricDistance(left, right map[string]bool) float64 {
	count := 0
	for key := range left {
		if !right[key] {
			count++
		}
	}
	for key := range right {
		if !left[key] {
			count++
		}
	}
	return float64(count)
}
