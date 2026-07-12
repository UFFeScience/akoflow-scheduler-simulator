package main

import (
	"fmt"
	"math/rand"
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
			priceMultiplier := 0.0
			status := "warm"
			if spec.Kind == "cloud" {
				priceMultiplier = 1.0
				status = "cold"
			}
			_, stages := stagesForPreset(presets[(index+1)%len(presets)]["id"].(string))
			cacheCount := rng.Intn(len(stages)) + 1
			cache := []string{}
			for _, stage := range stages[:cacheCount] {
				cache = append(cache, strings.ToLower(stage)+":latest")
			}
			finPrice := 0.0
			if spec.Kind == "cloud" {
				finPrice = round(0.0008+rng.Float64()*0.0052, 5)
			}
			boot := 0.0
			if spec.Kind == "cloud" {
				boot = round(spec.BootOverhead, 3)
			}
			resources = append(resources, Resource{
				ID: spec.ID, Name: spec.Name, Kind: spec.Kind, Cores: makeCores(spec.ID, spec.Cores), CPU: float64(spec.Cores),
				Memory: round(spec.Memory, 2), PricePerCPUSecond: round((0.006+rng.Float64()*0.019)*priceMultiplier, 5),
				PricePerGBSecond: round((0.001+rng.Float64()*0.005)*priceMultiplier, 5), FinancialNetworkPrice: finPrice,
				Bandwidth: round(spec.Bandwidth, 2), Location: spec.Location, Status: status, BootOverhead: boot, ImageCache: cache,
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
				multiplier := 1.0
				if left.Location == right.Location {
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

func makeCores(resourceID string, count int) []Core {
	cores := make([]Core, 0, count)
	for i := 0; i < count; i++ {
		cores = append(cores, Core{ID: fmt.Sprintf("%s-core-%d", resourceID, i+1), Index: i, Avail: 0})
	}
	return cores
}

func generateInterference(req SimulationRequest, workflow Workflow, resources []Resource) map[string]map[string]map[string]map[string]float64 {
	rng := rand.New(rand.NewSource(req.Seed + 23))
	matrix := map[string]map[string]map[string]map[string]float64{}
	dimensions := []string{"cpu", "memory", "io", "network"}
	for _, resource := range resources {
		matrix[resource.ID] = map[string]map[string]map[string]float64{}
		for _, dimension := range dimensions {
			matrix[resource.ID][dimension] = map[string]map[string]float64{}
			for _, source := range workflow.Tasks {
				matrix[resource.ID][dimension][source.ID] = map[string]float64{}
				for _, target := range workflow.Tasks {
					value := 0.0
					if source.ID != target.ID {
						value = rng.Float64() * 0.18
					}
					matrix[resource.ID][dimension][source.ID][target.ID] = round(value, 4)
				}
			}
		}
	}
	return matrix
}
