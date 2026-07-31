package main

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"
	"strings"
)

func generateResources(req SimulationRequest) ([]Resource, map[string]map[string]float64, error) {
	rng := rand.New(rand.NewSource(req.Seed + 11))
	resources := []Resource{}
	locations := []string{"eu-west", "us-east", "ap-south", "on-prem"}
	makeResource := func(kind string, index int) Resource {
		nodeID := fmt.Sprintf("c%d", index)
		if kind == "cloud" {
			nodeID = fmt.Sprintf("v%d", index)
		}
		memBase, priceMultiplier := 16.0, 0.0
		if kind == "cloud" {
			memBase, priceMultiplier = 24.0, 1.0
		}
		_, stages := stagesForPreset(presets[index%len(presets)]["id"].(string))
		status := "warm"
		if kind == "cloud" {
			status = "cold"
		}
		boot := 0.0
		if status != "warm" {
			if kind == "cloud" {
				boot = round(6.0+rng.Float64()*12.0, 3)
			} else {
				boot = round(2.0+rng.Float64()*4.0, 3)
			}
		}
		cacheCount := rng.Intn(len(stages)) + 1
		cache := []string{}
		for _, stage := range stages[:cacheCount] {
			cache = append(cache, strings.ToLower(stage)+":latest")
		}
		return Resource{
			ID: nodeID, Name: fmt.Sprintf("%s-%d", kind, index), Kind: kind,
			Cores: makeCores(nodeID, req.CoresPerMachine), CPU: float64(req.CoresPerMachine),
			Memory:            round(memBase+rng.Float64()*32.0, 2),
			PricePerCPUSecond: round((0.006+rng.Float64()*0.019)*priceMultiplier, 5),
			PricePerGBSecond:  round((0.001+rng.Float64()*0.005)*priceMultiplier, 5),
			FinancialNetworkPrice: func() float64 {
				if kind == "cluster" {
					return 0
				}
				return round(0.0008+rng.Float64()*0.0052, 5)
			}(),
			Bandwidth: round(func() float64 {
				if kind == "cluster" {
					return 650 + rng.Float64()*550
				}
				return 120 + rng.Float64()*730
			}(), 2),
			Location: locations[(index+map[bool]int{true: 1, false: 0}[kind == "cloud"])%len(locations)],
			Status:   status, BootOverhead: boot, ImageCache: cache,
		}
	}
	if len(req.ResourceSpecs) > 0 {
		seen := map[string]bool{}
		for index, spec := range req.ResourceSpecs {
			if seen[spec.ID] {
				return nil, nil, fmt.Errorf("Duplicate resource id: %s", spec.ID)
			}
			seen[spec.ID] = true
			status := "warm"
			if spec.Kind == "cloud" {
				status = "cold"
			}
			_, stages := stagesForPreset(presets[(index+1)%len(presets)]["id"].(string))
			cacheCount := rng.Intn(len(stages)) + 1
			cache := []string{}
			for _, stage := range stages[:cacheCount] {
				cache = append(cache, strings.ToLower(stage)+":latest")
			}
			finPrice := spec.NetworkPricePerGBUSD / 1024
			boot := 0.0
			if spec.Kind == "cloud" {
				boot = round(spec.BootOverhead, 3)
			}
			resources = append(resources, Resource{
				ID: spec.ID, Name: spec.Name, Kind: spec.Kind, Cores: makeCores(spec.ID, spec.Cores), CPU: float64(spec.Cores),
				Memory: round(spec.Memory, 2), PricePerCPUSecond: spec.PricePerHourUSD / float64(spec.Cores) / 3600,
				PricePerGBSecond: 0, FinancialNetworkPrice: finPrice,
				Bandwidth: round(spec.Bandwidth, 2), Location: spec.Location, Status: status, BootOverhead: boot, Speedup: spec.Speedup,
				NetworkLatencyMS: spec.NetworkLatencyMS,
				PricePerHourUSD:  spec.PricePerHourUSD, PricingModel: spec.PricingModel, ImageCache: cache,
			})
		}
	} else {
		for i := 1; i <= req.ClusterMachines; i++ {
			resources = append(resources, makeResource("cluster", i))
		}
		for i := 1; i <= req.CloudMachines; i++ {
			resources = append(resources, makeResource("cloud", i))
		}
	}
	bandwidth := map[string]map[string]float64{}
	for _, left := range resources {
		bandwidth[left.ID] = map[string]float64{}
		for _, right := range resources {
			if left.ID == right.ID {
				bandwidth[left.ID][right.ID] = 10000
			} else if len(req.ResourceSpecs) > 0 {
				if isRealNetworkStressScenario(req.ExperimentScenarioID) {
					bandwidth[left.ID][right.ID] = realNetworkStressBandwidthMBps(left, right)
					continue
				}
				if req.ExperimentScenarioID == "hybrid_communication_trap" ||
					req.ExperimentScenarioID == "hybrid_heft_network_trap" {
					bandwidth[left.ID][right.ID] = communicationTrapBandwidthMBps(left.ID, right.ID)
					continue
				}
				multiplier := 1.0
				if left.Location == right.Location && req.ExperimentScenarioID != "hybrid_raspberry_500mbps" {
					multiplier = 1.5
				}
				bandwidth[left.ID][right.ID] = round(minf(left.Bandwidth, right.Bandwidth)*multiplier, 2)
			} else {
				multiplier := 1.0
				if left.Location == right.Location {
					multiplier = 2
				}
				bandwidth[left.ID][right.ID] = round((80+rng.Float64()*870)*multiplier, 2)
			}
		}
	}
	return resources, bandwidth, nil
}

func realNetworkStressTier(resource Resource) string {
	if strings.HasPrefix(resource.ID, "rpi-edge-") {
		return "fog"
	}
	if resource.Kind == "cloud" {
		return "cloud"
	}
	return "hpc"
}

func realNetworkStressDomain(resourceID string) int {
	number := 0
	multiplier := 1
	for index := len(resourceID) - 1; index >= 0; index-- {
		character := resourceID[index]
		if character < '0' || character > '9' {
			break
		}
		number += int(character-'0') * multiplier
		multiplier *= 10
	}
	if number == 0 {
		return int(stablePriority(resourceID) % 2)
	}
	return (number - 1) / 2
}

// Values are grounded in published interface/egress limits:
// Raspberry Pi 3: 100 Mbps Ethernet and approximately 35 Mbps Wi-Fi;
// Google H3/H4D: 200 Gbps internal and 1 Gbps outside the VPC;
// Google C3: 23 Gbps internal for small VMs and 3 Gbps external per flow.
func realNetworkStressBandwidthMBps(left, right Resource) float64 {
	leftTier, rightTier := realNetworkStressTier(left), realNetworkStressTier(right)
	if leftTier == rightTier {
		sameDomain := realNetworkStressDomain(left.ID) == realNetworkStressDomain(right.ID)
		switch leftTier {
		case "fog":
			if sameDomain {
				return megabitsPerSecondToMegabytesPerSecond(100)
			}
			return megabitsPerSecondToMegabytesPerSecond(35)
		case "cloud":
			if sameDomain {
				return gigabitsPerSecondToMegabytesPerSecond(23)
			}
			return gigabitsPerSecondToMegabytesPerSecond(3)
		default:
			if sameDomain {
				return gigabitsPerSecondToMegabytesPerSecond(200)
			}
			return gigabitsPerSecondToMegabytesPerSecond(1)
		}
	}
	pair := leftTier + "-" + rightTier
	if pair == "fog-cloud" || pair == "cloud-fog" {
		return megabitsPerSecondToMegabytesPerSecond(35)
	}
	if pair == "fog-hpc" || pair == "hpc-fog" {
		return megabitsPerSecondToMegabytesPerSecond(100)
	}
	return gigabitsPerSecondToMegabytesPerSecond(1)
}

func realNetworkStressLatencySeconds(left, right Resource) float64 {
	leftTier, rightTier := realNetworkStressTier(left), realNetworkStressTier(right)
	if leftTier == rightTier {
		if realNetworkStressDomain(left.ID) == realNetworkStressDomain(right.ID) {
			return map[string]float64{"fog": 0.005, "hpc": 0.001, "cloud": 0.002}[leftTier]
		}
		return map[string]float64{"fog": 0.015, "hpc": 0.020, "cloud": 0.030}[leftTier]
	}
	pair := leftTier + "-" + rightTier
	if pair == "fog-cloud" || pair == "cloud-fog" {
		return 0.120
	}
	if pair == "fog-hpc" || pair == "hpc-fog" {
		return 0.040
	}
	return 0.080
}

func communicationTrapTier(resourceID string) string {
	if strings.HasPrefix(resourceID, "trap-fog-") || strings.HasPrefix(resourceID, "hefttrap-fog-") {
		return "fog"
	}
	if strings.HasPrefix(resourceID, "trap-hpc-") || strings.HasPrefix(resourceID, "hefttrap-hpc-") {
		return "hpc"
	}
	return "cloud"
}

func communicationTrapBandwidthMBps(leftID, rightID string) float64 {
	leftTier, rightTier := communicationTrapTier(leftID), communicationTrapTier(rightID)
	heftTrap := strings.HasPrefix(leftID, "hefttrap-") || strings.HasPrefix(rightID, "hefttrap-")
	if heftTrap {
		if leftTier == rightTier {
			return megabitsPerSecondToMegabytesPerSecond(2000)
		}
		pair := leftTier + "-" + rightTier
		if pair == "cloud-fog" || pair == "fog-cloud" {
			return megabitsPerSecondToMegabytesPerSecond(5)
		}
		if pair == "fog-hpc" || pair == "hpc-fog" {
			return megabitsPerSecondToMegabytesPerSecond(25)
		}
		return megabitsPerSecondToMegabytesPerSecond(20)
	}
	if leftTier == rightTier {
		return megabitsPerSecondToMegabytesPerSecond(750)
	}
	pair := leftTier + "-" + rightTier
	if pair == "cloud-fog" || pair == "fog-cloud" {
		return megabitsPerSecondToMegabytesPerSecond(50)
	}
	if pair == "fog-hpc" || pair == "hpc-fog" {
		return megabitsPerSecondToMegabytesPerSecond(100)
	}
	return megabitsPerSecondToMegabytesPerSecond(200)
}

func communicationTrapLatencySeconds(leftID, rightID string) float64 {
	leftTier, rightTier := communicationTrapTier(leftID), communicationTrapTier(rightID)
	heftTrap := strings.HasPrefix(leftID, "hefttrap-") || strings.HasPrefix(rightID, "hefttrap-")
	if heftTrap {
		if leftTier == rightTier {
			switch leftTier {
			case "fog":
				return 0.005
			case "hpc":
				return 0.002
			default:
				return 0.025
			}
		}
		pair := leftTier + "-" + rightTier
		if pair == "cloud-fog" || pair == "fog-cloud" {
			return 0.200
		}
		if pair == "fog-hpc" || pair == "hpc-fog" {
			return 0.080
		}
		return 0.120
	}
	if leftTier == rightTier {
		switch leftTier {
		case "fog":
			return 0.010
		case "hpc":
			return 0.003
		default:
			return 0.020
		}
	}
	pair := leftTier + "-" + rightTier
	if pair == "cloud-fog" || pair == "fog-cloud" {
		return 0.130
	}
	if pair == "fog-hpc" || pair == "hpc-fog" {
		return 0.030
	}
	return 0.080
}

func makeCores(resourceID string, count int) []Core {
	cores := make([]Core, 0, count)
	for i := 0; i < count; i++ {
		cores = append(cores, Core{ID: fmt.Sprintf("%s-core-%d", resourceID, i+1), Index: i, Avail: 0})
	}
	return cores
}

func generateInterference(req SimulationRequest, workflow Workflow, resources []Resource) map[string]map[string]map[string]map[string]float64 {
	matrix := map[string]map[string]map[string]map[string]float64{}
	dimensions := []string{"cpu", "memory", "io", "network"}
	if len(workflow.Tasks) > 500 {
		for _, resource := range resources {
			matrix[resource.ID] = map[string]map[string]map[string]float64{}
			for _, dimension := range dimensions {
				matrix[resource.ID][dimension] = map[string]map[string]float64{}
			}
		}
		return matrix
	}
	for _, resource := range resources {
		matrix[resource.ID] = map[string]map[string]map[string]float64{}
		for _, dimension := range dimensions {
			matrix[resource.ID][dimension] = map[string]map[string]float64{}
			for _, source := range workflow.Tasks {
				matrix[resource.ID][dimension][source.ID] = map[string]float64{}
				for _, target := range workflow.Tasks {
					value := 0.0
					if source.ID != target.ID {
						value = fixedInterferenceValue(dimension, source.ID, target.ID)
					}
					matrix[resource.ID][dimension][source.ID][target.ID] = round(value, 4)
				}
			}
		}
	}
	return matrix
}

func fixedInterferenceValue(dimension, sourceID, targetID string) float64 {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte("scheduler-simulator/interference/v1|" + dimension + "|" + sourceID + "|" + targetID))
	return round(float64(hash.Sum32()%1801)/10000.0, 4)
}

func controlledPairInterference(metadata *ExperimentMetadata, resourceID, sourceID, targetID string) float64 {
	if metadata == nil {
		return 0
	}
	if metadata.ScenarioID != "edge_cloud_interference_aware" {
		return metadata.InterferenceRate
	}
	resourceFactor := map[string]float64{
		"edge-isolated-01":   0.25,
		"fog-shared-01":      1.20,
		"cloud-balanced-01":  0.60,
		"cloud-burst-shared": 1.60,
	}[resourceID]
	if resourceFactor == 0 {
		resourceFactor = 1
	}
	profile := func(taskID string) uint32 {
		hash := fnv.New32a()
		_, _ = hash.Write([]byte("scheduler-simulator/task-profile/v1|" + taskID))
		return hash.Sum32() % 4
	}
	pairFactor := 0.45
	if profile(sourceID) == profile(targetID) {
		pairFactor = 1.25
	}
	return round(metadata.InterferenceRate*resourceFactor*pairFactor, 4)
}

func applyControlledInterference(generated *GeneratedSimulation, seed int64, rate float64, disabled bool) {
	taskIDs := make([]string, 0, len(generated.Workflow.Tasks))
	for _, task := range generated.Workflow.Tasks {
		taskIDs = append(taskIDs, task.ID)
	}
	sort.Strings(taskIDs)
	selected := []string{}
	if !disabled {
		rng := rand.New(rand.NewSource(seed))
		rng.Shuffle(len(taskIDs), func(i, j int) { taskIDs[i], taskIDs[j] = taskIDs[j], taskIDs[i] })
		selected = append(selected, taskIDs[:len(taskIDs)/2]...)
		sort.Strings(selected)
	}
	selectedSet := map[string]bool{}
	for _, id := range selected {
		selectedSet[id] = true
	}
	for resourceID, dimensions := range generated.Matrices.InterferenceIN {
		for dimension, sources := range dimensions {
			for sourceID, targets := range sources {
				for targetID := range targets {
					value := 0.0
					if sourceID != targetID && selectedSet[sourceID] && selectedSet[targetID] {
						value = rate
					}
					generated.Matrices.InterferenceIN[resourceID][dimension][sourceID][targetID] = value
				}
			}
		}
	}
	generated.Experimental = &ExperimentMetadata{
		InterferenceSeed:        seed,
		InterferenceActivityIDs: selected, InterferenceRate: rate, InterferenceDisabled: disabled,
		interferenceActivitySet: selectedSet,
	}
}
