package main

import (
	"errors"
	"fmt"
	"sort"
)

func candidatePairwiseInterference(generated GeneratedSimulation, taskID, resourceID string, scheduled []Assignment, start, finish float64) (float64, []PairwiseInterference) {
	colocated := []string{}
	for _, item := range scheduled {
		if item.ResourceID == resourceID && maxf(start, item.StartTime) < minf(finish, item.FinishTime) {
			colocated = append(colocated, item.TaskID)
		}
	}
	if len(colocated) == 0 {
		return 0, []PairwiseInterference{}
	}
	total, count := 0.0, 0
	pairs := []PairwiseInterference{}
	for _, otherID := range colocated {
		dimensions := map[string]float64{}
		keys := sortedKeys4(generated.Matrices.InterferenceIN[resourceID])
		for _, dimensionName := range keys {
			value := generated.Matrices.InterferenceIN[resourceID][dimensionName][otherID][taskID]
			total += value
			count++
			dimensions[dimensionName] = value
		}
		sum := 0.0
		for _, value := range dimensions {
			sum += value
		}
		pairs = append(pairs, PairwiseInterference{OtherTaskID: otherID, Value: round(sum/float64(max(1, len(dimensions))), 4), Dimensions: dimensions})
	}
	return round(total/float64(max(1, count)), 4), pairs
}

func sortedKeys4(m map[string]map[string]map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func scheduleWorkflow(generated GeneratedSimulation) (SimulationResult, error) {
	tasks := taskMap(generated.Workflow.Tasks)
	resources := resourceMap(generated.Resources)
	depsByTarget := dependenciesByTarget(generated.Workflow.Dependencies)
	coreAvail := map[string]float64{}
	for _, resource := range generated.Resources {
		for _, core := range resource.Cores {
			coreAvail[core.ID] = 0
		}
	}
	nodeHasBooted, nodeReady, nodeLast := initialNodeState(generated.Resources)
	assignments := []Assignment{}
	assignmentByTask := map[string]Assignment{}
	stopIntervals := []MachineStopInterval{}
	steps := []ScheduleStep{}
	order, err := topologicalOrder(generated)
	if err != nil {
		return SimulationResult{}, err
	}
	for stepIndex, taskID := range order {
		task := tasks[taskID]
		candidates := []Assignment{}
		evaluations := []CandidateEvaluation{}
		type row struct {
			assignment Assignment
			candidate  CandidateEvaluation
			finish     float64
			rawCost    float64
			phi        float64
		}
		rows := []row{}
		for _, resource := range generated.Resources {
			if task.CPU > resource.CPU || task.Memory > resource.Memory {
				continue
			}
			predecessorFloor, transferTotal := predecessorTiming(depsByTarget[task.ID], assignmentByTask, generated, resource.ID)
			for _, core := range resource.Cores {
				readyFloor := maxf(predecessorFloor, coreAvail[core.ID], nodeReady[resource.ID])
				lastActive := nodeLast[resource.ID]
				stopBoot := resource.Kind == "cloud" && nodeHasBooted[resource.ID] && resource.BootOverhead > 0 && readyFloor-lastActive >= resource.BootOverhead
				coldBoot := !nodeHasBooted[resource.ID]
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
				phi, pairwise := candidatePairwiseInterference(generated, task.ID, resource.ID, assignments, start, start+baseRuntime)
				effective := round(baseRuntime*(1+phi), 3)
				finish := round(start+effective, 3)
				rawCost := effective * (task.CPU*resource.PricePerCPUSecond + task.Memory*resource.PricePerGBSecond)
				score := ScoreBreakdown{}
				assignment := Assignment{TaskID: task.ID, ResourceID: resource.ID, CoreID: core.ID, StartTime: start, FinishTime: finish, EffectiveRuntime: effective, TransferDelay: round(transferTotal, 3), BootOverhead: boot, ContainerOverhead: container, PhiN: phi, PredecessorFinishFloor: round(predecessorFloor, 3), Score: score}
				candidate := CandidateEvaluation{TaskID: task.ID, ResourceID: resource.ID, CoreID: core.ID, StartTime: start, FinishTime: finish, BaseRuntime: baseRuntime, EffectiveRuntime: effective, InterferenceTime: round(effective-baseRuntime, 3), TransferDelay: round(transferTotal, 3), BootOverhead: boot, ContainerOverhead: container, PredecessorFinishFloor: round(predecessorFloor, 3), RawCost: round(rawCost, 4), PhiN: phi, PairwiseInterference: pairwise, Score: score}
				candidates = append(candidates, assignment)
				evaluations = append(evaluations, candidate)
				rows = append(rows, row{assignment, candidate, finish, rawCost, phi})
			}
		}
		if len(candidates) == 0 {
			return SimulationResult{}, fmt.Errorf("No feasible resource for task %s", task.ID)
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
		candidates = candidates[:0]
		evaluations = evaluations[:0]
		for _, row := range rows {
			candidates = append(candidates, row.assignment)
			evaluations = append(evaluations, row.candidate)
		}
		sortAssignments(candidates)
		selected := candidates[0]
		ranked := rankCandidates(evaluations, selected)
		steps = append(steps, ScheduleStep{Step: stepIndex + 1, TaskID: task.ID, SelectedResourceID: selected.ResourceID, SelectedCoreID: selected.CoreID, SelectedTotalScore: selected.Score.TotalScore, Candidates: ranked})
		assignments = append(assignments, selected)
		assignmentByTask[selected.TaskID] = selected
		coreAvail[selected.CoreID] = selected.FinishTime
		updateNodeState(selected, resources[selected.ResourceID], nodeHasBooted, nodeReady, nodeLast, &stopIntervals)
		generated.Matrices.ETStar[selected.TaskID][selected.ResourceID] = selected.EffectiveRuntime
	}
	return buildResult(generated, assignments, stopIntervals, steps), nil
}

func taskMap(tasks []Task) map[string]Task {
	out := map[string]Task{}
	for _, task := range tasks {
		out[task.ID] = task
	}
	return out
}

func resourceMap(resources []Resource) map[string]Resource {
	out := map[string]Resource{}
	for _, resource := range resources {
		out[resource.ID] = resource
	}
	return out
}

func dependenciesByTarget(deps []Dependency) map[string][]Dependency {
	out := map[string][]Dependency{}
	for _, dep := range deps {
		out[dep.Target] = append(out[dep.Target], dep)
	}
	return out
}

func initialNodeState(resources []Resource) (map[string]bool, map[string]float64, map[string]float64) {
	hasBooted, ready, last := map[string]bool{}, map[string]float64{}, map[string]float64{}
	for _, resource := range resources {
		hasBooted[resource.ID] = resource.Status == "warm"
		ready[resource.ID] = 0
		last[resource.ID] = 0
	}
	return hasBooted, ready, last
}

func predecessorTiming(deps []Dependency, assignmentByTask map[string]Assignment, generated GeneratedSimulation, resourceID string) (float64, float64) {
	predecessorFloor, transferTotal := 0.0, 0.0
	for _, dep := range deps {
		predecessor := assignmentByTask[dep.Source]
		transfer := 0.0
		if predecessor.ResourceID != resourceID {
			transfer = dep.DataMB / generated.Matrices.BandwidthBW[predecessor.ResourceID][resourceID]
		}
		transferTotal += transfer
		predecessorFloor = maxf(predecessorFloor, predecessor.FinishTime+transfer)
	}
	return predecessorFloor, transferTotal
}

func sortAssignments(assignments []Assignment) {
	sort.Slice(assignments, func(i, j int) bool {
		a, b := assignments[i], assignments[j]
		if a.Score.TotalScore != b.Score.TotalScore {
			return a.Score.TotalScore < b.Score.TotalScore
		}
		if a.FinishTime != b.FinishTime {
			return a.FinishTime < b.FinishTime
		}
		if a.ResourceID != b.ResourceID {
			return a.ResourceID < b.ResourceID
		}
		return a.CoreID < b.CoreID
	})
}

func sortCandidates(candidates []CandidateEvaluation) {
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.Score.TotalScore != b.Score.TotalScore {
			return a.Score.TotalScore < b.Score.TotalScore
		}
		if a.FinishTime != b.FinishTime {
			return a.FinishTime < b.FinishTime
		}
		if a.ResourceID != b.ResourceID {
			return a.ResourceID < b.ResourceID
		}
		return a.CoreID < b.CoreID
	})
}

func rankCandidates(candidates []CandidateEvaluation, selected Assignment) []CandidateEvaluation {
	ranked := append([]CandidateEvaluation{}, candidates...)
	sortCandidates(ranked)
	for i := range ranked {
		ranked[i].Rank = i + 1
		ranked[i].Selected = ranked[i].ResourceID == selected.ResourceID && ranked[i].CoreID == selected.CoreID
	}
	return ranked
}

func updateNodeState(selected Assignment, resource Resource, hasBooted map[string]bool, ready map[string]float64, last map[string]float64, intervals *[]MachineStopInterval) {
	if selected.BootOverhead > 0 {
		previousActive := last[selected.ResourceID]
		bootFinish := round(selected.StartTime-selected.ContainerOverhead, 3)
		if resource.Kind == "cloud" && hasBooted[selected.ResourceID] && bootFinish-previousActive >= selected.BootOverhead {
			*intervals = append(*intervals, MachineStopInterval{ResourceID: selected.ResourceID, StopTime: round(previousActive, 3), BootStartTime: round(bootFinish-selected.BootOverhead, 3), BootFinishTime: bootFinish, BootOverhead: selected.BootOverhead, Reason: fmt.Sprintf("idle gap paid boot before %s", selected.TaskID)})
		}
		ready[selected.ResourceID] = maxf(ready[selected.ResourceID], bootFinish)
	}
	hasBooted[selected.ResourceID] = true
	last[selected.ResourceID] = maxf(last[selected.ResourceID], selected.FinishTime)
}

func topologicalOrder(generated GeneratedSimulation) ([]string, error) {
	remaining := map[string]bool{}
	predecessors := map[string]map[string]bool{}
	for _, task := range generated.Workflow.Tasks {
		remaining[task.ID] = true
		predecessors[task.ID] = map[string]bool{}
		for _, pred := range task.Predecessors {
			predecessors[task.ID][pred] = true
		}
	}
	order := []string{}
	for len(remaining) > 0 {
		ready := []string{}
		for taskID := range remaining {
			ok := true
			for pred := range predecessors[taskID] {
				if !containsString(order, pred) {
					ok = false
					break
				}
			}
			if ok {
				ready = append(ready, taskID)
			}
		}
		if len(ready) == 0 {
			return nil, errors.New("Workflow contains a cycle")
		}
		sort.Strings(ready)
		for _, taskID := range ready {
			order = append(order, taskID)
			delete(remaining, taskID)
		}
	}
	return order, nil
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
