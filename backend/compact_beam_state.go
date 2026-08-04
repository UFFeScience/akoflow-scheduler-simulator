package main

import (
	"hash/fnv"
	"math"
)

func resourceImageCacheKey(resourceID, image string) string {
	return resourceID + "\x00" + image
}

func initialCachedImages(resources []Resource) map[string]bool {
	out := map[string]bool{}
	for _, resource := range resources {
		for _, image := range resource.ImageCache {
			out[resourceImageCacheKey(resource.ID, image)] = true
		}
	}
	return out
}

func dynamicContainerOverhead(generated GeneratedSimulation, state beamState, task Task, resourceID string) float64 {
	if task.Image != "" && state.CachedImages[resourceImageCacheKey(resourceID, task.Image)] {
		return 0
	}
	return generated.Matrices.ContainerOverhead[task.ID][resourceID]
}

func resourceSupportsTask(resource Resource, task Task) bool {
	return task.CPU <= resource.CPU && task.Memory <= resource.Memory
}

func stateAssignmentsForCapacity(state beamState) []Assignment {
	if !state.Compact {
		return state.Assignments
	}
	out := make([]Assignment, 0, 32)
	for trace := state.AssignmentTrace; trace != nil; trace = trace.Prev {
		out = append(out, trace.Assignment)
	}
	return out
}

// capacityConstrainedStart enforces aggregate CPU and memory for overlapping
// tasks. The common one-core-task case is already guaranteed by exclusive
// core allocation and takes the constant-time fast path.
func capacityConstrainedStart(ctx optimizerContext, state beamState, task Task, resource Resource, start, runtime float64) float64 {
	if ctx.PartitionSafe[resource.ID] {
		return start
	}
	assignments := stateAssignmentsForCapacity(state)
	for {
		finish := start + runtime
		usedCPU, usedMemory := task.CPU, task.Memory
		nextRelease := math.Inf(1)
		for _, assignment := range assignments {
			if assignment.ResourceID != resource.ID ||
				maxf(start, assignment.StartTime) >= minf(finish, assignment.FinishTime) {
				continue
			}
			other := ctx.Tasks[assignment.TaskID]
			usedCPU += other.CPU
			usedMemory += other.Memory
			if assignment.FinishTime > start {
				nextRelease = minf(nextRelease, assignment.FinishTime)
			}
		}
		if usedCPU <= resource.CPU && usedMemory <= resource.Memory {
			return start
		}
		if math.IsInf(nextRelease, 1) {
			return start
		}
		start = nextRelease
	}
}

func taskInterferenceProfile(ctx optimizerContext, task Task) string {
	dataMB := 0.0
	for _, dependency := range ctx.DepsBySource[task.ID] {
		dataMB += dependency.DataMB
	}
	for _, dependency := range ctx.DepsByTarget[task.ID] {
		dataMB += dependency.DataMB
	}
	if dataMB > maxf(1024, task.BaseRuntime*100) {
		return "network"
	}
	if task.Memory > 4*maxf(task.CPU, 0.001) {
		return "memory"
	}
	if dataMB > 0 && task.BaseRuntime <= 0.1 {
		return "io"
	}
	return "cpu"
}

func profilePairPenalty(left, right string) float64 {
	if left == right {
		return 1.25
	}
	if (left == "io" && right == "network") || (left == "network" && right == "io") {
		return 1.1
	}
	if (left == "cpu" && right == "memory") || (left == "memory" && right == "cpu") {
		return 0.75
	}
	return 1
}

func explicitProfilePCC(ctx optimizerContext, task Task, base float64, pairs []PairwiseInterference) (float64, string) {
	profile := taskInterferenceProfile(ctx, task)
	if base == 0 || len(pairs) == 0 {
		return base, profile
	}
	weight := 0.0
	for _, pair := range pairs {
		other, exists := ctx.Tasks[pair.OtherTaskID]
		if !exists {
			weight++
			continue
		}
		weight += profilePairPenalty(profile, taskInterferenceProfile(ctx, other))
	}
	return round(base*weight/float64(len(pairs)), 4), profile
}

// assignmentTrace and the two persistent treaps below let Beam children share
// their complete history. A child allocates O(log n) index nodes instead of
// copying O(n) assignments and maps.
type assignmentTrace struct {
	Assignment Assignment
	Prev       *assignmentTrace
	Len        int
}

type assignmentIndexNode struct {
	Key      int
	Value    *Assignment
	Priority uint64
	Left     *assignmentIndexNode
	Right    *assignmentIndexNode
}

type intervalIndexNode struct {
	Key       string
	Start     float64
	Finish    float64
	MaxFinish float64
	Priority  uint64
	Left      *intervalIndexNode
	Right     *intervalIndexNode
}

type coreAvailabilityNode struct {
	Key         string
	Value       float64
	Priority    uint64
	BestKey     string
	BestValue   float64
	Left, Right *coreAvailabilityNode
}

type persistentIntNode struct {
	Key      int
	Value    int
	Priority uint64
	Left     *persistentIntNode
	Right    *persistentIntNode
}

func intPriority(key int) uint64 {
	value := uint64(key + 1)
	value ^= value >> 30
	value *= 0xbf58476d1ce4e5b9
	value ^= value >> 27
	value *= 0x94d049bb133111eb
	return value ^ (value >> 31)
}

func persistentIntLookup(root *persistentIntNode, key int) (int, bool) {
	for root != nil {
		switch {
		case key < root.Key:
			root = root.Left
		case key > root.Key:
			root = root.Right
		default:
			return root.Value, true
		}
	}
	return 0, false
}

func persistentIntInsert(root *persistentIntNode, key, value int) *persistentIntNode {
	if root == nil {
		return &persistentIntNode{Key: key, Value: value, Priority: intPriority(key)}
	}
	copy := *root
	if key < root.Key {
		copy.Left = persistentIntInsert(root.Left, key, value)
		if copy.Left.Priority < copy.Priority {
			return rotatePersistentIntRight(&copy)
		}
	} else if key > root.Key {
		copy.Right = persistentIntInsert(root.Right, key, value)
		if copy.Right.Priority < copy.Priority {
			return rotatePersistentIntLeft(&copy)
		}
	} else {
		copy.Value = value
	}
	return &copy
}

func persistentIntDelete(root *persistentIntNode, key int) *persistentIntNode {
	if root == nil {
		return nil
	}
	if key < root.Key {
		copy := *root
		copy.Left = persistentIntDelete(root.Left, key)
		return &copy
	}
	if key > root.Key {
		copy := *root
		copy.Right = persistentIntDelete(root.Right, key)
		return &copy
	}
	if root.Left == nil {
		return root.Right
	}
	if root.Right == nil {
		return root.Left
	}
	if root.Left.Priority < root.Right.Priority {
		rotated := rotatePersistentIntRight(root)
		copy := *rotated
		copy.Right = persistentIntDelete(rotated.Right, key)
		return &copy
	}
	rotated := rotatePersistentIntLeft(root)
	copy := *rotated
	copy.Left = persistentIntDelete(rotated.Left, key)
	return &copy
}

func rotatePersistentIntRight(root *persistentIntNode) *persistentIntNode {
	left := *root.Left
	newRoot := *root
	newRoot.Left = left.Right
	left.Right = &newRoot
	return &left
}

func rotatePersistentIntLeft(root *persistentIntNode) *persistentIntNode {
	right := *root.Right
	newRoot := *root
	newRoot.Right = right.Left
	right.Left = &newRoot
	return &right
}

func persistentIntFirstKeys(root *persistentIntNode, limit int, out *[]int) {
	if root == nil || len(*out) >= limit {
		return
	}
	persistentIntFirstKeys(root.Left, limit, out)
	if len(*out) < limit {
		*out = append(*out, root.Key)
	}
	persistentIntFirstKeys(root.Right, limit, out)
}

func stablePriority(value string) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(value))
	return hash.Sum64()
}

func appendAssignmentTrace(prev *assignmentTrace, assignment Assignment) *assignmentTrace {
	length := 1
	if prev != nil {
		length = prev.Len + 1
	}
	return &assignmentTrace{Assignment: assignment, Prev: prev, Len: length}
}

func traceAssignments(trace *assignmentTrace) []Assignment {
	if trace == nil {
		return []Assignment{}
	}
	out := make([]Assignment, trace.Len)
	for item := trace; item != nil; item = item.Prev {
		out[item.Len-1] = item.Assignment
	}
	return out
}

func assignmentIndexLookup(root *assignmentIndexNode, key int) (Assignment, bool) {
	for root != nil {
		switch {
		case key < root.Key:
			root = root.Left
		case key > root.Key:
			root = root.Right
		default:
			return *root.Value, true
		}
	}
	return Assignment{}, false
}

func assignmentIndexInsert(root *assignmentIndexNode, key int, value Assignment) *assignmentIndexNode {
	if root == nil {
		stored := value
		return &assignmentIndexNode{Key: key, Value: &stored, Priority: intPriority(key)}
	}
	copy := *root
	if key < root.Key {
		copy.Left = assignmentIndexInsert(root.Left, key, value)
		if copy.Left.Priority < copy.Priority {
			return rotateAssignmentRight(&copy)
		}
	} else if key > root.Key {
		copy.Right = assignmentIndexInsert(root.Right, key, value)
		if copy.Right.Priority < copy.Priority {
			return rotateAssignmentLeft(&copy)
		}
	} else {
		stored := value
		copy.Value = &stored
	}
	return &copy
}

func rotateAssignmentRight(root *assignmentIndexNode) *assignmentIndexNode {
	left := *root.Left
	newRoot := *root
	newRoot.Left = left.Right
	left.Right = &newRoot
	return &left
}

func rotateAssignmentLeft(root *assignmentIndexNode) *assignmentIndexNode {
	right := *root.Right
	newRoot := *root
	newRoot.Right = right.Left
	right.Left = &newRoot
	return &right
}

func intervalKey(assignment Assignment) string {
	return assignment.TaskID
}

func intervalMax(root *intervalIndexNode) float64 {
	if root == nil {
		return 0
	}
	return root.MaxFinish
}

func refreshInterval(root *intervalIndexNode) {
	root.MaxFinish = maxf(root.Finish, intervalMax(root.Left), intervalMax(root.Right))
}

func intervalLess(start float64, key string, root *intervalIndexNode) bool {
	return start < root.Start || (start == root.Start && key < root.Key)
}

func intervalIndexInsert(root *intervalIndexNode, assignment Assignment) *intervalIndexNode {
	key := intervalKey(assignment)
	if root == nil {
		return &intervalIndexNode{
			Key: key, Start: assignment.StartTime, Finish: assignment.FinishTime,
			MaxFinish: assignment.FinishTime, Priority: stablePriority(key),
		}
	}
	copy := *root
	if intervalLess(assignment.StartTime, key, root) {
		copy.Left = intervalIndexInsert(root.Left, assignment)
		if copy.Left.Priority < copy.Priority {
			return rotateIntervalRight(&copy)
		}
	} else {
		copy.Right = intervalIndexInsert(root.Right, assignment)
		if copy.Right.Priority < copy.Priority {
			return rotateIntervalLeft(&copy)
		}
	}
	refreshInterval(&copy)
	return &copy
}

func rotateIntervalRight(root *intervalIndexNode) *intervalIndexNode {
	left := *root.Left
	newRoot := *root
	newRoot.Left = left.Right
	refreshInterval(&newRoot)
	left.Right = &newRoot
	refreshInterval(&left)
	return &left
}

func rotateIntervalLeft(root *intervalIndexNode) *intervalIndexNode {
	right := *root.Right
	newRoot := *root
	newRoot.Right = right.Left
	refreshInterval(&newRoot)
	right.Left = &newRoot
	refreshInterval(&right)
	return &right
}

func intervalOverlapCount(root *intervalIndexNode, start, finish float64) int {
	if root == nil || root.MaxFinish <= start {
		return 0
	}
	count := intervalOverlapCount(root.Left, start, finish)
	if root.Start < finish && start < root.Finish {
		count++
	}
	if root.Start < finish {
		count += intervalOverlapCount(root.Right, start, finish)
	}
	return count
}

func intervalOverlapKeys(root *intervalIndexNode, start, finish float64, keys *[]string) {
	if root == nil || root.MaxFinish <= start {
		return
	}
	intervalOverlapKeys(root.Left, start, finish, keys)
	if maxf(start, root.Start) < minf(finish, root.Finish) {
		*keys = append(*keys, root.Key)
	}
	if root.Start < finish {
		intervalOverlapKeys(root.Right, start, finish, keys)
	}
}

func stateAssignments(state beamState) []Assignment {
	if state.Compact {
		return traceAssignments(state.AssignmentTrace)
	}
	return append([]Assignment{}, state.Assignments...)
}

func stateAssignment(state beamState, taskID string) (Assignment, bool) {
	if state.Compact {
		ordinal, exists := state.TaskOrdinals[taskID]
		if !exists {
			return Assignment{}, false
		}
		return assignmentIndexLookup(state.AssignmentIndex, ordinal)
	}
	assignment, ok := state.AssignmentByTask[taskID]
	return assignment, ok
}

func predecessorTimingForState(deps []Dependency, state beamState, generated GeneratedSimulation, resourceID string) (float64, float64) {
	floor, transferTotal := 0.0, 0.0
	for _, dep := range deps {
		predecessor, ok := stateAssignment(state, dep.Source)
		if !ok {
			continue
		}
		transfer := 0.0
		if predecessor.ResourceID != resourceID {
			transfer = dependencyTransferSeconds(
				dep, generated.Matrices.BandwidthBW[predecessor.ResourceID][resourceID],
				generated.Matrices.TransferDelay[predecessor.ResourceID][resourceID],
			)
		}
		floor = maxf(floor, predecessor.FinishTime+transfer)
		transferTotal += transfer
	}
	return floor, transferTotal
}

const maxAdaptiveLookaheadDepth = 8

// adaptiveTaskLookaheadDepths assigns a structural upper bound to each task.
// Ordinary tasks inspect one level, critical tasks inspect three levels, and a
// communication-heavy fork follows its branches up to the nearest join.
func adaptiveTaskLookaheadDepths(generated GeneratedSimulation, ctx optimizerContext) map[string]int {
	depths := map[string]int{}
	averageData := 0.0
	for _, dependency := range generated.Workflow.Dependencies {
		averageData += dependency.DataMB
	}
	averageData /= maxf(float64(len(generated.Workflow.Dependencies)), 1)
	for taskID := range ctx.Tasks {
		dependencies := ctx.DepsBySource[taskID]
		if len(dependencies) == 0 {
			depths[taskID] = 0
			continue
		}
		depth := 1
		if ctx.PriorityRanks[taskID] >= 0.8*ctx.MaxPriorityRank {
			depth = 3
		}
		outgoingData := 0.0
		for _, dependency := range dependencies {
			outgoingData += dependency.DataMB
		}
		if len(dependencies) > 1 && outgoingData/float64(len(dependencies)) >= averageData {
			depth = max(depth, nearestJoinDepth(ctx, taskID))
		}
		depths[taskID] = min(maxAdaptiveLookaheadDepth, depth)
	}
	return depths
}

func nearestJoinDepth(ctx optimizerContext, sourceTaskID string) int {
	frontier := []string{sourceTaskID}
	seen := map[string]bool{sourceTaskID: true}
	for depth := 1; depth <= maxAdaptiveLookaheadDepth; depth++ {
		next := []string{}
		for _, taskID := range frontier {
			for _, dependency := range ctx.DepsBySource[taskID] {
				if len(ctx.DepsByTarget[dependency.Target]) > 1 {
					return depth
				}
				if !seen[dependency.Target] {
					seen[dependency.Target] = true
					next = append(next, dependency.Target)
				}
			}
		}
		if len(next) == 0 {
			return depth
		}
		frontier = next
	}
	return maxAdaptiveLookaheadDepth
}

// buildAdaptiveSuccessorDelay caches an optimistic resource-sensitive future
// cost for every useful depth. Each level includes transfer, latency, image
// overhead and execution, while choosing the best compatible next resource.
func buildAdaptiveSuccessorDelay(generated GeneratedSimulation, ctx optimizerContext, depths map[string]int) map[string]map[string][]float64 {
	out := map[string]map[string][]float64{}
	type memoKey struct {
		taskID, resourceID string
		depth              int
	}
	memo := map[memoKey]float64{}
	var visit func(string, string, int) float64
	visit = func(taskID, resourceID string, depth int) float64 {
		if depth <= 0 {
			return 0
		}
		key := memoKey{taskID, resourceID, depth}
		if value, exists := memo[key]; exists {
			return value
		}
		criticalDelay := 0.0
		for _, dependency := range ctx.DepsBySource[taskID] {
			successor, exists := ctx.Tasks[dependency.Target]
			if !exists {
				continue
			}
			bestDelay := math.Inf(1)
			for _, targetResource := range generated.Resources {
				if !resourceSupportsTask(targetResource, successor) {
					continue
				}
				transfer := 0.0
				if resourceID != targetResource.ID {
					transfer = dependencyTransferSeconds(
						dependency, generated.Matrices.BandwidthBW[resourceID][targetResource.ID],
						generated.Matrices.TransferDelay[resourceID][targetResource.ID],
					)
				}
				delay := transfer + generated.Matrices.ContainerOverhead[successor.ID][targetResource.ID] +
					generated.Matrices.ET0[successor.ID][targetResource.ID] + visit(successor.ID, targetResource.ID, depth-1)
				bestDelay = minf(bestDelay, delay)
			}
			if !math.IsInf(bestDelay, 1) {
				criticalDelay = maxf(criticalDelay, bestDelay)
			}
		}
		memo[key] = round(criticalDelay, 3)
		return memo[key]
	}
	for taskID, maxDepth := range depths {
		out[taskID] = map[string][]float64{}
		for _, resource := range generated.Resources {
			values := make([]float64, maxDepth+1)
			for depth := 1; depth <= maxDepth; depth++ {
				values[depth] = visit(taskID, resource.ID, depth)
			}
			out[taskID][resource.ID] = values
		}
	}
	return out
}

func adaptiveSuccessorLookaheadFinish(ctx optimizerContext, task Task, candidate Assignment, depth int) float64 {
	values := ctx.SuccessorDelay[task.ID][candidate.ResourceID]
	if depth <= 0 || len(values) == 0 {
		return candidate.FinishTime
	}
	depth = min(depth, len(values)-1)
	return round(candidate.FinishTime+values[depth], 3)
}

// adaptiveDecisionLookaheadDepth disables speculative work when one resource
// clearly dominates both finish time and financial cost. Otherwise it uses the
// structural depth selected from criticality and the DAG's fork/join shape.
func adaptiveDecisionLookaheadDepth(ctx optimizerContext, task Task, finishes, costs []float64) int {
	depth := ctx.LookaheadDepth[task.ID]
	if depth == 0 || len(finishes) < 2 {
		return depth
	}
	best := 0
	for index := 1; index < len(finishes); index++ {
		if finishes[index] < finishes[best] ||
			(finishes[index] == finishes[best] && costs[index] < costs[best]) {
			best = index
		}
	}
	secondFinish := math.Inf(1)
	secondCost := math.Inf(1)
	for index := range finishes {
		if index == best {
			continue
		}
		secondFinish = minf(secondFinish, finishes[index])
		secondCost = minf(secondCost, costs[index])
	}
	finishClearlyBetter := finishes[best] <= 0.95*secondFinish
	costNotWorse := costs[best] <= secondCost+1e-9
	if finishClearlyBetter && costNotWorse {
		return 0
	}
	return depth
}

func candidatePairwiseInterferenceForState(generated GeneratedSimulation, taskID, resourceID string, state beamState, start, finish float64) (float64, []PairwiseInterference) {
	if !state.Compact {
		return candidatePairwiseInterference(generated, taskID, resourceID, state.Assignments, start, finish)
	}
	metadata := generated.Experimental
	if metadata == nil || metadata.InterferenceDisabled || !metadata.interferenceActivitySet[taskID] {
		return 0, []PairwiseInterference{}
	}
	if state.Compact {
		if metadata.ScenarioID == "edge_cloud_interference_aware" {
			keys := []string{}
			intervalOverlapKeys(state.SelectedIntervals[resourceID], start, finish, &keys)
			total := 0.0
			for _, otherID := range keys {
				total += controlledPairInterference(metadata, resourceID, otherID, taskID)
			}
			return round(total, 4), nil
		}
		count := intervalOverlapCount(state.SelectedIntervals[resourceID], start, finish)
		return round(float64(count)*metadata.InterferenceRate, 4), nil
	}
	return 0, nil
}

func incrementalMachineActiveCostForState(state beamState, candidate Assignment, resource Resource) float64 {
	if !state.Compact {
		return incrementalMachineActiveCost(state.Assignments, candidate, resource)
	}
	if resource.PricePerHourUSD == 0 {
		return 0
	}
	candidateStart := assignmentBillingStart(candidate)
	beforeStart, exists := state.BillingStart[resource.ID]
	beforeFinish := state.BillingFinish[resource.ID]
	if !exists {
		return maxf(0, candidate.FinishTime-candidateStart) * resource.PricePerHourUSD / 3600
	}
	before := maxf(0, beforeFinish-beforeStart)
	after := maxf(0, maxf(beforeFinish, candidate.FinishTime)-minf(beforeStart, candidateStart))
	return maxf(0, after-before) * resource.PricePerHourUSD / 3600
}

func updatedBillingBounds(state beamState, assignment Assignment) (map[string]float64, map[string]float64) {
	starts := copyFloatMap(state.BillingStart)
	finishes := copyFloatMap(state.BillingFinish)
	start := assignmentBillingStart(assignment)
	if previous, ok := starts[assignment.ResourceID]; !ok {
		starts[assignment.ResourceID] = start
		finishes[assignment.ResourceID] = assignment.FinishTime
	} else {
		starts[assignment.ResourceID] = math.Min(previous, start)
		finishes[assignment.ResourceID] = math.Max(finishes[assignment.ResourceID], assignment.FinishTime)
	}
	return starts, finishes
}

func copyIntervalRootMap(in map[string]*intervalIndexNode) map[string]*intervalIndexNode {
	out := make(map[string]*intervalIndexNode, len(in)+1)
	for key, value := range in {
		out[key] = value
	}
	return out
}

func refreshCore(root *coreAvailabilityNode) {
	root.BestKey, root.BestValue = root.Key, root.Value
	for _, child := range []*coreAvailabilityNode{root.Left, root.Right} {
		if child != nil && (child.BestValue < root.BestValue ||
			(child.BestValue == root.BestValue && child.BestKey < root.BestKey)) {
			root.BestKey, root.BestValue = child.BestKey, child.BestValue
		}
	}
}

func coreIndexInsert(root *coreAvailabilityNode, key string, value float64) *coreAvailabilityNode {
	if root == nil {
		return &coreAvailabilityNode{Key: key, Value: value, Priority: stablePriority(key), BestKey: key, BestValue: value}
	}
	copy := *root
	if key < root.Key {
		copy.Left = coreIndexInsert(root.Left, key, value)
		if copy.Left.Priority < copy.Priority {
			return rotateCoreRight(&copy)
		}
	} else if key > root.Key {
		copy.Right = coreIndexInsert(root.Right, key, value)
		if copy.Right.Priority < copy.Priority {
			return rotateCoreLeft(&copy)
		}
	} else {
		copy.Value = value
	}
	refreshCore(&copy)
	return &copy
}

func rotateCoreRight(root *coreAvailabilityNode) *coreAvailabilityNode {
	left := *root.Left
	newRoot := *root
	newRoot.Left = left.Right
	refreshCore(&newRoot)
	left.Right = &newRoot
	refreshCore(&left)
	return &left
}

func rotateCoreLeft(root *coreAvailabilityNode) *coreAvailabilityNode {
	right := *root.Right
	newRoot := *root
	newRoot.Right = right.Left
	refreshCore(&newRoot)
	right.Left = &newRoot
	refreshCore(&right)
	return &right
}

func coreIndexLookup(root *coreAvailabilityNode, key string) float64 {
	for root != nil {
		if key < root.Key {
			root = root.Left
		} else if key > root.Key {
			root = root.Right
		} else {
			return root.Value
		}
	}
	return 0
}

func copyCoreRootMap(in map[string]*coreAvailabilityNode) map[string]*coreAvailabilityNode {
	out := make(map[string]*coreAvailabilityNode, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func earliestAvailableCoreForState(resource Resource, state beamState) Core {
	if !state.Compact {
		return earliestAvailableCore(resource, state.CoreAvail)
	}
	root := state.CoreIndexes[resource.ID]
	if root == nil {
		return resource.Cores[0]
	}
	return Core{ID: root.BestKey}
}

func coreAvailabilityForState(state beamState, resourceID, coreID string) float64 {
	if !state.Compact {
		return state.CoreAvail[coreID]
	}
	return coreIndexLookup(state.CoreIndexes[resourceID], coreID)
}
