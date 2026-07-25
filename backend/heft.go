package main

import (
	"fmt"
	"sort"
)

func scheduleHEFT(generated GeneratedSimulation) (SimulationResult, error) {
	ranks, err := heftUpwardRanks(generated)
	if err != nil {
		return SimulationResult{}, err
	}
	order := make([]string, 0, len(generated.Workflow.Tasks))
	for _, task := range generated.Workflow.Tasks {
		order = append(order, task.ID)
	}
	sort.SliceStable(order, func(i, j int) bool {
		if ranks[order[i]] != ranks[order[j]] {
			return ranks[order[i]] > ranks[order[j]]
		}
		return order[i] < order[j]
	})

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
	steps := []ScheduleStep{}
	intervals := []MachineStopInterval{}

	for stepIndex, taskID := range order {
		task := tasks[taskID]
		type heftCandidate struct {
			assignment Assignment
			evaluation CandidateEvaluation
		}
		candidates := []heftCandidate{}
		for _, resource := range generated.Resources {
			if task.CPU > resource.CPU || task.Memory > resource.Memory {
				continue
			}
			predecessorFloor, transferTotal := predecessorTiming(depsByTarget[task.ID], assignmentByTask, generated, resource.ID)
			core := earliestAvailableCore(resource, coreAvail)
			readyFloor := maxf(predecessorFloor, coreAvail[core.ID], nodeReady[resource.ID])
			stopBoot := resource.Kind == "cloud" && nodeHasBooted[resource.ID] && resource.BootOverhead > 0 && readyFloor-nodeLast[resource.ID] >= resource.BootOverhead
			coldBoot := !nodeHasBooted[resource.ID]
			boot := 0.0
			if coldBoot || stopBoot {
				boot = resource.BootOverhead
			}
			container := generated.Matrices.ContainerOverhead[task.ID][resource.ID]
			if coldBoot {
				readyFloor += boot
			}
			start := round(readyFloor+container, 3)
			baseRuntime := generated.Matrices.ET0[task.ID][resource.ID]
			phi, pairwise := candidatePairwiseInterference(generated, task.ID, resource.ID, assignments, start, start+baseRuntime)
			effective := round(baseRuntime*(1+phi), 3)
			finish := round(start+effective, 3)
			assignment := Assignment{
				TaskID: task.ID, ResourceID: resource.ID, CoreID: core.ID, StartTime: start, FinishTime: finish,
				EffectiveRuntime: effective, TransferDelay: round(transferTotal, 3), BootOverhead: boot,
				ContainerOverhead: container, PhiN: phi, PredecessorFinishFloor: round(predecessorFloor, 3),
			}
			rawCost := incrementalMachineActiveCost(assignments, assignment, resource)
			evaluation := CandidateEvaluation{
				TaskID: task.ID, ResourceID: resource.ID, CoreID: core.ID, StartTime: start, FinishTime: finish,
				BaseRuntime: baseRuntime, EffectiveRuntime: effective, InterferenceTime: round(effective-baseRuntime, 3),
				TransferDelay: round(transferTotal, 3), BootOverhead: boot, ContainerOverhead: container,
				PredecessorFinishFloor: round(predecessorFloor, 3), RawCost: round(rawCost, 4),
				PhiN: phi, PairwiseInterference: pairwise,
			}
			candidates = append(candidates, heftCandidate{assignment: assignment, evaluation: evaluation})
		}
		if len(candidates) == 0 {
			return SimulationResult{}, fmt.Errorf("No feasible resource for task %s", task.ID)
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].assignment.FinishTime != candidates[j].assignment.FinishTime {
				return candidates[i].assignment.FinishTime < candidates[j].assignment.FinishTime
			}
			if candidates[i].assignment.ResourceID != candidates[j].assignment.ResourceID {
				return candidates[i].assignment.ResourceID < candidates[j].assignment.ResourceID
			}
			return candidates[i].assignment.CoreID < candidates[j].assignment.CoreID
		})
		selected := candidates[0].assignment
		evaluations := make([]CandidateEvaluation, 0, len(candidates))
		for _, candidate := range candidates {
			evaluations = append(evaluations, candidate.evaluation)
		}
		steps = append(steps, ScheduleStep{
			Step: stepIndex + 1, TaskID: task.ID, SelectedResourceID: selected.ResourceID,
			SelectedCoreID: selected.CoreID, Candidates: rankCandidates(evaluations, selected),
		})
		assignments = append(assignments, selected)
		assignmentByTask[selected.TaskID] = selected
		coreAvail[selected.CoreID] = selected.FinishTime
		updateNodeState(selected, resources[selected.ResourceID], nodeHasBooted, nodeReady, nodeLast, &intervals)
	}
	return buildResult(generated, assignments, intervals, steps), nil
}

func heftUpwardRanks(generated GeneratedSimulation) (map[string]float64, error) {
	if _, err := topologicalOrder(generated); err != nil {
		return nil, err
	}
	successors := map[string][]Dependency{}
	for _, dependency := range generated.Workflow.Dependencies {
		successors[dependency.Source] = append(successors[dependency.Source], dependency)
	}
	ranks := map[string]float64{}
	visiting := map[string]bool{}
	var rank func(string) float64
	rank = func(taskID string) float64 {
		if value, ok := ranks[taskID]; ok {
			return value
		}
		if visiting[taskID] {
			return 0
		}
		visiting[taskID] = true
		computation := 0.0
		for _, resource := range generated.Resources {
			computation += generated.Matrices.ET0[taskID][resource.ID]
		}
		computation /= float64(max(1, len(generated.Resources)))
		maxSuccessor := 0.0
		for _, dependency := range successors[taskID] {
			communication := 0.0
			pairs := 0
			for _, left := range generated.Resources {
				for _, right := range generated.Resources {
					if left.ID == right.ID {
						continue
					}
					communication += dependency.DataMB / maxf(generated.Matrices.BandwidthBW[left.ID][right.ID], 0.001)
					pairs++
				}
			}
			communication /= float64(max(1, pairs))
			maxSuccessor = maxf(maxSuccessor, communication+rank(dependency.Target))
		}
		visiting[taskID] = false
		ranks[taskID] = round(computation+maxSuccessor, 6)
		return ranks[taskID]
	}
	for _, task := range generated.Workflow.Tasks {
		rank(task.ID)
	}
	return ranks, nil
}
