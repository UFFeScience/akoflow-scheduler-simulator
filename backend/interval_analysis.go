package main

import "sort"

// analyzeAssignmentOverlaps uses a sweep line per resource. Its work is
// O(n log n + p), where p is the number of actual overlapping pairs.
func analyzeAssignmentOverlaps(assignments []Assignment, selected map[string]bool) (map[string][]string, int) {
	byResource := map[string][]Assignment{}
	colocated := make(map[string][]string, len(assignments))
	for _, assignment := range assignments {
		byResource[assignment.ResourceID] = append(byResource[assignment.ResourceID], assignment)
		colocated[assignment.TaskID] = []string{}
	}
	pairCount := 0
	for _, resourceAssignments := range byResource {
		sort.Slice(resourceAssignments, func(i, j int) bool {
			if resourceAssignments[i].StartTime != resourceAssignments[j].StartTime {
				return resourceAssignments[i].StartTime < resourceAssignments[j].StartTime
			}
			return resourceAssignments[i].TaskID < resourceAssignments[j].TaskID
		})
		active := []Assignment{}
		for _, current := range resourceAssignments {
			kept := active[:0]
			for _, previous := range active {
				if previous.FinishTime <= current.StartTime {
					continue
				}
				kept = append(kept, previous)
				colocated[current.TaskID] = append(colocated[current.TaskID], previous.TaskID)
				colocated[previous.TaskID] = append(colocated[previous.TaskID], current.TaskID)
				if selected != nil && selected[current.TaskID] && selected[previous.TaskID] {
					pairCount++
				}
			}
			active = append(kept, current)
		}
	}
	for taskID := range colocated {
		sort.Strings(colocated[taskID])
	}
	return colocated, pairCount
}
