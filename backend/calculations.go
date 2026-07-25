package main

func buildResult(generated GeneratedSimulation, assignments []Assignment, intervals []MachineStopInterval, steps []ScheduleStep) SimulationResult {
	for _, assignment := range assignments {
		generated.Matrices.ETStar[assignment.TaskID][assignment.ResourceID] = assignment.EffectiveRuntime
	}
	timing := calculateTiming(assignments)
	costs := calculateCost(generated, assignments)
	interference := calculateInterference(generated, assignments)
	deviation := calculateDeviation(generated, assignments)
	availN := map[string]float64{}
	for _, resource := range generated.Resources {
		maxFinish := 0.0
		for _, assignment := range assignments {
			if assignment.ResourceID == resource.ID {
				maxFinish = maxf(maxFinish, assignment.FinishTime)
			}
		}
		availN[resource.ID] = maxFinish
	}
	x, f, s := map[string]string{}, map[string]string{}, map[string]float64{}
	for _, item := range assignments {
		x[item.TaskID] = item.ResourceID
		f[item.TaskID] = item.CoreID
		s[item.TaskID] = item.StartTime
	}
	return SimulationResult{ID: generated.ID, Seed: generated.Seed, Workflow: generated.Workflow, Resources: generated.Resources, SLA: generated.SLA, Matrices: generated.Matrices, Assignments: assignments, MachineStopIntervals: intervals, SchedulerSteps: steps, SchedulerVariables: SchedulerVariables{XTN: x, FTTask: f, SNT: s, AvailN: availN, ST: timing.ST, FT: timing.FT, Makespan: timing.Makespan, BUsed: costs.BUsed}, TimingVariables: timing, CostVariables: costs, InterferenceVariables: interference, DeviationVariables: deviation, Experimental: generated.Experimental}
}

func calculateTiming(assignments []Assignment) TimingVariables {
	st, ft, avail := map[string]float64{}, map[string]float64{}, map[string]float64{}
	makespan := 0.0
	transfer, boot, container := map[string]float64{}, map[string]float64{}, map[string]float64{}
	for _, item := range assignments {
		st[item.TaskID] = item.StartTime
		ft[item.TaskID] = item.FinishTime
		avail[item.CoreID] = maxf(avail[item.CoreID], item.FinishTime)
		makespan = maxf(makespan, item.FinishTime)
		transfer[item.TaskID] = item.TransferDelay
		boot[item.TaskID] = item.BootOverhead
		container[item.TaskID] = item.ContainerOverhead
	}
	return TimingVariables{ST: st, FT: ft, Avail: avail, Makespan: round(makespan, 3), TransferDelayByTask: transfer, BootOverheadByTask: boot, ContainerOverheadByTask: container}
}

func calculateCost(generated GeneratedSimulation, assignments []Assignment) CostVariables {
	resources, tasks := resourceMap(generated.Resources), taskMap(generated.Workflow.Tasks)
	assignmentByTask := map[string]Assignment{}
	for _, item := range assignments {
		assignmentByTask[item.TaskID] = item
	}
	cCPU, cMem, cFin, fc := map[string]float64{}, map[string]float64{}, map[string]float64{}, map[string]float64{}
	cTN := map[string]map[string]float64{}
	for _, task := range generated.Workflow.Tasks {
		cTN[task.ID] = map[string]float64{}
		for _, resource := range generated.Resources {
			runtime := generated.Matrices.ETStar[task.ID][resource.ID] + generated.Matrices.ContainerOverhead[task.ID][resource.ID]
			cTN[task.ID][resource.ID] = round(runtime*resource.PricePerHourUSD/3600, 6)
		}
	}
	assignmentsByResource := map[string][]Assignment{}
	for _, assignment := range assignments {
		assignmentsByResource[assignment.ResourceID] = append(assignmentsByResource[assignment.ResourceID], assignment)
	}
	resourceMachineCost := map[string]float64{}
	resourceRuntimeTotal := map[string]float64{}
	for resourceID, resourceAssignments := range assignmentsByResource {
		resource := resources[resourceID]
		resourceMachineCost[resourceID] = machineActiveCost(resourceAssignments, resource)
		for _, assignment := range resourceAssignments {
			resourceRuntimeTotal[resourceID] += assignment.EffectiveRuntime
		}
	}
	for _, assignment := range assignments {
		task, resource := tasks[assignment.TaskID], resources[assignment.ResourceID]
		share := assignment.EffectiveRuntime / maxf(resourceRuntimeTotal[resource.ID], 0.001)
		cCPU[task.ID] = round(resourceMachineCost[resource.ID]*share, 6)
		cMem[task.ID] = 0
		networkCost := 0.0
		for _, dep := range generated.Workflow.Dependencies {
			if dep.Target != task.ID {
				continue
			}
			predecessor := assignmentByTask[dep.Source]
			networkCost += dep.DataMB * generated.Matrices.FinancialNetworkCost[predecessor.ResourceID][assignment.ResourceID]
		}
		cFin[task.ID] = round(networkCost, 6)
		fc[task.ID] = round(cCPU[task.ID]+cMem[task.ID]+cFin[task.ID], 6)
	}
	bUsed, pcc := 0.0, 0.0
	for _, value := range fc {
		bUsed += value
	}
	for _, value := range cFin {
		pcc += value
	}
	bUsed, pcc = round(bUsed, 6), round(pcc, 6)
	return CostVariables{CCPU: cCPU, CMem: cMem, CFin: cFin, FC: fc, CTN: cTN, BUsed: bUsed, PCC: pcc, CW: round(bUsed+pcc, 6)}
}

func assignmentBillingStart(assignment Assignment) float64 {
	return maxf(0, assignment.StartTime-assignment.ContainerOverhead-assignment.BootOverhead)
}

func machineActiveCost(assignments []Assignment, resource Resource) float64 {
	if len(assignments) == 0 || resource.PricePerHourUSD == 0 {
		return 0
	}
	start, finish := assignmentBillingStart(assignments[0]), assignments[0].FinishTime
	for _, assignment := range assignments[1:] {
		start = minf(start, assignmentBillingStart(assignment))
		finish = maxf(finish, assignment.FinishTime)
	}
	return maxf(0, finish-start) * resource.PricePerHourUSD / 3600
}

func incrementalMachineActiveCost(assignments []Assignment, candidate Assignment, resource Resource) float64 {
	if resource.PricePerHourUSD == 0 {
		return 0
	}
	before := []Assignment{}
	for _, assignment := range assignments {
		if assignment.ResourceID == resource.ID {
			before = append(before, assignment)
		}
	}
	after := append(append([]Assignment{}, before...), candidate)
	return maxf(0, machineActiveCost(after, resource)-machineActiveCost(before, resource))
}

func calculateInterference(generated GeneratedSimulation, assignments []Assignment) InterferenceVariables {
	phi, etStar, colocated := map[string]float64{}, map[string]float64{}, map[string][]string{}
	total := 0.0
	for _, assignment := range assignments {
		phi[assignment.TaskID] = assignment.PhiN
		etStar[assignment.TaskID] = assignment.EffectiveRuntime
		baseRuntime := generated.Matrices.ET0[assignment.TaskID][assignment.ResourceID]
		total += maxf(0, assignment.EffectiveRuntime-baseRuntime)
		colocated[assignment.TaskID] = []string{}
		for _, other := range assignments {
			if other.TaskID != assignment.TaskID && other.ResourceID == assignment.ResourceID && maxf(assignment.StartTime, other.StartTime) < minf(assignment.FinishTime, other.FinishTime) {
				colocated[assignment.TaskID] = append(colocated[assignment.TaskID], other.TaskID)
			}
		}
	}
	sum := 0.0
	for _, value := range phi {
		sum += value
	}
	return InterferenceVariables{PhiN: phi, ETStarByTask: etStar, ColocatedTasks: colocated, TotalInterferenceTime: round(total, 3), AveragePhiN: round(sum/float64(max(1, len(phi))), 4)}
}

func calculateDeviation(generated GeneratedSimulation, assignments []Assignment) DeviationVariables {
	etObs, variance, excess, dTime, dExcess, dN := map[string]float64{}, map[string]float64{}, map[string]float64{}, map[string]float64{}, map[string]float64{}, map[string]float64{}
	resourceValues := map[string][]float64{}
	for i, assignment := range assignments {
		drift := float64((generated.Seed+int64(i)*17)%13-6) / 100.0
		observed := round(assignment.EffectiveRuntime*(1+drift+assignment.PhiN*0.2), 3)
		baseRuntime := generated.Matrices.ET0[assignment.TaskID][assignment.ResourceID]
		v := round(observed-baseRuntime, 3)
		over := round(maxf(0, v), 3)
		etObs[assignment.TaskID] = observed
		variance[assignment.TaskID] = v
		excess[assignment.TaskID] = over
		dTime[assignment.TaskID] = round(v/maxf(baseRuntime, 0.001), 4)
		dExcess[assignment.TaskID] = round(over/maxf(baseRuntime, 0.001), 4)
		resourceValues[assignment.ResourceID] = append(resourceValues[assignment.ResourceID], dExcess[assignment.TaskID])
	}
	total := 0.0
	for resourceID, values := range resourceValues {
		sum := 0.0
		for _, value := range values {
			sum += value
		}
		dN[resourceID] = round(sum/float64(max(1, len(values))), 4)
	}
	for _, value := range dExcess {
		total += value
	}
	return DeviationVariables{ETObs: etObs, Var: variance, Excess: excess, DTime: dTime, DExcess: dExcess, DN: dN, DWTime: round(total/float64(max(1, len(dExcess))), 4)}
}
