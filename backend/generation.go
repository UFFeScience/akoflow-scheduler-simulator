package main

import (
	"fmt"
	"math/rand"
)

func generateSimulation(req SimulationRequest) (GeneratedSimulation, error) {
	workflow, err := generateWorkflow(req)
	if err != nil {
		return GeneratedSimulation{}, err
	}
	resources, bandwidth, err := generateResources(req)
	if err != nil {
		return GeneratedSimulation{}, err
	}
	interference := generateInterference(req, workflow, resources)
	rng := rand.New(rand.NewSource(req.Seed + 37))
	et0 := map[string]map[string]float64{}
	container := map[string]map[string]float64{}
	for _, task := range workflow.Tasks {
		et0[task.ID] = map[string]float64{}
		container[task.ID] = map[string]float64{}
		for _, resource := range resources {
			imageHit := false
			for _, image := range resource.ImageCache {
				if image == task.Image {
					imageHit = true
					break
				}
			}
			speed := maxf(0.35, resource.CPU/maxf(task.CPU, 0.1))
			et0[task.ID][resource.ID] = round(task.BaseRuntime/speed+rng.Float64()*4.0, 3)
			baseOverhead := 3.5
			if imageHit {
				baseOverhead = 0.4
			}
			container[task.ID][resource.ID] = round(baseOverhead+rng.Float64()*1.8, 3)
		}
	}
	transferDelay := map[string]map[string]float64{}
	financialCost := map[string]map[string]float64{}
	for _, left := range resources {
		transferDelay[left.ID] = map[string]float64{}
		financialCost[left.ID] = map[string]float64{}
		for _, right := range resources {
			if left.ID == right.ID {
				transferDelay[left.ID][right.ID] = 0
				financialCost[left.ID][right.ID] = 0
				continue
			}
			transferDelay[left.ID][right.ID] = round(100.0/bandwidth[left.ID][right.ID], 4)
			if left.Kind == "cluster" && right.Kind == "cluster" {
				financialCost[left.ID][right.ID] = 0
			} else {
				financialCost[left.ID][right.ID] = round((left.FinancialNetworkPrice+right.FinancialNetworkPrice)/2, 5)
			}
		}
	}
	etStar := map[string]map[string]float64{}
	for _, task := range workflow.Tasks {
		etStar[task.ID] = map[string]float64{}
		for _, resource := range resources {
			etStar[task.ID][resource.ID] = et0[task.ID][resource.ID]
		}
	}
	clusterCount, cloudCount := 0, 0
	for _, resource := range resources {
		if resource.Kind == "cluster" {
			clusterCount++
		} else if resource.Kind == "cloud" {
			cloudCount++
		}
	}
	return GeneratedSimulation{
		ID:   fmt.Sprintf("sim-%d-%d-%d-%d", req.Seed, len(workflow.Tasks), clusterCount, cloudCount),
		Seed: req.Seed, Workflow: workflow, Resources: resources,
		SLA:      SLA{WeightTime: req.WeightTime, WeightCost: req.WeightCost, BudgetLimit: req.BudgetLimit, DeadlineLimit: req.DeadlineLimit, OptionCount: req.OptionCount, BeamWidth: req.BeamWidth},
		Matrices: Matrices{ET0: et0, ETStar: etStar, InterferenceIN: interference, BandwidthBW: bandwidth, TransferDelay: transferDelay, FinancialNetworkCost: financialCost, ContainerOverhead: container},
	}, nil
}
