package main

import (
	"hash/fnv"
	"math"
)

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
			transfer = dep.DataMB/maxf(generated.Matrices.BandwidthBW[predecessor.ResourceID][resourceID], 0.001) +
				generated.Matrices.TransferDelay[predecessor.ResourceID][resourceID]
		}
		floor = maxf(floor, predecessor.FinishTime+transfer)
		transferTotal += transfer
	}
	return floor, transferTotal
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
