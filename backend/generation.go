package main

import (
	"fmt"
	"math/rand"
	"strings"
)

func generateSimulation(req SimulationRequest) (GeneratedSimulation, error) {
	if req.ExperimentScenarioID != "" {
		specs, err := experimentScenarioResources(req.ExperimentScenarioID)
		if err != nil {
			return GeneratedSimulation{}, err
		}
		req.ResourceSpecs = specs
		req.ClusterMachines = 1
		req.CloudMachines = 0
		workflowFile := experimentWorkflowYAMLFile(req.ExperimentWorkflowID)
		workflowYAML, err := readExperimentText(workflowFile)
		if err != nil {
			return GeneratedSimulation{}, err
		}
		req.WorkflowYAML = &workflowYAML
	}
	workflow, err := generateWorkflow(req)
	if err != nil {
		return GeneratedSimulation{}, err
	}
	if req.ExperimentScenarioID != "" && strings.EqualFold(req.Preset, "Montage") {
		var runtimeErr error
		if req.ExperimentWorkflowID == imageDataflow8WorkflowID {
			runtimeErr = applyImageDataflow8ExperimentData(&workflow)
		} else if req.ExperimentWorkflowID == montageDSS20WorkflowID {
			runtimeErr = applyMontageDSS20ExperimentData(&workflow)
		} else if _, exists := wfcommonsWorkflowDatasets[req.ExperimentWorkflowID]; exists {
			runtimeErr = applyWfcommonsExperimentData(req.ExperimentWorkflowID, &workflow)
		} else {
			runtimeErr = applyMontageExperimentRuntimes(&workflow)
		}
		if runtimeErr != nil {
			return GeneratedSimulation{}, runtimeErr
		}
		if req.ExperimentScenarioID == "edge_cloud_communication_dominant" {
			for index := range workflow.Dependencies {
				workflow.Dependencies[index].DataMB = round(
					workflow.Dependencies[index].DataMB*250, 9,
				)
			}
		}
		if req.ExperimentScenarioID == "hybrid_communication_trap" {
			for index := range workflow.Dependencies {
				workflow.Dependencies[index].DataMB = round(
					workflow.Dependencies[index].DataMB*25, 9,
				)
			}
		}
		dataScale := req.ExperimentDataScale
		if dataScale <= 0 {
			dataScale = 1
		}
		if dataScale != 1 {
			for index := range workflow.Dependencies {
				workflow.Dependencies[index].DataMB = round(
					workflow.Dependencies[index].DataMB*dataScale, 9,
				)
			}
		}
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
			if resource.Speedup > 0 {
				speed = resource.Speedup
			}
			et0[task.ID][resource.ID] = round(task.BaseRuntime/speed, 6)
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
			if isRealNetworkStressScenario(req.ExperimentScenarioID) {
				transferDelay[left.ID][right.ID] = realNetworkStressLatencySeconds(left, right)
			} else if isNetworkCriticalScenario(req.ExperimentScenarioID) {
				transferDelay[left.ID][right.ID] = networkCriticalLatencySeconds(
					req.ExperimentScenarioID, left, right,
				)
			} else if req.ExperimentScenarioID == "hybrid_communication_trap" ||
				req.ExperimentScenarioID == "hybrid_heft_network_trap" {
				transferDelay[left.ID][right.ID] = communicationTrapLatencySeconds(left.ID, right.ID)
			} else {
				transferDelay[left.ID][right.ID] = round(
					(left.NetworkLatencyMS+right.NetworkLatencyMS)/2000.0, 6,
				)
			}
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
