package main

import (
	"fmt"
	"sort"
)

type classicMachineSlot struct {
	Start float64
	End   float64
}

// scheduleHEFTClassic implements the canonical two-phase HEFT policy:
// upward-rank task priority followed by insertion-based earliest-finish-time
// machine selection. Each Resource is an indivisible processor, so tasks never
// overlap on the same machine and controlled co-location interference is not
// activated.
func scheduleHEFTClassic(generated GeneratedSimulation) (SimulationResult, error) {
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
	depsByTarget := dependenciesByTarget(generated.Workflow.Dependencies)
	machineSlots := map[string][]classicMachineSlot{}
	machineBooted := map[string]bool{}
	assignments := []Assignment{}
	assignmentByTask := map[string]Assignment{}
	steps := []ScheduleStep{}

	for stepIndex, taskID := range order {
		task := tasks[taskID]
		type classicCandidate struct {
			assignment   Assignment
			evaluation   CandidateEvaluation
			reservedSlot classicMachineSlot
		}
		candidates := []classicCandidate{}
		for _, resource := range generated.Resources {
			if task.CPU > resource.CPU || task.Memory > resource.Memory {
				continue
			}
			predecessorFloor, transferTotal := predecessorTiming(
				depsByTarget[task.ID], assignmentByTask, generated, resource.ID,
			)
			boot := 0.0
			if !machineBooted[resource.ID] {
				boot = resource.BootOverhead
			}
			container := generated.Matrices.ContainerOverhead[task.ID][resource.ID]
			runtime := generated.Matrices.ET0[task.ID][resource.ID]
			reservationDuration := boot + container + runtime
			reservationStart := earliestClassicSlot(
				machineSlots[resource.ID], predecessorFloor, reservationDuration,
			)
			start := round(reservationStart+boot+container, 3)
			finish := round(start+runtime, 3)
			coreID := resource.ID + "-machine-slot"
			if len(resource.Cores) > 0 {
				coreID = resource.Cores[0].ID
			}
			assignment := Assignment{
				TaskID: task.ID, ResourceID: resource.ID, CoreID: coreID,
				StartTime: start, FinishTime: finish, EffectiveRuntime: runtime,
				TransferDelay: round(transferTotal, 3), BootOverhead: boot,
				ContainerOverhead: container, PhiN: 0,
				PredecessorFinishFloor: round(predecessorFloor, 3),
			}
			evaluation := CandidateEvaluation{
				TaskID: task.ID, ResourceID: resource.ID, CoreID: coreID,
				StartTime: start, FinishTime: finish, BaseRuntime: runtime,
				EffectiveRuntime: runtime, InterferenceTime: 0,
				TransferDelay: round(transferTotal, 3), BootOverhead: boot,
				ContainerOverhead:      container,
				PredecessorFinishFloor: round(predecessorFloor, 3),
				RawCost:                round(incrementalMachineActiveCost(assignments, assignment, resource), 4),
				PhiN:                   0, PairwiseInterference: []PairwiseInterference{},
			}
			candidates = append(candidates, classicCandidate{
				assignment: assignment, evaluation: evaluation,
				reservedSlot: classicMachineSlot{Start: reservationStart, End: finish},
			})
		}
		if len(candidates) == 0 {
			return SimulationResult{}, fmt.Errorf("No feasible resource for task %s", task.ID)
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].assignment.FinishTime != candidates[j].assignment.FinishTime {
				return candidates[i].assignment.FinishTime < candidates[j].assignment.FinishTime
			}
			return candidates[i].assignment.ResourceID < candidates[j].assignment.ResourceID
		})
		selected := candidates[0]
		evaluations := make([]CandidateEvaluation, 0, len(candidates))
		for _, candidate := range candidates {
			evaluations = append(evaluations, candidate.evaluation)
		}
		steps = append(steps, ScheduleStep{
			Step: stepIndex + 1, TaskID: task.ID,
			SelectedResourceID: selected.assignment.ResourceID,
			SelectedCoreID:     selected.assignment.CoreID,
			Candidates:         rankCandidates(evaluations, selected.assignment),
		})
		assignments = append(assignments, selected.assignment)
		assignmentByTask[task.ID] = selected.assignment
		resourceID := selected.assignment.ResourceID
		machineSlots[resourceID] = insertClassicSlot(machineSlots[resourceID], selected.reservedSlot)
		machineBooted[resourceID] = true
	}

	return buildResult(generated, assignments, []MachineStopInterval{}, steps), nil
}

func earliestClassicSlot(slots []classicMachineSlot, readyTime, duration float64) float64 {
	candidate := readyTime
	for _, slot := range slots {
		if candidate+duration <= slot.Start {
			return candidate
		}
		if candidate < slot.End {
			candidate = slot.End
		}
	}
	return candidate
}

func insertClassicSlot(slots []classicMachineSlot, selected classicMachineSlot) []classicMachineSlot {
	out := append(append([]classicMachineSlot{}, slots...), selected)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Start != out[j].Start {
			return out[i].Start < out[j].Start
		}
		return out[i].End < out[j].End
	})
	return out
}
