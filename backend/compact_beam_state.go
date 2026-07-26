package main

import (
	"hash/fnv"
	"math"
	"sort"
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
	Key      string
	Value    Assignment
	Priority uint64
	Left     *assignmentIndexNode
	Right    *assignmentIndexNode
}

type intervalIndexNode struct {
	Key        string
	Start      float64
	Finish     float64
	MaxFinish  float64
	Assignment Assignment
	Priority   uint64
	Left       *intervalIndexNode
	Right      *intervalIndexNode
}

type coreAvailabilityNode struct {
	Key         string
	Value       float64
	Priority    uint64
	BestKey     string
	BestValue   float64
	Left, Right *coreAvailabilityNode
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

func assignmentIndexLookup(root *assignmentIndexNode, key string) (Assignment, bool) {
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
	return Assignment{}, false
}

func assignmentIndexInsert(root *assignmentIndexNode, key string, value Assignment) *assignmentIndexNode {
	if root == nil {
		return &assignmentIndexNode{Key: key, Value: value, Priority: stablePriority(key)}
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
		copy.Value = value
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
			MaxFinish: assignment.FinishTime, Assignment: assignment,
			Priority: stablePriority(key),
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

func intervalOverlaps(root *intervalIndexNode, start, finish float64, out *[]Assignment) {
	if root == nil || root.MaxFinish <= start {
		return
	}
	intervalOverlaps(root.Left, start, finish, out)
	if root.Start < finish && start < root.Finish {
		*out = append(*out, root.Assignment)
	}
	if root.Start < finish {
		intervalOverlaps(root.Right, start, finish, out)
	}
}

func sortedOverlaps(root *intervalIndexNode, start, finish float64) []Assignment {
	out := []Assignment{}
	intervalOverlaps(root, start, finish, &out)
	sort.Slice(out, func(i, j int) bool { return out[i].TaskID < out[j].TaskID })
	return out
}

func stateAssignments(state beamState) []Assignment {
	if state.Compact {
		return traceAssignments(state.AssignmentTrace)
	}
	return append([]Assignment{}, state.Assignments...)
}

func stateAssignment(state beamState, taskID string) (Assignment, bool) {
	if state.Compact {
		return assignmentIndexLookup(state.AssignmentIndex, taskID)
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
			transfer = dep.DataMB / maxf(generated.Matrices.BandwidthBW[predecessor.ResourceID][resourceID], 0.001)
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
	overlaps := sortedOverlaps(state.SelectedIntervals[resourceID], start, finish)
	if state.Compact {
		return round(float64(len(overlaps))*metadata.InterferenceRate, 4), nil
	}
	pairs := make([]PairwiseInterference, 0, len(overlaps))
	for _, item := range overlaps {
		pairs = append(pairs, PairwiseInterference{
			OtherTaskID: item.TaskID, Value: metadata.InterferenceRate,
			Dimensions: map[string]float64{"controlled": metadata.InterferenceRate},
		})
	}
	return round(float64(len(pairs))*metadata.InterferenceRate, 4), pairs
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
