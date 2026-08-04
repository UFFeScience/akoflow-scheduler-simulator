package main

import (
	"fmt"
	"log"
	"math"
	"math/bits"
	"runtime"
	"sort"
	"strings"
	"sync"
)

type beamState struct {
	Assignments         []Assignment
	AssignmentByTask    map[string]Assignment
	AssignmentTrace     *assignmentTrace
	AssignmentIndex     *assignmentIndexNode
	TaskOrdinals        map[string]int
	SelectedIntervals   map[string]*intervalIndexNode
	BillingStart        map[string]float64
	BillingFinish       map[string]float64
	Compact             bool
	SignatureHash       uint64
	CoreAvail           map[string]float64
	CoreIndexes         map[string]*coreAvailabilityNode
	NodeHasBooted       map[string]bool
	NodeReadyTime       map[string]float64
	NodeLastActive      map[string]float64
	CachedImages        map[string]bool
	StopIntervals       []MachineStopInterval
	StepTrace           *scheduleStepTrace
	PartialBudgetUsed   float64
	PartialMakespan     float64
	PartialScore        float64
	PartialInterference float64
	RemainingMinCost    float64
	ScheduledTaskHash   uint64
	FrontierMask        uint16
	FrontierScores      [beamFrontierCount]float64
	PendingPredChunks   [][]uint16
	ReadyTaskBits       []uint64
	TaskOrderSearch     bool
	OrderDeviated       bool
}

type scheduleStepTrace struct {
	Step ScheduleStep
	Prev *scheduleStepTrace
	Len  int
}

type optimizerContext struct {
	Tasks            map[string]Task
	Resources        map[string]Resource
	DepsByTarget     map[string][]Dependency
	DepsBySource     map[string][]Dependency
	PriorityOrder    []string
	PriorityRanks    map[string]float64
	MaxPriorityRank  float64
	DynamicReady     bool
	AdaptiveReady    bool
	TaskOrdinal      map[string]int
	TaskIDs          []string
	Successors       [][]int
	ResourceMasks    []uint64
	MinTaskCosts     []float64
	MinCriticalRanks []float64
	CanonicalHEFT    map[string]Assignment
	CanonicalState   *beamState
	SuccessorDelay   map[string]map[string][]float64
	LookaheadDepth   map[string]int
	PartitionSafe    map[string]bool
}

type beamFrontier struct {
	WeightTime float64
	WeightCost float64
}

const beamFrontierCount = 11
const readyTaskBranchLimit = 4
const pendingPredecessorChunkSize = 64
const compactBeamTaskThreshold = 100

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
	coreIndexes := map[string]*coreAvailabilityNode{}
	for _, resource := range generated.Resources {
		for _, core := range resource.Cores {
			coreAvail[core.ID] = 0
			coreIndexes[resource.ID] = coreIndexInsert(coreIndexes[resource.ID], core.ID, 0)
		}
	}
	hasBooted, ready, last := initialNodeState(generated.Resources)
	// Textual schedule signatures are useful for small, inspectable workflows,
	// but become quadratic when sorting Beam candidates. Above 100 tasks, use
	// persistent indexes and the incremental uint64 signature exclusively.
	compact := len(generated.Workflow.Tasks) > compactBeamTaskThreshold
	ctx := optimizerContext{
		Tasks: taskMap(generated.Workflow.Tasks), Resources: resourceMap(generated.Resources),
		DepsByTarget: dependenciesByTarget(generated.Workflow.Dependencies),
		DepsBySource: dependenciesBySource(generated.Workflow.Dependencies),
	}
	order, err := prismCCPriorityOrder(generated)
	if err != nil {
		return nil, err
	}
	ctx.PriorityOrder = order
	ctx.DynamicReady = generated.Experimental != nil &&
		(generated.Experimental.PriorityPolicy == "ready_lookahead" ||
			generated.Experimental.PriorityPolicy == "adaptive_ready")
	ctx.AdaptiveReady = generated.Experimental != nil &&
		generated.Experimental.PriorityPolicy == "adaptive_ready"
	var canonicalHEFTResult SimulationResult
	if ctx.AdaptiveReady {
		canonicalHEFT, canonicalErr := scheduleHEFTColocation(generated)
		if canonicalErr != nil {
			return nil, canonicalErr
		}
		ctx.CanonicalHEFT = map[string]Assignment{}
		for _, assignment := range canonicalHEFT.Assignments {
			ctx.CanonicalHEFT[assignment.TaskID] = assignment
		}
		canonicalHEFTResult = canonicalHEFT
	}
	if err := configureOptimizerContext(generated, &ctx); err != nil {
		return nil, err
	}
	ctx.LookaheadDepth = adaptiveTaskLookaheadDepths(generated, ctx)
	ctx.SuccessorDelay = buildAdaptiveSuccessorDelay(generated, ctx, ctx.LookaheadDepth)
	if ctx.AdaptiveReady {
		canonicalState := canonicalBeamState(generated, ctx, canonicalHEFTResult, compact)
		ctx.CanonicalState = &canonicalState
	}
	initialState := beamState{
		Assignments: []Assignment{}, AssignmentByTask: map[string]Assignment{},
		TaskOrdinals: ctx.TaskOrdinal, SelectedIntervals: map[string]*intervalIndexNode{},
		BillingStart: map[string]float64{}, BillingFinish: map[string]float64{},
		Compact: compact, CoreAvail: coreAvail, CoreIndexes: coreIndexes,
		NodeHasBooted: hasBooted, NodeReadyTime: ready, NodeLastActive: last,
		CachedImages:  initialCachedImages(generated.Resources),
		StopIntervals: []MachineStopInterval{},
	}
	if ctx.DynamicReady {
		initialState = initializeReadyState(ctx, initialState)
		initialState.TaskOrderSearch = true
	}
	for _, cost := range ctx.MinTaskCosts {
		initialState.RemainingMinCost += cost
	}
	if ctx.AdaptiveReady {
		return adaptiveBeamSearch(generated, ctx, initialState, beamWidth, len(order))
	}
	initialBeam := []beamState{initialState}
	frontiers := beamFrontiers()
	widths := beamFrontierWidths(beamWidth, len(frontiers))
	beams := make([][]beamState, len(frontiers))
	for index := range frontiers {
		beams[index] = append([]beamState{}, initialBeam...)
	}
	for stepIndex := range order {
		anyExpanded := false
		for index, frontier := range frontiers {
			var expanded []beamState
			if ctx.DynamicReady {
				expanded = expandReadyStatesParallel(generated, ctx, beams[index], stepIndex+1, frontier)
			} else {
				expanded = expandStatesParallel(generated, ctx, beams[index], stepIndex+1, order[stepIndex], frontier)
			}
			if len(expanded) == 0 {
				beams[index] = []beamState{}
				continue
			}
			anyExpanded = true
			beams[index] = selectBeamStates(expanded, widths[index], generated)
		}
		if !anyExpanded {
			return nil, fmt.Errorf("no feasible PRISM state at step %d", stepIndex+1)
		}
	}
	finalStates := []beamState{}
	for index := range frontiers {
		finalStates = append(finalStates, beams[index]...)
	}
	return dedupeStates(finalStates), nil
}

func configureOptimizerContext(generated GeneratedSimulation, ctx *optimizerContext) error {
	ctx.PartitionSafe = map[string]bool{}
	for _, resource := range generated.Resources {
		perCoreMemory := resource.Memory / float64(max(1, len(resource.Cores)))
		safe := true
		for _, task := range ctx.Tasks {
			if task.CPU > 1 || task.Memory > perCoreMemory {
				safe = false
				break
			}
		}
		ctx.PartitionSafe[resource.ID] = safe
	}
	ctx.TaskOrdinal = make(map[string]int, len(ctx.PriorityOrder))
	ctx.TaskIDs = append([]string(nil), ctx.PriorityOrder...)
	ctx.Successors = make([][]int, len(ctx.PriorityOrder))
	ctx.ResourceMasks = make([]uint64, len(ctx.PriorityOrder))
	ctx.MinTaskCosts = make([]float64, len(ctx.PriorityOrder))
	ctx.MinCriticalRanks = make([]float64, len(ctx.PriorityOrder))
	for ordinal, taskID := range ctx.PriorityOrder {
		ctx.TaskOrdinal[taskID] = ordinal
		task := ctx.Tasks[taskID]
		minRuntime := math.Inf(1)
		minCost := math.Inf(1)
		for resourceIndex, resource := range generated.Resources {
			if resourceIndex < 64 && resourceSupportsTask(resource, task) {
				ctx.ResourceMasks[ordinal] |= uint64(1) << resourceIndex
				runtime := generated.Matrices.ET0[taskID][resource.ID]
				minRuntime = minf(minRuntime, runtime)
				minCost = minf(minCost, runtime*resource.PricePerHourUSD/3600)
			}
		}
		if math.IsInf(minRuntime, 1) {
			minRuntime = 0
		}
		if math.IsInf(minCost, 1) {
			minCost = 0
		}
		ctx.MinCriticalRanks[ordinal] = minRuntime
		ctx.MinTaskCosts[ordinal] = minCost
	}
	for _, dependency := range generated.Workflow.Dependencies {
		source, sourceOK := ctx.TaskOrdinal[dependency.Source]
		target, targetOK := ctx.TaskOrdinal[dependency.Target]
		if sourceOK && targetOK {
			ctx.Successors[source] = append(ctx.Successors[source], target)
		}
	}
	for ordinal := len(ctx.PriorityOrder) - 1; ordinal >= 0; ordinal-- {
		maxSuccessor := 0.0
		for _, successor := range ctx.Successors[ordinal] {
			maxSuccessor = maxf(maxSuccessor, ctx.MinCriticalRanks[successor])
		}
		ctx.MinCriticalRanks[ordinal] += maxSuccessor
	}
	if !ctx.DynamicReady {
		return nil
	}
	ranks, err := prismCommunicationInterferenceRanks(generated)
	if err != nil {
		return err
	}
	ctx.PriorityRanks = ranks
	for _, rank := range ctx.PriorityRanks {
		ctx.MaxPriorityRank = maxf(ctx.MaxPriorityRank, rank)
	}
	return nil
}

func prismCCPriorityOrder(generated GeneratedSimulation) ([]string, error) {
	if generated.Experimental == nil || generated.Experimental.PriorityPolicy == "" ||
		generated.Experimental.PriorityPolicy == "topological_order" {
		return topologicalOrder(generated)
	}
	if generated.Experimental.PriorityPolicy != "upward_rank" &&
		generated.Experimental.PriorityPolicy != "ready_lookahead" &&
		generated.Experimental.PriorityPolicy != "adaptive_ready" {
		return nil, fmt.Errorf("unsupported PRISM-CC priority policy %q", generated.Experimental.PriorityPolicy)
	}
	ranks, err := prismCommunicationInterferenceRanks(generated)
	if err != nil {
		return nil, err
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
	return order, nil
}

// prismCommunicationInterferenceRanks extends HEFT's communication-aware
// upward rank with the expected P_cc exposure of each task profile. HEFT keeps
// its classic rank; only PRISM uses this continuum-specific priority.
func prismCommunicationInterferenceRanks(generated GeneratedSimulation) (map[string]float64, error) {
	if _, err := topologicalOrder(generated); err != nil {
		return nil, err
	}
	ctx := optimizerContext{
		Tasks:        taskMap(generated.Workflow.Tasks),
		DepsBySource: dependenciesBySource(generated.Workflow.Dependencies),
		DepsByTarget: dependenciesByTarget(generated.Workflow.Dependencies),
	}
	ranks := map[string]float64{}
	visiting := map[string]bool{}
	interferenceRate := 0.0
	if generated.Experimental != nil && !generated.Experimental.InterferenceDisabled {
		interferenceRate = generated.Experimental.InterferenceRate
	}
	profileRisk := map[string]float64{"cpu": 1.25, "memory": 1.15, "io": 1.1, "network": 1.1}
	var rank func(string) float64
	rank = func(taskID string) float64 {
		if value, exists := ranks[taskID]; exists {
			return value
		}
		if visiting[taskID] {
			return 0
		}
		visiting[taskID] = true
		task := ctx.Tasks[taskID]
		computation := 0.0
		for _, resource := range generated.Resources {
			computation += generated.Matrices.ET0[taskID][resource.ID]
		}
		computation /= float64(max(1, len(generated.Resources)))
		computation *= 1 + interferenceRate*profileRisk[taskInterferenceProfile(ctx, task)]
		maxSuccessor := 0.0
		for _, dependency := range ctx.DepsBySource[taskID] {
			communication, pairs := 0.0, 0
			for _, left := range generated.Resources {
				for _, right := range generated.Resources {
					if left.ID == right.ID {
						continue
					}
					communication += dependencyTransferSeconds(
						dependency, generated.Matrices.BandwidthBW[left.ID][right.ID],
						generated.Matrices.TransferDelay[left.ID][right.ID],
					)
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

func initializeReadyState(ctx optimizerContext, state beamState) beamState {
	chunkCount := (len(ctx.TaskIDs) + pendingPredecessorChunkSize - 1) / pendingPredecessorChunkSize
	state.PendingPredChunks = make([][]uint16, chunkCount)
	for index := range state.PendingPredChunks {
		state.PendingPredChunks[index] = make([]uint16, pendingPredecessorChunkSize)
	}
	state.ReadyTaskBits = make([]uint64, (len(ctx.TaskIDs)+63)/64)
	for ordinal, taskID := range ctx.TaskIDs {
		pending := len(ctx.DepsByTarget[taskID])
		if pending == 0 {
			state.ReadyTaskBits[ordinal/64] |= uint64(1) << (ordinal % 64)
		} else {
			state.PendingPredChunks[ordinal/pendingPredecessorChunkSize][ordinal%pendingPredecessorChunkSize] = uint16(pending)
		}
	}
	return state
}

func readyTaskOrdinals(ctx optimizerContext, state beamState) []int {
	keys := make([]int, 0, readyTaskBranchLimit*2)
	for wordIndex, word := range state.ReadyTaskBits {
		for word != 0 && len(keys) < readyTaskBranchLimit*2 {
			bit := bits.TrailingZeros64(word)
			ordinal := wordIndex*64 + bit
			if ordinal < len(ctx.TaskIDs) {
				keys = append(keys, ordinal)
			}
			word &^= uint64(1) << bit
		}
		if len(keys) == readyTaskBranchLimit*2 {
			break
		}
	}
	selected := make([]int, 0, readyTaskBranchLimit)
	for _, ordinal := range keys {
		canonicalAfterEarlier := false
		for _, earlier := range selected {
			if ctx.ResourceMasks[ordinal]&ctx.ResourceMasks[earlier] == 0 {
				canonicalAfterEarlier = true
				break
			}
		}
		if canonicalAfterEarlier {
			continue
		}
		selected = append(selected, ordinal)
		if len(selected) == readyTaskBranchLimit {
			break
		}
	}
	return selected
}

func readyTaskCandidates(ctx optimizerContext, state beamState) []string {
	ordinals := readyTaskOrdinals(ctx, state)
	ready := make([]string, 0, len(ordinals))
	for _, ordinal := range ordinals {
		ready = append(ready, ctx.TaskIDs[ordinal])
	}
	return ready
}

func advanceReadyTaskFrontier(ctx optimizerContext, state beamState, scheduledTaskID string) beamState {
	ordinal := ctx.TaskOrdinal[scheduledTaskID]
	readyBits := append([]uint64(nil), state.ReadyTaskBits...)
	readyBits[ordinal/64] &^= uint64(1) << (ordinal % 64)
	pendingChunks := append([][]uint16(nil), state.PendingPredChunks...)
	clonedChunks := make([]int, 0, len(ctx.Successors[ordinal]))
	for _, successor := range ctx.Successors[ordinal] {
		chunkIndex := successor / pendingPredecessorChunkSize
		offset := successor % pendingPredecessorChunkSize
		pending := pendingChunks[chunkIndex][offset]
		if pending == 0 {
			continue
		}
		cloned := false
		for _, index := range clonedChunks {
			cloned = cloned || index == chunkIndex
		}
		if !cloned {
			pendingChunks[chunkIndex] = append([]uint16(nil), pendingChunks[chunkIndex]...)
			clonedChunks = append(clonedChunks, chunkIndex)
		}
		pending--
		pendingChunks[chunkIndex][offset] = pending
		if pending == 0 {
			readyBits[successor/64] |= uint64(1) << (successor % 64)
		}
	}
	state.PendingPredChunks = pendingChunks
	state.ReadyTaskBits = readyBits
	return state
}

func expandReadyStatesParallel(generated GeneratedSimulation, ctx optimizerContext, states []beamState, stepIndex int, frontier beamFrontier) []beamState {
	if len(states) == 0 {
		return []beamState{}
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
				for _, taskID := range readyTaskCandidates(ctx, states[index]) {
					results[index] = append(
						results[index],
						expandState(generated, ctx, states[index], stepIndex, taskID, frontier)...,
					)
				}
			}
		}()
	}
	for index := range states {
		jobs <- index
	}
	close(jobs)
	wg.Wait()
	out := []beamState{}
	for _, stateResults := range results {
		out = append(out, stateResults...)
	}
	return out
}

type adaptiveStateEvaluation struct {
	State               beamState
	TimeLowerBound      float64
	CostLowerBound      float64
	PotentiallyFeasible bool
	BestScore           float64
}

func prismIncrementalFinancialCost(computeCost, networkCost float64) float64 {
	return computeCost + networkCost
}

func adaptiveBeamSearch(generated GeneratedSimulation, ctx optimizerContext, initial beamState, beamWidth, taskCount int) ([]beamState, error) {
	frontiers := beamFrontiers()
	perFrontierWidth := max(1, min(10, beamWidth/max(1, len(frontiers))))
	beams := make([][]beamState, len(frontiers))
	for index := range beams {
		beams[index] = []beamState{initial}
	}
	paretoArchive := []beamState{initial}
	for stepIndex := 1; stepIndex <= taskCount; stepIndex++ {
		candidateCount := 0
		nextBeams := make([][]beamState, len(frontiers))
		for frontierIndex, frontier := range frontiers {
			candidates := expandAdaptiveStatesParallel(generated, ctx, beams[frontierIndex], stepIndex, frontier)
			candidateCount += len(candidates)
			if len(candidates) == 0 {
				continue
			}
			nextBeams[frontierIndex] = selectIndependentFrontierStates(
				candidates, perFrontierWidth, frontierIndex, generated,
			)
		}
		pool := []beamState{}
		for _, beam := range nextBeams {
			pool = append(pool, beam...)
		}
		// The shared archive is a twelfth, objective-neutral search stream.
		// Every frontier contributes to it, but archived states are expanded
		// only once instead of being duplicated into all eleven beams.
		archiveCandidates := expandAdaptiveStatesParallel(
			generated, ctx, paretoArchive, stepIndex, beamFrontier{WeightTime: 0.5, WeightCost: 0.5},
		)
		candidateCount += len(archiveCandidates)
		pool = append(pool, archiveCandidates...)
		if len(pool) == 0 {
			return nil, fmt.Errorf("adaptive Beam produced no state at step %d", stepIndex)
		}
		paretoArchive = partialParetoArchive(pool, generated)
		beams = nextBeams
		if stepIndex == 1 || stepIndex%500 == 0 || stepIndex == taskCount {
			retained := 0
			for _, beam := range beams {
				retained += len(beam)
			}
			log.Printf(
				"independent Beam step %d/%d: candidates=%d retained=%d pareto=%d frontiers=%d width=%d",
				stepIndex, taskCount, candidateCount, retained, len(paretoArchive),
				len(frontiers), perFrontierWidth,
			)
		}
	}
	finalStates := append([]beamState{}, paretoArchive...)
	for _, beam := range beams {
		finalStates = append(finalStates, beam...)
	}
	if ctx.CanonicalState != nil {
		finalStates = append(finalStates, *ctx.CanonicalState)
	}
	finalStates = dedupeStates(finalStates)
	sort.SliceStable(finalStates, func(i, j int) bool {
		if finalStates[i].OrderDeviated != finalStates[j].OrderDeviated {
			return !finalStates[i].OrderDeviated
		}
		return beamStateLess(finalStates[i], finalStates[j])
	})
	if len(finalStates) > beamWidth {
		finalStates = finalStates[:beamWidth]
	}
	return finalStates, nil
}

func canonicalBeamState(generated GeneratedSimulation, ctx optimizerContext, result SimulationResult, compact bool) beamState {
	state := beamState{
		Assignments:      append([]Assignment(nil), result.Assignments...),
		AssignmentByTask: map[string]Assignment{}, TaskOrdinals: ctx.TaskOrdinal,
		Compact: compact, PartialMakespan: result.TimingVariables.Makespan,
		PartialBudgetUsed: result.CostVariables.BUsed, TaskOrderSearch: true,
		CachedImages: initialCachedImages(generated.Resources),
	}
	for _, assignment := range result.Assignments {
		state.AssignmentByTask[assignment.TaskID] = assignment
		state.ScheduledTaskHash ^= intPriority(ctx.TaskOrdinal[assignment.TaskID])
		state.SignatureHash ^= stablePriority(fmt.Sprintf(
			"%s:%s:%s:%.6f:%.6f", assignment.TaskID, assignment.ResourceID,
			assignment.CoreID, assignment.StartTime, assignment.FinishTime,
		))
		if compact {
			state.AssignmentTrace = appendAssignmentTrace(state.AssignmentTrace, assignment)
			state.AssignmentIndex = assignmentIndexInsert(state.AssignmentIndex, ctx.TaskOrdinal[assignment.TaskID], assignment)
		}
	}
	if compact {
		state.Assignments = nil
		state.AssignmentByTask = nil
	}
	return state
}

func selectIndependentFrontierStates(states []beamState, width, frontierIndex int, generated GeneratedSimulation) []beamState {
	allUnique := dedupeStates(states)
	var canonical *beamState
	for _, state := range allUnique {
		if !state.OrderDeviated && (canonical == nil || beamStateLess(state, *canonical)) {
			copy := state
			canonical = &copy
		}
	}
	unique := preferPartiallyFeasibleStates(allUnique, generated)
	sort.SliceStable(unique, func(i, j int) bool {
		left, right := unique[i], unique[j]
		if left.FrontierScores[frontierIndex] != right.FrontierScores[frontierIndex] {
			return left.FrontierScores[frontierIndex] < right.FrontierScores[frontierIndex]
		}
		if left.OrderDeviated != right.OrderDeviated {
			return !left.OrderDeviated
		}
		return beamStateLess(left, right)
	})
	if len(unique) > width {
		unique = unique[:width]
	}
	if canonical != nil {
		found := false
		for _, state := range unique {
			found = found || stateSignature(state) == stateSignature(*canonical)
		}
		if !found && len(unique) > 0 {
			unique[len(unique)-1] = *canonical
		}
	}
	return unique
}

func partialParetoArchive(states []beamState, generated GeneratedSimulation) []beamState {
	unique := preferPartiallyFeasibleStates(dedupeStates(states), generated)
	archive := make([]beamState, 0, len(unique))
	for index, candidate := range unique {
		dominated := false
		for otherIndex, other := range unique {
			if index == otherIndex || candidate.ScheduledTaskHash != other.ScheduledTaskHash {
				continue
			}
			noWorse := other.PartialMakespan <= candidate.PartialMakespan &&
				other.PartialBudgetUsed <= candidate.PartialBudgetUsed
			strict := other.PartialMakespan < candidate.PartialMakespan ||
				other.PartialBudgetUsed < candidate.PartialBudgetUsed
			if noWorse && strict {
				dominated = true
				break
			}
		}
		if !dominated {
			archive = append(archive, candidate)
		}
	}
	maxArchive := beamFrontierCount * 2
	if len(archive) > maxArchive {
		sort.SliceStable(archive, func(i, j int) bool {
			if archive[i].PartialMakespan != archive[j].PartialMakespan {
				return archive[i].PartialMakespan < archive[j].PartialMakespan
			}
			return archive[i].PartialBudgetUsed < archive[j].PartialBudgetUsed
		})
		diverse := make([]beamState, 0, maxArchive)
		for index := 0; index < maxArchive; index++ {
			position := index * (len(archive) - 1) / (maxArchive - 1)
			diverse = append(diverse, archive[position])
		}
		archive = dedupeStates(diverse)
	}
	return archive
}

func expandAdaptiveStatesParallel(generated GeneratedSimulation, ctx optimizerContext, states []beamState, stepIndex int, frontier beamFrontier) []beamState {
	if len(states) == 0 {
		return nil
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
				for _, taskID := range readyTaskCandidates(ctx, states[index]) {
					results[index] = append(
						results[index],
						expandState(generated, ctx, states[index], stepIndex, taskID, frontier)...,
					)
				}
			}
		}()
	}
	for index := range states {
		jobs <- index
	}
	close(jobs)
	wg.Wait()
	total := 0
	for _, children := range results {
		total += len(children)
	}
	out := make([]beamState, 0, total)
	for _, children := range results {
		out = append(out, children...)
	}
	return out
}

func adaptiveBounds(state beamState, ctx optimizerContext) (float64, float64) {
	timeLowerBound := state.PartialMakespan
	for wordIndex, word := range state.ReadyTaskBits {
		for word != 0 {
			bit := bits.TrailingZeros64(word)
			ordinal := wordIndex*64 + bit
			if ordinal < len(ctx.MinCriticalRanks) {
				timeLowerBound = maxf(timeLowerBound, ctx.MinCriticalRanks[ordinal])
			}
			word &^= uint64(1) << bit
		}
	}
	return timeLowerBound, state.PartialBudgetUsed + state.RemainingMinCost
}

func adaptivePotentiallyFeasible(state beamState, timeLowerBound, costLowerBound float64, generated GeneratedSimulation) bool {
	if generated.SLA.DeadlineLimit != nil && timeLowerBound > *generated.SLA.DeadlineLimit {
		return false
	}
	if generated.SLA.BudgetLimit != nil && costLowerBound > *generated.SLA.BudgetLimit {
		return false
	}
	return true
}

func adaptiveDominates(left, right adaptiveStateEvaluation) bool {
	if left.State.ScheduledTaskHash != right.State.ScheduledTaskHash {
		return false
	}
	noWorse := left.TimeLowerBound <= right.TimeLowerBound &&
		left.CostLowerBound <= right.CostLowerBound &&
		left.State.PartialInterference <= right.State.PartialInterference
	strict := left.TimeLowerBound < right.TimeLowerBound ||
		left.CostLowerBound < right.CostLowerBound ||
		left.State.PartialInterference < right.State.PartialInterference
	return noWorse && strict
}

func removeAdaptiveDominated(items []adaptiveStateEvaluation) []adaptiveStateEvaluation {
	groups := map[uint64][]adaptiveStateEvaluation{}
	for _, item := range items {
		groups[item.State.ScheduledTaskHash] = append(groups[item.State.ScheduledTaskHash], item)
	}
	out := make([]adaptiveStateEvaluation, 0, len(items))
	for _, group := range groups {
		for index, candidate := range group {
			dominated := false
			for otherIndex, other := range group {
				if index != otherIndex && adaptiveDominates(other, candidate) {
					dominated = true
					break
				}
			}
			if !dominated {
				out = append(out, candidate)
			}
		}
	}
	return out
}

func markAdaptiveFrontierRelevance(items []adaptiveStateEvaluation) {
	if len(items) == 0 {
		return
	}
	maxTime, maxCost := 0.0, 0.0
	for _, item := range items {
		maxTime = maxf(maxTime, item.TimeLowerBound)
		maxCost = maxf(maxCost, item.CostLowerBound)
	}
	for frontierIndex, frontier := range beamFrontiers() {
		bestIndex, bestScore := -1, math.Inf(1)
		for index := range items {
			score := frontier.WeightTime*items[index].TimeLowerBound/maxf(maxTime, 0.001) +
				frontier.WeightCost*items[index].CostLowerBound/maxf(maxCost, 0.001)
			items[index].BestScore = minf(items[index].BestScore, score)
			if score < bestScore {
				bestIndex, bestScore = index, score
			}
		}
		if bestIndex >= 0 {
			items[bestIndex].State.FrontierMask |= uint16(1) << frontierIndex
		}
	}
}

// markPersistentFrontierIncumbents reserves one continuing path for every
// time/cost objective. FrontierScores are cumulative, so each lane follows
// its own best history instead of competing only on the current step's
// lower bounds. A single state may represent more than one lane when their
// best paths coincide.
func markPersistentFrontierIncumbents(items []adaptiveStateEvaluation) {
	for frontierIndex := range beamFrontiers() {
		bestIndex := -1
		for index := range items {
			if bestIndex < 0 ||
				items[index].State.FrontierScores[frontierIndex] < items[bestIndex].State.FrontierScores[frontierIndex] ||
				(items[index].State.FrontierScores[frontierIndex] == items[bestIndex].State.FrontierScores[frontierIndex] &&
					beamStateLess(items[index].State, items[bestIndex].State)) {
				bestIndex = index
			}
		}
		if bestIndex >= 0 {
			items[bestIndex].State.FrontierMask |= uint16(1) << frontierIndex
		}
	}
}

func markCanonicalFrontierRelevance(items []adaptiveStateEvaluation, width int) {
	canonicalCapacity := max(1, 3*width/4)
	perFrontier := max(1, (canonicalCapacity+len(beamFrontiers())-1)/len(beamFrontiers()))
	for frontierIndex := range beamFrontiers() {
		indexes := make([]int, len(items))
		for index := range indexes {
			indexes[index] = index
		}
		sort.SliceStable(indexes, func(i, j int) bool {
			left := items[indexes[i]].State.FrontierScores[frontierIndex]
			right := items[indexes[j]].State.FrontierScores[frontierIndex]
			if left != right {
				return left < right
			}
			return beamStateLess(items[indexes[i]].State, items[indexes[j]].State)
		})
		for rank, index := range indexes {
			if rank == perFrontier {
				break
			}
			items[index].State.FrontierMask |= uint16(1) << frontierIndex
		}
	}
}

func adaptiveParetoIndexes(items []adaptiveStateEvaluation) map[int]bool {
	indexes := make([]int, len(items))
	for index := range indexes {
		indexes[index] = index
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		left, right := items[indexes[i]], items[indexes[j]]
		if left.TimeLowerBound != right.TimeLowerBound {
			return left.TimeLowerBound < right.TimeLowerBound
		}
		return left.CostLowerBound < right.CostLowerBound
	})
	pareto := map[int]bool{}
	bestCost := math.Inf(1)
	for _, index := range indexes {
		if items[index].CostLowerBound < bestCost {
			pareto[index] = true
			bestCost = items[index].CostLowerBound
		}
	}
	return pareto
}

func adaptiveViolationScore(item adaptiveStateEvaluation, generated GeneratedSimulation) float64 {
	score := 0.0
	if generated.SLA.DeadlineLimit != nil {
		score += maxf(0, item.TimeLowerBound-*generated.SLA.DeadlineLimit) /
			maxf(*generated.SLA.DeadlineLimit, 0.001)
	}
	if generated.SLA.BudgetLimit != nil {
		score += maxf(0, item.CostLowerBound-*generated.SLA.BudgetLimit) /
			maxf(*generated.SLA.BudgetLimit, 0.001)
	}
	return score
}

func adaptiveAlternativeAddsValue(candidate, canonical adaptiveStateEvaluation, generated GeneratedSimulation) bool {
	canonicalDominates := canonical.TimeLowerBound <= candidate.TimeLowerBound &&
		canonical.CostLowerBound <= candidate.CostLowerBound &&
		canonical.State.PartialInterference <= candidate.State.PartialInterference
	if canonicalDominates {
		return false
	}
	improvesObjective := candidate.TimeLowerBound < canonical.TimeLowerBound ||
		candidate.CostLowerBound < canonical.CostLowerBound ||
		candidate.State.PartialInterference < canonical.State.PartialInterference
	if !improvesObjective {
		return false
	}
	if candidate.PotentiallyFeasible {
		return true
	}
	if canonical.PotentiallyFeasible {
		return false
	}
	return adaptiveViolationScore(candidate, generated) < adaptiveViolationScore(canonical, generated)
}

func selectAdaptiveBeamStates(states []beamState, width int, generated GeneratedSimulation, ctx optimizerContext) []beamState {
	unique := dedupeStates(states)
	evaluated := make([]adaptiveStateEvaluation, 0, len(unique))
	anyPotentiallyFeasible := false
	for _, state := range unique {
		timeBound, costBound := adaptiveBounds(state, ctx)
		potential := adaptivePotentiallyFeasible(state, timeBound, costBound, generated)
		anyPotentiallyFeasible = anyPotentiallyFeasible || potential
		evaluated = append(evaluated, adaptiveStateEvaluation{
			State: state, TimeLowerBound: timeBound, CostLowerBound: costBound,
			PotentiallyFeasible: potential, BestScore: math.Inf(1),
		})
	}
	var canonical *adaptiveStateEvaluation
	for _, item := range evaluated {
		if !item.State.OrderDeviated {
			candidate := item
			if canonical == nil || candidate.TimeLowerBound < canonical.TimeLowerBound ||
				(candidate.TimeLowerBound == canonical.TimeLowerBound &&
					candidate.CostLowerBound < canonical.CostLowerBound) {
				canonical = &candidate
			}
		}
	}
	if canonical != nil {
		filtered := make([]adaptiveStateEvaluation, 0, len(evaluated))
		for _, item := range evaluated {
			if !item.State.OrderDeviated ||
				adaptiveAlternativeAddsValue(item, *canonical, generated) {
				filtered = append(filtered, item)
			}
		}
		evaluated = filtered
	}
	evaluated = removeAdaptiveDominated(evaluated)
	present := map[string]bool{}
	for _, item := range evaluated {
		present[stateSignature(item.State)] = true
	}
	if canonical != nil {
		signature := stateSignature(canonical.State)
		if !present[signature] {
			evaluated = append(evaluated, *canonical)
			present[signature] = true
		}
	}
	competitive := evaluated
	if anyPotentiallyFeasible {
		competitive = make([]adaptiveStateEvaluation, 0, len(evaluated))
		for _, item := range evaluated {
			if item.PotentiallyFeasible || !item.State.OrderDeviated {
				competitive = append(competitive, item)
			}
		}
	}
	// Keep the incumbent of every cumulative objective lane. In addition,
	// retain the best lower-bound challengers: they may accept a small loss
	// now and still improve the completed schedule later.
	markPersistentFrontierIncumbents(competitive)
	markAdaptiveFrontierRelevance(competitive)
	valuable := make([]adaptiveStateEvaluation, 0, len(competitive))
	for _, item := range competitive {
		if item.State.FrontierMask != 0 || !item.State.OrderDeviated {
			valuable = append(valuable, item)
		}
	}
	sort.SliceStable(valuable, func(i, j int) bool {
		left, right := valuable[i], valuable[j]
		if left.PotentiallyFeasible != right.PotentiallyFeasible {
			return left.PotentiallyFeasible
		}
		leftCanonical, rightCanonical := !left.State.OrderDeviated, !right.State.OrderDeviated
		if leftCanonical != rightCanonical {
			return leftCanonical
		}
		leftRelevance := bits.OnesCount16(left.State.FrontierMask)
		rightRelevance := bits.OnesCount16(right.State.FrontierMask)
		if leftRelevance != rightRelevance {
			return leftRelevance > rightRelevance
		}
		if left.BestScore != right.BestScore {
			return left.BestScore < right.BestScore
		}
		return beamStateLess(left.State, right.State)
	})
	if len(valuable) > width {
		valuable = valuable[:width]
	}
	out := make([]beamState, 0, len(valuable))
	for _, item := range valuable {
		out = append(out, item.State)
	}
	return out
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
		lookaheadFinish float64
		phi, incBudget  float64
	}
	rows := []candidateRow{}
	for _, resource := range generated.Resources {
		if !resourceSupportsTask(resource, task) {
			continue
		}
		predecessorFloor, transferTotal := predecessorTimingForState(ctx.DepsByTarget[task.ID], state, generated, resource.ID)
		networkCost := 0.0
		for _, dep := range ctx.DepsByTarget[task.ID] {
			predecessor, _ := stateAssignment(state, dep.Source)
			networkCost += dep.DataMB * generated.Matrices.FinancialNetworkCost[predecessor.ResourceID][resource.ID]
		}
		for _, core := range []Core{earliestAvailableCoreForState(resource, state)} {
			readyFloor := maxf(predecessorFloor, coreAvailabilityForState(state, resource.ID, core.ID), state.NodeReadyTime[resource.ID])
			stopBoot := resource.Kind == "cloud" && state.NodeHasBooted[resource.ID] && resource.BootOverhead > 0 && readyFloor-state.NodeLastActive[resource.ID] >= resource.BootOverhead
			coldBoot := !state.NodeHasBooted[resource.ID]
			boot := 0.0
			if coldBoot || stopBoot {
				boot = resource.BootOverhead
			}
			container := dynamicContainerOverhead(generated, state, task, resource.ID)
			bootReady := readyFloor
			if coldBoot {
				bootReady += boot
			}
			start := round(bootReady+container, 3)
			baseRuntime := generated.Matrices.ET0[task.ID][resource.ID]
			start = round(capacityConstrainedStart(ctx, state, task, resource, start, baseRuntime), 3)
			phi, pairwise := candidatePairwiseInterferenceForState(generated, task.ID, resource.ID, state, start, start+baseRuntime)
			phi, interferenceProfile := explicitProfilePCC(ctx, task, phi, pairwise)
			effective := round(baseRuntime*(1+phi), 3)
			finish := round(start+effective, 3)
			score := ScoreBreakdown{}
			assignment := Assignment{TaskID: task.ID, ResourceID: resource.ID, CoreID: core.ID, StartTime: start, FinishTime: finish, EffectiveRuntime: effective, TransferDelay: round(transferTotal, 3), BootOverhead: boot, ContainerOverhead: container, PhiN: phi, PredecessorFinishFloor: round(predecessorFloor, 3), Score: score, InterferenceProfile: interferenceProfile}
			rawCost := incrementalMachineActiveCostForState(state, assignment, resource)
			candidate := CandidateEvaluation{TaskID: task.ID, ResourceID: resource.ID, CoreID: core.ID, StartTime: start, FinishTime: finish, BaseRuntime: baseRuntime, EffectiveRuntime: effective, InterferenceTime: round(effective-baseRuntime, 3), TransferDelay: round(transferTotal, 3), BootOverhead: boot, ContainerOverhead: container, PredecessorFinishFloor: round(predecessorFloor, 3), RawCost: round(rawCost, 4), PhiN: phi, PairwiseInterference: pairwise, Score: score, InterferenceProfile: interferenceProfile}
			rows = append(rows, candidateRow{
				assignment: assignment, candidate: candidate, finish: finish, rawCost: rawCost,
				lookaheadFinish: finish, phi: phi,
				incBudget: prismIncrementalFinancialCost(rawCost, networkCost),
			})
		}
	}
	if len(rows) == 0 {
		return []beamState{}
	}
	localFinishes := make([]float64, len(rows))
	incrementalCosts := make([]float64, len(rows))
	for index := range rows {
		localFinishes[index] = rows[index].finish
		incrementalCosts[index] = rows[index].incBudget
	}
	lookaheadDepth := adaptiveDecisionLookaheadDepth(ctx, task, localFinishes, incrementalCosts)
	for index := range rows {
		rows[index].lookaheadFinish = adaptiveSuccessorLookaheadFinish(
			ctx, task, rows[index].assignment, lookaheadDepth,
		)
	}
	maxFinish, maxLookaheadFinish, maxIncrementalCost := 0.0, 0.0, 0.0
	for _, row := range rows {
		maxFinish = maxf(maxFinish, row.finish)
		maxLookaheadFinish = maxf(maxLookaheadFinish, row.lookaheadFinish)
		maxIncrementalCost = maxf(maxIncrementalCost, row.incBudget)
	}
	for i := range rows {
		localTimeScore := rows[i].finish / maxf(maxFinish, 0.001)
		lookaheadTimeScore := rows[i].lookaheadFinish / maxf(maxLookaheadFinish, 0.001)
		timeScore := 0.65*localTimeScore + 0.35*lookaheadTimeScore
		usesAlternativeTaskOrder := ctx.DynamicReady &&
			(state.OrderDeviated || ctx.TaskOrdinal[task.ID] != stepIndex-1)
		if usesAlternativeTaskOrder {
			criticalUrgencyPenalty := 1 - ctx.PriorityRanks[task.ID]/maxf(ctx.MaxPriorityRank, 0.001)
			interferenceRisk := 0.0
			if generated.Experimental != nil &&
				generated.Experimental.interferenceActivitySet[task.ID] {
				interferenceRisk = rows[i].phi * generated.Experimental.InterferenceRate
			}
			lookaheadScore := criticalUrgencyPenalty + interferenceRisk
			timeScore = 0.75*timeScore + 0.25*lookaheadScore
		}
		costScore := 0.0
		if maxIncrementalCost != 0 {
			// Financial network transfer is part of C_fin, not merely a
			// posterior budget accounting term.
			costScore = rows[i].incBudget / maxIncrementalCost
		}
		score := ScoreBreakdown{
			TimeScore: round(timeScore, 5), CostScore: round(costScore, 5),
			InterferenceScore: round(rows[i].phi, 5),
			TotalScore: round(
				0.5*timeScore+0.5*costScore+rows[i].phi,
				5,
			),
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
	displayRankBySlot := map[string]int{}
	if !state.Compact {
		for _, row := range rows {
			rankedCandidates = append(rankedCandidates, row.candidate)
		}
		sortCandidates(rankedCandidates)
		for i, candidate := range rankedCandidates {
			displayRankBySlot[candidate.ResourceID+"|"+candidate.CoreID] = i + 1
		}
	}
	nextStates := []beamState{}
	canonicalTaskID := ""
	if ctx.AdaptiveReady {
		readyTasks := readyTaskCandidates(ctx, state)
		if len(readyTasks) > 0 {
			canonicalTaskID = readyTasks[0]
		}
	}
	canonicalMachineSlot := ""
	if ctx.AdaptiveReady && task.ID == canonicalTaskID {
		if canonical, ok := ctx.CanonicalHEFT[task.ID]; ok {
			canonicalMachineSlot = canonical.ResourceID + "|" + canonical.CoreID
		}
	}
	nextReadyState := state
	if ctx.DynamicReady {
		nextReadyState = advanceReadyTaskFrontier(ctx, state, task.ID)
	}
	for _, row := range rows {
		selectedCandidates := []CandidateEvaluation{}
		if !state.Compact {
			for _, ranked := range rankedCandidates {
				item := ranked
				item.Rank = displayRankBySlot[item.ResourceID+"|"+item.CoreID]
				item.Selected = item.ResourceID == row.assignment.ResourceID && item.CoreID == row.assignment.CoreID
				selectedCandidates = append(selectedCandidates, item)
			}
		}
		selectedRank := frontierRankBySlot[row.assignment.ResourceID+"|"+row.assignment.CoreID]
		step := ScheduleStep{Step: stepIndex, TaskID: task.ID, SelectedResourceID: row.assignment.ResourceID, SelectedCoreID: row.assignment.CoreID, SelectedTotalScore: row.assignment.Score.TotalScore, Candidates: selectedCandidates}
		coreAvail := state.CoreAvail
		coreIndexes := state.CoreIndexes
		if state.Compact {
			coreIndexes = copyCoreRootMap(state.CoreIndexes)
			coreIndexes[row.assignment.ResourceID] = coreIndexInsert(coreIndexes[row.assignment.ResourceID], row.assignment.CoreID, row.assignment.FinishTime)
		} else {
			coreAvail = copyFloatMap(state.CoreAvail)
			coreAvail[row.assignment.CoreID] = row.assignment.FinishTime
		}
		nodeHasBooted := copyBoolMap(state.NodeHasBooted)
		nodeReady := copyFloatMap(state.NodeReadyTime)
		nodeLast := copyFloatMap(state.NodeLastActive)
		cachedImages := state.CachedImages
		if task.Image != "" && !state.CachedImages[resourceImageCacheKey(row.assignment.ResourceID, task.Image)] {
			cachedImages = copyBoolMap(state.CachedImages)
			cachedImages[resourceImageCacheKey(row.assignment.ResourceID, task.Image)] = true
		}
		intervals := append([]MachineStopInterval{}, state.StopIntervals...)
		updateNodeState(row.assignment, ctx.Resources[row.assignment.ResourceID], nodeHasBooted, nodeReady, nodeLast, &intervals)
		partialBudget := round(state.PartialBudgetUsed+row.incBudget, 4)
		partialMakespan := round(maxf(state.PartialMakespan, row.assignment.FinishTime), 3)
		partialScore := round(state.PartialScore+beamFrontierScore(row.assignment.Score, frontier)+float64(selectedRank)*0.0001, 6)
		frontierScores := state.FrontierScores
		for frontierIndex, objective := range beamFrontiers() {
			frontierScores[frontierIndex] = round(
				frontierScores[frontierIndex]+beamFrontierScore(row.assignment.Score, objective),
				6,
			)
		}
		partialInterference := round(
			state.PartialInterference+maxf(0, row.assignment.EffectiveRuntime-generated.Matrices.ET0[task.ID][row.assignment.ResourceID]),
			3,
		)
		taskOrdinal, hasTaskOrdinal := ctx.TaskOrdinal[task.ID]
		minTaskCost := 0.0
		if hasTaskOrdinal && taskOrdinal < len(ctx.MinTaskCosts) {
			minTaskCost = ctx.MinTaskCosts[taskOrdinal]
		}
		remainingMinCost := maxf(0, state.RemainingMinCost-minTaskCost)
		scheduledTaskHash := state.ScheduledTaskHash ^ stablePriority(task.ID)
		if hasTaskOrdinal {
			scheduledTaskHash = state.ScheduledTaskHash ^ intPriority(taskOrdinal)
		}
		assignments, byTask := state.Assignments, state.AssignmentByTask
		trace, index := state.AssignmentTrace, state.AssignmentIndex
		selectedIntervals := state.SelectedIntervals
		billingStart, billingFinish := state.BillingStart, state.BillingFinish
		if state.Compact {
			trace = appendAssignmentTrace(trace, row.assignment)
			index = assignmentIndexInsert(index, ctx.TaskOrdinal[row.assignment.TaskID], row.assignment)
			selectedIntervals = copyIntervalRootMap(selectedIntervals)
			if generated.Experimental != nil && generated.Experimental.interferenceActivitySet[row.assignment.TaskID] {
				selectedIntervals[row.assignment.ResourceID] = intervalIndexInsert(selectedIntervals[row.assignment.ResourceID], row.assignment)
			}
			billingStart, billingFinish = updatedBillingBounds(state, row.assignment)
		} else {
			assignments = append(append([]Assignment{}, state.Assignments...), row.assignment)
			byTask = map[string]Assignment{}
			for key, value := range state.AssignmentByTask {
				byTask[key] = value
			}
			byTask[row.assignment.TaskID] = row.assignment
		}
		signatureHash := state.SignatureHash ^ stablePriority(fmt.Sprintf(
			"%s:%s:%s:%.6f:%.6f",
			row.assignment.TaskID, row.assignment.ResourceID, row.assignment.CoreID,
			row.assignment.StartTime, row.assignment.FinishTime,
		))
		stepTrace := state.StepTrace
		if !state.Compact {
			stepTrace = appendStepTrace(state.StepTrace, step)
		}
		orderDeviated := state.OrderDeviated
		if ctx.AdaptiveReady {
			slot := row.assignment.ResourceID + "|" + row.assignment.CoreID
			if task.ID != canonicalTaskID || slot != canonicalMachineSlot {
				orderDeviated = true
			}
		} else if ctx.DynamicReady && ctx.TaskOrdinal[task.ID] != stepIndex-1 {
			orderDeviated = true
		}
		nextStates = append(nextStates, beamState{Assignments: assignments, AssignmentByTask: byTask, AssignmentTrace: trace, AssignmentIndex: index, TaskOrdinals: state.TaskOrdinals, SelectedIntervals: selectedIntervals, BillingStart: billingStart, BillingFinish: billingFinish, Compact: state.Compact, SignatureHash: signatureHash, CoreAvail: coreAvail, CoreIndexes: coreIndexes, NodeHasBooted: nodeHasBooted, NodeReadyTime: nodeReady, NodeLastActive: nodeLast, CachedImages: cachedImages, StopIntervals: intervals, StepTrace: stepTrace, PartialBudgetUsed: partialBudget, PartialMakespan: partialMakespan, PartialScore: partialScore, PartialInterference: partialInterference, RemainingMinCost: remainingMinCost, ScheduledTaskHash: scheduledTaskHash, FrontierScores: frontierScores, PendingPredChunks: nextReadyState.PendingPredChunks, ReadyTaskBits: nextReadyState.ReadyTaskBits, TaskOrderSearch: state.TaskOrderSearch, OrderDeviated: orderDeviated})
	}
	return nextStates
}

func beamFrontierScore(score ScoreBreakdown, frontier beamFrontier) float64 {
	// P_cc is an explicit, unweighted penalty shared by every objective
	// frontier, as defined by the PRISM-CC cost function.
	return frontier.WeightTime*score.TimeScore + frontier.WeightCost*score.CostScore + score.InterferenceScore
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
	taskOrderSearch := false
	for _, state := range unique {
		taskOrderSearch = taskOrderSearch || state.TaskOrderSearch
	}
	if !taskOrderSearch {
		unique = preferPartiallyFeasibleStates(unique, generated)
		if len(unique) <= width {
			return unique
		}
		sort.SliceStable(unique, func(i, j int) bool {
			return beamStateLess(unique[i], unique[j])
		})
		return append([]beamState{}, unique[:width]...)
	}
	canonical := []beamState{}
	for _, state := range unique {
		if !state.OrderDeviated {
			canonical = append(canonical, state)
		}
	}
	sort.SliceStable(canonical, func(i, j int) bool {
		return beamStateLess(canonical[i], canonical[j])
	})
	unique = preferPartiallyFeasibleStates(unique, generated)
	sort.SliceStable(unique, func(i, j int) bool {
		return beamStateLess(unique[i], unique[j])
	})
	eliteLimit := max(1, (3*width+3)/4)
	selected := make([]beamState, 0, width)
	seen := map[string]bool{}
	for _, state := range canonical {
		if len(selected) == eliteLimit {
			break
		}
		signature := stateSignature(state)
		selected = append(selected, state)
		seen[signature] = true
	}
	for _, state := range unique {
		if len(selected) == width {
			break
		}
		signature := stateSignature(state)
		if seen[signature] {
			continue
		}
		selected = append(selected, state)
		seen[signature] = true
	}
	return selected
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
	sort.SliceStable(states, func(i, j int) bool {
		a, b := states[i], states[j]
		if a.PartialScore != b.PartialScore {
			return a.PartialScore < b.PartialScore
		}
		if a.PartialMakespan != b.PartialMakespan {
			return a.PartialMakespan < b.PartialMakespan
		}
		if a.PartialBudgetUsed != b.PartialBudgetUsed {
			return a.PartialBudgetUsed < b.PartialBudgetUsed
		}
		if a.OrderDeviated != b.OrderDeviated {
			return !a.OrderDeviated
		}
		return stateSignature(a) < stateSignature(b)
	})
	indexBySignature := map[string]int{}
	out := []beamState{}
	for _, state := range states {
		sig := stateSignature(state)
		if index, seen := indexBySignature[sig]; seen {
			// Equivalent task permutations can reach the same schedule. Keep
			// the HEFT-canonical representative even if its partial score is
			// worse than the already-seen deviated copy.
			if out[index].OrderDeviated && !state.OrderDeviated {
				out[index] = state
			}
			continue
		}
		indexBySignature[sig] = len(out)
		out = append(out, state)
	}
	return out
}

func stateSignature(state beamState) string {
	if state.Compact {
		return fmt.Sprintf("%016x", state.SignatureHash)
	}
	parts := []string{}
	for _, assignment := range state.Assignments {
		parts = append(parts, fmt.Sprintf(
			"%s:%s:%s:%.6f:%.6f",
			assignment.TaskID, assignment.ResourceID, assignment.CoreID,
			assignment.StartTime, assignment.FinishTime,
		))
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

func buildOptions(generated GeneratedSimulation, states []beamState, optionCount int, budgetLimit, deadlineLimit *float64) []ScheduleOption {
	unique := dedupeStates(states)
	if len(generated.Workflow.Tasks) > 1000 && optionCount == 1 {
		unique = selectCompactFinalState(unique, generated, budgetLimit, deadlineLimit)
	}
	built := buildOptionsParallel(generated, unique, budgetLimit, deadlineLimit)
	annotateOptionScores(built, generated)
	ranked := rankOptions(built, optionCount, generated.SLA.WeightTime, generated.SLA.WeightCost)
	if len(ranked) > optionCount {
		ranked = ranked[:optionCount]
	}
	for i := range ranked {
		ranked[i].Rank = i + 1
		ranked[i].ID = fmt.Sprintf("option-%d", i+1)
		ranked[i].ScenarioID = fmt.Sprintf("scenario-%d", i+1)
		ranked[i].ScenarioName = fmt.Sprintf("Scenario #%d", i+1)
		ranked[i].Recommended = i == 0
	}
	return ranked
}

func selectCompactFinalState(states []beamState, generated GeneratedSimulation, budgetLimit, deadlineLimit *float64) []beamState {
	if len(states) == 0 {
		return states
	}
	ranked := append([]beamState{}, states...)
	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		aBudget := violationRatio(a.PartialBudgetUsed, budgetLimit)
		bBudget := violationRatio(b.PartialBudgetUsed, budgetLimit)
		aDeadline := violationRatio(a.PartialMakespan, deadlineLimit)
		bDeadline := violationRatio(b.PartialMakespan, deadlineLimit)
		aFeasible, bFeasible := aBudget == 0 && aDeadline == 0, bBudget == 0 && bDeadline == 0
		if aFeasible != bFeasible {
			return aFeasible
		}
		if !aFeasible {
			if generated.SLA.WeightTime > generated.SLA.WeightCost {
				if aDeadline != bDeadline {
					return aDeadline < bDeadline
				}
				if aBudget != bBudget {
					return aBudget < bBudget
				}
			} else if generated.SLA.WeightCost > generated.SLA.WeightTime {
				if aBudget != bBudget {
					return aBudget < bBudget
				}
				if aDeadline != bDeadline {
					return aDeadline < bDeadline
				}
			}
		}
		if aBudget+aDeadline != bBudget+bDeadline {
			return aBudget+aDeadline < bBudget+bDeadline
		}
		if generated.SLA.WeightTime > generated.SLA.WeightCost && a.PartialMakespan != b.PartialMakespan {
			return a.PartialMakespan < b.PartialMakespan
		}
		if generated.SLA.WeightCost > generated.SLA.WeightTime && a.PartialBudgetUsed != b.PartialBudgetUsed {
			return a.PartialBudgetUsed < b.PartialBudgetUsed
		}
		return beamStateLess(a, b)
	})
	return ranked[:1]
}

func violationRatio(value float64, limit *float64) float64 {
	if limit == nil {
		return 0
	}
	return maxf(0, value-*limit) / maxf(*limit, 0.000001)
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
	assignments := stateAssignments(state)
	result := buildResult(copyGenerated, assignments, append([]MachineStopInterval{}, state.StopIntervals...), traceSteps(state.StepTrace))
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
	for _, assignment := range stateAssignments(state) {
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
