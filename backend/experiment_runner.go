package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

var experimentScenarioIDs = []string{
	"cluster_homo", "cluster_hetero", "cloud_homo", "cloud_hetero", "hybrid_homo", "hybrid_hetero",
}

var experimentSupportedScenarioIDs = append(
	append([]string(nil), experimentScenarioIDs...),
	"edge_cloud_extreme", "edge_cloud_communication_dominant", "edge_cloud_interference_aware",
	"hybrid_raspberry_500mbps",
	"hybrid_communication_trap",
	"hybrid_heft_network_trap",
	"real_network_stress_cluster_homo",
	"real_network_stress_cluster_hetero",
	"real_network_stress_cloud_homo",
	"real_network_stress_cloud_hetero",
	"real_network_stress_hybrid_homo",
	"real_network_stress_hybrid_hetero",
	"real_network_stress_hybrid_raspberry_500mbps",
	"wfcommons_chameleon_dss20",
	"network_hpc_local",
	"network_hpc_multisite",
	"network_cloud_multiregion",
	"network_hpc_cloud",
	"network_edge_cloud",
	"network_fog_hpc_cloud",
	"network_wfcommons_overlay",
)

const (
	c3dStandard16ReferencePricePerHourUSD = 0.726384
	c3dStandard16ReferenceMakespanSeconds = 2815.0
	c3dStandard16ReferenceBudgetUSD       = c3dStandard16ReferencePricePerHourUSD * c3dStandard16ReferenceMakespanSeconds / 3600
	experimentSLAMargin                   = 1.20
	prismImprovementEpsilon               = 1e-6
)

type ExperimentRunOptions struct {
	OutputDirectory          string
	Repetitions              int
	StructuralSeed           int64
	BeamWidth                int
	RecommendationCount      int
	Workers                  int
	PRISMCCPriority          string
	WorkflowID               string
	HEFTMode                 string
	ScenarioIDs              []string
	InterferenceRate         float64
	FixedBudgetLimit         float64
	FixedDeadlineLimit       float64
	BudgetMargin             float64
	DeadlineMargin           float64
	DataScale                float64
	NetworkLatencyMS         float64
	NetworkBandwidthMbps     float64
	ExportSchedules          bool
	DisableContainerOverhead bool
}

type ExperimentRecord struct {
	Algorithm              string
	ScenarioID             string
	InterferenceSeed       int64
	InterferenceRate       float64
	InterferenceActivities []string
	Makespan               float64
	BudgetUsed             float64
	BudgetLimit            float64
	DeadlineLimit          float64
	BudgetViolation        float64
	DeadlineViolation      float64
	Feasible               bool
	InterferenceTime       float64
	InterferencePairs      int
	AlgorithmMilliseconds  float64
	MachineDistribution    map[string]int
	MachineUtilization     map[string]float64
	Recommendations        []ExperimentRecommendationRecord
}

type ExperimentRecommendationRecord struct {
	Rank              int     `json:"rank"`
	Recommended       bool    `json:"recommended"`
	Feasible          bool    `json:"feasible"`
	BudgetUsed        float64 `json:"budget_used"`
	BudgetViolation   float64 `json:"budget_violation"`
	Makespan          float64 `json:"makespan"`
	DeadlineViolation float64 `json:"deadline_violation"`
	MachineSignature  string  `json:"machine_signature"`
	WeightedScore     float64 `json:"weighted_score"`
	DiversityScore    float64 `json:"diversity_score"`
}

type ExperimentManifest struct {
	GeneratedAt               string                 `json:"generated_at"`
	StructuralSeed            int64                  `json:"structural_seed"`
	InterferenceSeeds         []int64                `json:"interference_seeds"`
	Scenarios                 []string               `json:"scenarios"`
	Algorithms                []string               `json:"algorithms"`
	WorkflowID                string                 `json:"workflow_id"`
	TaskCount                 int                    `json:"task_count"`
	InterferenceRate          float64                `json:"interference_rate"`
	SelectedActivities        int                    `json:"selected_activities"`
	SLAMargin                 float64                `json:"sla_margin"`
	BudgetMargin              float64                `json:"budget_margin"`
	DeadlineMargin            float64                `json:"deadline_margin"`
	ScenarioSLAs              map[string]ScenarioSLA `json:"scenario_slas"`
	ReferencePolicy           string                 `json:"reference_policy"`
	Calibration               string                 `json:"calibration"`
	HEFTMode                  string                 `json:"heft_mode"`
	PRISMCCPriority           string                 `json:"prism_cc_priority"`
	BeamWidth                 int                    `json:"beam_width"`
	RecommendationCount       int                    `json:"recommendation_count"`
	DataScale                 float64                `json:"data_scale"`
	ContainerOverheadDisabled bool                   `json:"container_overhead_disabled"`
	FileTransferParallelism   int                    `json:"file_transfer_parallelism"`
	NetworkLatencyOverrideMS  float64                `json:"network_latency_override_ms,omitempty"`
	NetworkBandwidthMbps      float64                `json:"network_bandwidth_override_mbps,omitempty"`
}

type ScenarioSLA struct {
	BudgetLimit   float64 `json:"budget_limit"`
	DeadlineLimit float64 `json:"deadline_limit"`
}

func splitNonEmptyCSV(value string) []string {
	items := []string{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func experimentReferencePolicy(options ExperimentRunOptions, baselineAlgorithm string) string {
	if options.FixedBudgetLimit > 0 {
		return fmt.Sprintf(
			"Fixed SLA across interference levels: budget %.6f USD and deadline %.6f s",
			options.FixedBudgetLimit, options.FixedDeadlineLimit,
		)
	}
	return fmt.Sprintf(
		"Per-scenario %s mean multiplied by explicit margins: %.2fx budget and %.2fx deadline",
		baselineAlgorithm, options.BudgetMargin, options.DeadlineMargin,
	)
}

func runExperimentalProtocol(options ExperimentRunOptions) error {
	if options.Repetitions <= 0 {
		options.Repetitions = 30
	}
	if options.StructuralSeed == 0 {
		options.StructuralSeed = 42
	}
	if options.BeamWidth == 0 {
		options.BeamWidth = minBeamWidth
	}
	options.BeamWidth = normalizedBeamWidth(options.BeamWidth)
	if options.RecommendationCount <= 0 {
		options.RecommendationCount = 100
	}
	options.RecommendationCount = min(options.RecommendationCount, maxScheduleOptions)
	if options.PRISMCCPriority == "" {
		options.PRISMCCPriority = "topological_order"
	}
	if options.WorkflowID == "" {
		options.WorkflowID = "montage_050d"
	}
	if options.HEFTMode == "" {
		options.HEFTMode = "classic_no_colocation"
	}
	if options.InterferenceRate < 0 || options.InterferenceRate > 1 {
		return fmt.Errorf("experiment interference rate must be between 0 and 1, got %v", options.InterferenceRate)
	}
	if options.DataScale == 0 {
		options.DataScale = 1
	}
	if options.DataScale <= 0 {
		return fmt.Errorf("experiment data scale must be greater than zero")
	}
	if options.NetworkLatencyMS < 0 {
		return fmt.Errorf("experiment network latency must be non-negative")
	}
	if options.NetworkBandwidthMbps < 0 {
		return fmt.Errorf("experiment network bandwidth must be non-negative")
	}
	if options.BudgetMargin <= 0 {
		options.BudgetMargin = experimentSLAMargin
	}
	if options.DeadlineMargin <= 0 {
		options.DeadlineMargin = experimentSLAMargin
	}
	scenarioIDs := options.ScenarioIDs
	if len(scenarioIDs) == 0 {
		scenarioIDs = append([]string(nil), experimentScenarioIDs...)
	}
	validScenarios := map[string]bool{}
	for _, scenarioID := range experimentSupportedScenarioIDs {
		validScenarios[scenarioID] = true
	}
	for _, scenarioID := range scenarioIDs {
		if !validScenarios[scenarioID] {
			return fmt.Errorf("unsupported experiment scenario %q", scenarioID)
		}
	}
	if (options.FixedBudgetLimit > 0) != (options.FixedDeadlineLimit > 0) {
		return fmt.Errorf("fixed budget and deadline limits must be provided together")
	}
	if !experimentWorkflowSupported(options.WorkflowID) {
		return fmt.Errorf("unsupported experiment workflow %q", options.WorkflowID)
	}
	if options.PRISMCCPriority != "topological_order" && options.PRISMCCPriority != "upward_rank" &&
		options.PRISMCCPriority != "ready_lookahead" && options.PRISMCCPriority != "adaptive_ready" {
		return fmt.Errorf("unsupported PRISM-CC priority policy %q", options.PRISMCCPriority)
	}
	if options.HEFTMode != "classic_no_colocation" && options.HEFTMode != "colocation" {
		return fmt.Errorf("unsupported HEFT mode %q", options.HEFTMode)
	}
	baselineAlgorithm := heftAlgorithmForMode(options.HEFTMode)
	experimentAlgorithms := []string{"prism_cc_time", "prism_cc_cost", baselineAlgorithm}
	if options.OutputDirectory == "" {
		options.OutputDirectory = filepath.Join("experiments", "results")
	}
	if err := validateExperimentScenarios(); err != nil {
		return err
	}
	if err := os.MkdirAll(options.OutputDirectory, 0o755); err != nil {
		return err
	}

	type calibratedHEFTRun struct {
		scenarioID       string
		interferenceSeed int64
		record           ExperimentRecord
		result           SimulationResult
	}
	heftRuns := make([]calibratedHEFTRun, 0, len(scenarioIDs)*options.Repetitions)
	heftMakespans := map[string][]float64{}
	heftBudgets := map[string][]float64{}
	records := make([]ExperimentRecord, 0, len(scenarioIDs)*len(experimentAlgorithms)*options.Repetitions)
	seeds := make([]int64, 0, options.Repetitions)
	taskCount := 0
	for repetition := 1; repetition <= options.Repetitions; repetition++ {
		seeds = append(seeds, int64(repetition))
	}
	for _, scenarioID := range scenarioIDs {
		var referenceResult SimulationResult
		referenceElapsed := 0.0
		for repetition := 1; repetition <= options.Repetitions; repetition++ {
			interferenceSeed := int64(repetition)
			heftGenerated, generationErr := generateExperimentSimulationForWorkflowAtRateAndDataScale(
				scenarioID, options.WorkflowID, options.StructuralSeed, interferenceSeed, false,
				options.BeamWidth, options.InterferenceRate, options.DataScale,
			)
			if generationErr != nil {
				return fmt.Errorf("%s seed %d HEFT generation: %w", scenarioID, interferenceSeed, generationErr)
			}
			applyNetworkLatencyOverride(&heftGenerated, options.NetworkLatencyMS)
			applyNetworkBandwidthOverride(&heftGenerated, options.NetworkBandwidthMbps)
			if options.DisableContainerOverhead {
				disableContainerOverhead(&heftGenerated)
			}
			if taskCount == 0 {
				taskCount = len(heftGenerated.Workflow.Tasks)
			}
			var heftResult SimulationResult
			if repetition == 1 || options.HEFTMode == "colocation" {
				heftGenerated.Experimental.Algorithm = baselineAlgorithm
				runStarted := time.Now()
				var scheduleErr error
				referenceResult, scheduleErr = scheduleHEFTBaseline(heftGenerated, options.HEFTMode)
				if scheduleErr != nil {
					return fmt.Errorf("%s/heft seed %d: %w", scenarioID, interferenceSeed, scheduleErr)
				}
				referenceElapsed = float64(time.Since(runStarted).Microseconds()) / 1000
			}
			heftResult = referenceResult
			heftResult.Experimental = heftGenerated.Experimental
			heftRuns = append(heftRuns, calibratedHEFTRun{
				scenarioID: scenarioID, interferenceSeed: interferenceSeed,
				result: heftResult,
				record: experimentRecordFromResult(
					heftResult, baselineAlgorithm, scenarioID, interferenceSeed, 0, 0, referenceElapsed,
				),
			})
			heftMakespans[scenarioID] = append(heftMakespans[scenarioID], heftResult.TimingVariables.Makespan)
			heftBudgets[scenarioID] = append(heftBudgets[scenarioID], heftResult.CostVariables.BUsed)
		}
	}

	scenarioSLAs := map[string]ScenarioSLA{}
	for _, scenarioID := range scenarioIDs {
		if options.FixedBudgetLimit > 0 {
			scenarioSLAs[scenarioID] = ScenarioSLA{
				BudgetLimit: options.FixedBudgetLimit, DeadlineLimit: options.FixedDeadlineLimit,
			}
		} else {
			scenarioSLAs[scenarioID] = ScenarioSLA{
				BudgetLimit:   round(mean(heftBudgets[scenarioID])*options.BudgetMargin, 6),
				DeadlineLimit: round(mean(heftMakespans[scenarioID])*options.DeadlineMargin, 6),
			}
		}
	}
	for _, heftRun := range heftRuns {
		sla := scenarioSLAs[heftRun.scenarioID]
		record := heftRun.record
		record.BudgetLimit = sla.BudgetLimit
		record.DeadlineLimit = sla.DeadlineLimit
		record.BudgetViolation = round(maxf(0, record.BudgetUsed-sla.BudgetLimit), 4)
		record.DeadlineViolation = round(maxf(0, record.Makespan-sla.DeadlineLimit), 3)
		record.Feasible = record.BudgetViolation == 0 && record.DeadlineViolation == 0
		records = append(records, record)
		if options.ExportSchedules {
			result := heftRun.result
			result.SLA.BudgetLimit = &sla.BudgetLimit
			result.SLA.DeadlineLimit = &sla.DeadlineLimit
			if err := writeExperimentSchedule(
				options.OutputDirectory, heftRun.scenarioID, heftRun.interferenceSeed,
				baselineAlgorithm, result,
			); err != nil {
				return err
			}
		}
	}

	type prismJob struct {
		scenarioID string
		seed       int64
	}
	type prismJobResult struct {
		scenarioID string
		seed       int64
		elapsed    time.Duration
		records    []ExperimentRecord
		err        error
	}
	jobs := make(chan prismJob)
	jobResults := make(chan prismJobResult)
	jobCount := options.Repetitions * len(scenarioIDs)
	workerCount := options.Workers
	if workerCount <= 0 {
		workerCount = min(4, runtime.GOMAXPROCS(0))
	}
	workerCount = min(workerCount, runtime.GOMAXPROCS(0), jobCount)
	for worker := 0; worker < workerCount; worker++ {
		go func(workerID int) {
			for job := range jobs {
				log.Printf(
					"worker %d started: scenario=%s seed=%d PRISM-CC-Time+Cost",
					workerID, job.scenarioID, job.seed,
				)
				started := time.Now()
				jobRecords, jobErr := runPRISMExperimentJob(
					options, job.scenarioID, job.seed, scenarioSLAs[job.scenarioID],
				)
				jobResults <- prismJobResult{
					scenarioID: job.scenarioID, seed: job.seed, elapsed: time.Since(started),
					records: jobRecords, err: jobErr,
				}
			}
		}(worker + 1)
	}
	protocolStarted := time.Now()
	log.Printf(
		"PRISM-CC experiment started: %d environment/seed combinations, %d workers, %d result rows expected",
		jobCount, workerCount, jobCount*2,
	)
	go func() {
		for repetition := 1; repetition <= options.Repetitions; repetition++ {
			for _, scenarioID := range scenarioIDs {
				jobs <- prismJob{scenarioID: scenarioID, seed: int64(repetition)}
			}
		}
		close(jobs)
	}()
	for completed := 0; completed < jobCount; completed++ {
		jobResult := <-jobResults
		if jobResult.err != nil {
			return jobResult.err
		}
		records = append(records, jobResult.records...)
		done := completed + 1
		remaining := jobCount - done
		totalElapsed := time.Since(protocolStarted)
		eta := time.Duration(0)
		if done > 0 {
			eta = time.Duration(float64(totalElapsed) / float64(done) * float64(remaining))
		}
		log.Printf(
			"completed %d/%d (%.1f%%): scenario=%s seed=%d PRISM-CC-Time+Cost duration=%s remaining=%d ETA=%s",
			done, jobCount, 100*float64(done)/float64(jobCount),
			jobResult.scenarioID, jobResult.seed, jobResult.elapsed.Round(time.Second),
			remaining, eta.Round(time.Second),
		)
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].InterferenceSeed != records[j].InterferenceSeed {
			return records[i].InterferenceSeed < records[j].InterferenceSeed
		}
		if records[i].ScenarioID != records[j].ScenarioID {
			return records[i].ScenarioID < records[j].ScenarioID
		}
		return records[i].Algorithm < records[j].Algorithm
	})

	if err := writeExperimentRawCSV(filepath.Join(options.OutputDirectory, "raw_results.csv"), records); err != nil {
		return err
	}
	if err := writeExperimentSummaryCSV(filepath.Join(options.OutputDirectory, "summary.csv"), records); err != nil {
		return err
	}
	manifest := ExperimentManifest{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339), StructuralSeed: options.StructuralSeed,
		InterferenceSeeds: seeds, Scenarios: scenarioIDs, Algorithms: experimentAlgorithms,
		WorkflowID: options.WorkflowID, TaskCount: taskCount,
		InterferenceRate: options.InterferenceRate, SelectedActivities: taskCount / 2,
		SLAMargin: experimentSLAMargin, ScenarioSLAs: scenarioSLAs,
		BudgetMargin: options.BudgetMargin, DeadlineMargin: options.DeadlineMargin,
		ReferencePolicy: experimentReferencePolicy(options, baselineAlgorithm),
		Calibration:     "Per-scenario mean of the selected HEFT baseline runs",
		HEFTMode:        options.HEFTMode, PRISMCCPriority: options.PRISMCCPriority,
		BeamWidth: options.BeamWidth, RecommendationCount: options.RecommendationCount,
		DataScale:                 options.DataScale,
		ContainerOverheadDisabled: options.DisableContainerOverhead,
		FileTransferParallelism:   fileTransferParallelism,
		NetworkLatencyOverrideMS:  options.NetworkLatencyMS,
		NetworkBandwidthMbps:      options.NetworkBandwidthMbps,
	}
	return writeJSONFile(filepath.Join(options.OutputDirectory, "manifest.json"), manifest)
}

func heftAlgorithmForMode(mode string) string {
	if mode == "colocation" {
		return "heft_colocation"
	}
	return "heft_classic"
}

func scheduleHEFTBaseline(generated GeneratedSimulation, mode string) (SimulationResult, error) {
	if mode == "colocation" {
		return scheduleHEFTColocation(generated)
	}
	return scheduleHEFTClassic(generated)
}

func runPRISMExperimentJob(options ExperimentRunOptions, scenarioID string, interferenceSeed int64, sla ScenarioSLA) ([]ExperimentRecord, error) {
	generated, generationErr := generateExperimentSimulationForWorkflowAtRateAndDataScale(
		scenarioID, options.WorkflowID, options.StructuralSeed, interferenceSeed, false,
		options.BeamWidth, options.InterferenceRate, options.DataScale,
	)
	if generationErr != nil {
		return nil, fmt.Errorf("%s seed %d generation: %w", scenarioID, interferenceSeed, generationErr)
	}
	applyNetworkLatencyOverride(&generated, options.NetworkLatencyMS)
	applyNetworkBandwidthOverride(&generated, options.NetworkBandwidthMbps)
	if options.DisableContainerOverhead {
		disableContainerOverhead(&generated)
	}
	generated.SLA.BudgetLimit = &sla.BudgetLimit
	generated.SLA.DeadlineLimit = &sla.DeadlineLimit
	generated.Experimental.PriorityPolicy = options.PRISMCCPriority
	heftAnchor, heftErr := scheduleHEFTBaseline(generated, options.HEFTMode)
	if heftErr != nil {
		return nil, fmt.Errorf("%s/HEFT anchor seed %d: %w", scenarioID, interferenceSeed, heftErr)
	}
	runStarted := time.Now()
	finalStates, searchErr := beamSearch(generated, normalizedBeamWidth(generated.SLA.BeamWidth))
	searchMilliseconds := float64(time.Since(runStarted).Microseconds()) / 1000
	if searchErr != nil {
		return nil, fmt.Errorf("%s/PRISM-CC seed %d: %w", scenarioID, interferenceSeed, searchErr)
	}
	records := make([]ExperimentRecord, 0, 2)
	for _, algorithm := range []string{"prism_cc_time", "prism_cc_cost"} {
		scored := generated
		metadataCopy := *generated.Experimental
		scored.Experimental = &metadataCopy
		scored.Experimental.Algorithm = algorithm
		if algorithm == "prism_cc_time" {
			scored.SLA.WeightTime, scored.SLA.WeightCost = 1, 0
		} else {
			scored.SLA.WeightTime, scored.SLA.WeightCost = 0, 1
		}
		response := buildOptions(
			scored, finalStates, options.RecommendationCount,
			scored.SLA.BudgetLimit, scored.SLA.DeadlineLimit,
		)
		if len(response) == 0 {
			return nil, fmt.Errorf("%s/%s seed %d: beam returned no schedule", scenarioID, algorithm, interferenceSeed)
		}
		selected := selectAnchoredPRISMResult(algorithm, heftAnchor, response, sla)
		if selected.Experimental != nil {
			metadata := *selected.Experimental
			metadata.Algorithm = algorithm
			metadata.PriorityPolicy = options.PRISMCCPriority
			selected.Experimental = &metadata
		}
		if options.ExportSchedules {
			if err := writeExperimentSchedule(
				options.OutputDirectory, scenarioID, interferenceSeed, algorithm, selected,
			); err != nil {
				return nil, err
			}
		}
		record := experimentRecordFromResult(
			selected, algorithm, scenarioID, interferenceSeed, sla.BudgetLimit, sla.DeadlineLimit,
			searchMilliseconds,
		)
		for _, option := range response {
			record.Recommendations = append(record.Recommendations, ExperimentRecommendationRecord{
				Rank: option.Rank, Recommended: option.Recommended, Feasible: option.Feasible,
				BudgetUsed: option.BudgetUsed, BudgetViolation: option.BudgetViolation,
				Makespan: option.Makespan, DeadlineViolation: option.DeadlineViolation,
				MachineSignature: compactRecommendationSignature(option.MachineSignature),
				WeightedScore:    option.WeightedScore,
				DiversityScore:   option.DiversityScore,
			})
		}
		records = append(records, record)
	}
	return records, nil
}

func applyNetworkLatencyOverride(generated *GeneratedSimulation, latencyMS float64) {
	if latencyMS <= 0 {
		return
	}
	latencySeconds := latencyMS / 1000.0
	for sourceID, targets := range generated.Matrices.TransferDelay {
		for targetID := range targets {
			if sourceID == targetID {
				targets[targetID] = 0
			} else {
				targets[targetID] = latencySeconds
			}
		}
	}
}

func applyNetworkBandwidthOverride(generated *GeneratedSimulation, bandwidthMbps float64) {
	if bandwidthMbps <= 0 {
		return
	}
	bandwidthMBps := megabitsPerSecondToMegabytesPerSecond(bandwidthMbps)
	for sourceID, targets := range generated.Matrices.BandwidthBW {
		for targetID := range targets {
			if sourceID != targetID {
				targets[targetID] = bandwidthMBps
			}
		}
	}
}

func selectAnchoredPRISMResult(algorithm string, heft SimulationResult, options []ScheduleOption, sla ScenarioSLA) SimulationResult {
	selected := heft
	for _, option := range options {
		if algorithm == "prism_cc_time" {
			if option.Makespan < selected.TimingVariables.Makespan-prismImprovementEpsilon {
				selected = option.Result
			}
			continue
		}
		if !option.Feasible ||
			option.Makespan > sla.DeadlineLimit+prismImprovementEpsilon ||
			option.BudgetUsed > sla.BudgetLimit+prismImprovementEpsilon {
			continue
		}
		costImproved := option.BudgetUsed < selected.CostVariables.BUsed-prismImprovementEpsilon
		costTied := math.Abs(option.BudgetUsed-selected.CostVariables.BUsed) <= prismImprovementEpsilon
		if costImproved || (costTied && option.Makespan < selected.TimingVariables.Makespan-prismImprovementEpsilon) {
			selected = option.Result
		}
	}
	return selected
}

func compactRecommendationSignature(signature string) string {
	sum := sha256.Sum256([]byte(signature))
	return fmt.Sprintf("%x", sum[:8])
}

func generateExperimentSimulation(scenarioID string, structuralSeed, interferenceSeed int64, disabled bool, beamWidth int) (GeneratedSimulation, error) {
	return generateExperimentSimulationForWorkflow(
		scenarioID, "montage_050d", structuralSeed, interferenceSeed, disabled, beamWidth,
	)
}

func generateExperimentSimulationForWorkflow(scenarioID, workflowID string, structuralSeed, interferenceSeed int64, disabled bool, beamWidth int) (GeneratedSimulation, error) {
	return generateExperimentSimulationForWorkflowAtRate(
		scenarioID, workflowID, structuralSeed, interferenceSeed, disabled, beamWidth, 0.20,
	)
}

func generateExperimentSimulationForWorkflowAtRate(scenarioID, workflowID string, structuralSeed, interferenceSeed int64, disabled bool, beamWidth int, interferenceRate float64) (GeneratedSimulation, error) {
	return generateExperimentSimulationForWorkflowAtRateAndDataScale(
		scenarioID, workflowID, structuralSeed, interferenceSeed, disabled, beamWidth, interferenceRate, 1,
	)
}

func generateExperimentSimulationForWorkflowAtRateAndDataScale(scenarioID, workflowID string, structuralSeed, interferenceSeed int64, disabled bool, beamWidth int, interferenceRate, dataScale float64) (GeneratedSimulation, error) {
	req := defaultRequest()
	req.Preset = "Montage"
	req.ExperimentScenarioID = scenarioID
	req.ExperimentWorkflowID = workflowID
	req.ExperimentDataScale = dataScale
	req.Seed = structuralSeed
	req.TaskCount = experimentWorkflowTaskCount(workflowID)
	req.OptionCount = 1
	req.BeamWidth = beamWidth
	generated, err := generateSimulation(req)
	if err != nil {
		return GeneratedSimulation{}, err
	}
	applyControlledInterference(&generated, interferenceSeed, interferenceRate, disabled)
	generated.Experimental.ScenarioID = scenarioID
	return generated, nil
}

func validateExperimentScenarios() error {
	rows, err := readExperimentMachines()
	if err != nil {
		return err
	}
	counts := map[string]int{}
	for _, row := range rows {
		counts[row.ScenarioID]++
		if row.Speedup <= 0 {
			return fmt.Errorf("scenario %s machine %s has invalid speedup", row.ScenarioID, row.MachineID)
		}
	}
	for _, scenarioID := range experimentSupportedScenarioIDs {
		expected := 4
		baseScenarioID := realNetworkStressBaseScenario(scenarioID)
		if baseScenarioID == "hybrid_raspberry_500mbps" || scenarioID == "hybrid_communication_trap" ||
			scenarioID == "hybrid_heft_network_trap" {
			expected = 14
		} else if scenarioID == "wfcommons_chameleon_dss20" || scenarioID == "network_wfcommons_overlay" {
			expected = 5
		} else if scenarioID == "network_edge_cloud" {
			expected = 4
		} else if scenarioID == "network_fog_hpc_cloud" {
			expected = 14
		}
		if counts[baseScenarioID] != expected {
			return fmt.Errorf("scenario %s must define exactly %d machines, got %d", scenarioID, expected, counts[baseScenarioID])
		}
	}
	return nil
}

func experimentRecordFromResult(result SimulationResult, algorithm, scenarioID string, seed int64, budgetLimit, deadlineLimit, elapsed float64) ExperimentRecord {
	budgetViolation := maxf(0, result.CostVariables.BUsed-budgetLimit)
	deadlineViolation := maxf(0, result.TimingVariables.Makespan-deadlineLimit)
	distribution := map[string]int{}
	busy := map[string]float64{}
	for _, assignment := range result.Assignments {
		distribution[assignment.ResourceID]++
		busy[assignment.ResourceID] += assignment.EffectiveRuntime
	}
	utilization := map[string]float64{}
	for _, resource := range result.Resources {
		denominator := result.TimingVariables.Makespan * float64(len(resource.Cores))
		utilization[resource.ID] = round(busy[resource.ID]/maxf(denominator, 0.001), 6)
	}
	activities := []string{}
	if result.Experimental != nil {
		activities = append(activities, result.Experimental.InterferenceActivityIDs...)
	}
	return ExperimentRecord{
		Algorithm: algorithm, ScenarioID: scenarioID, InterferenceSeed: seed, InterferenceActivities: activities,
		InterferenceRate: result.Experimental.InterferenceRate,
		Makespan:         result.TimingVariables.Makespan, BudgetUsed: result.CostVariables.BUsed,
		BudgetLimit: budgetLimit, DeadlineLimit: deadlineLimit, BudgetViolation: round(budgetViolation, 4),
		DeadlineViolation: round(deadlineViolation, 3), Feasible: budgetViolation == 0 && deadlineViolation == 0,
		InterferenceTime:  result.InterferenceVariables.TotalInterferenceTime,
		InterferencePairs: countEffectiveInterferencePairs(result), AlgorithmMilliseconds: round(elapsed, 3),
		MachineDistribution: distribution, MachineUtilization: utilization,
		Recommendations: []ExperimentRecommendationRecord{},
	}
}

func countEffectiveInterferencePairs(result SimulationResult) int {
	selected := map[string]bool{}
	if result.Experimental != nil {
		for _, id := range result.Experimental.InterferenceActivityIDs {
			selected[id] = true
		}
	}
	_, count := analyzeAssignmentOverlaps(result.Assignments, selected)
	return count
}

func writeExperimentRawCSV(path string, records []ExperimentRecord) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	header := []string{
		"algorithm", "scenario_id", "interference_seed", "interference_activity_ids", "interference_rate",
		"makespan", "budget_used", "budget_limit", "deadline_limit", "budget_violation", "deadline_violation",
		"feasible", "interference_time", "interference_pairs", "algorithm_milliseconds",
		"machine_distribution", "machine_utilization", "recommendations_json",
	}
	if err := writer.Write(header); err != nil {
		return err
	}
	for _, record := range records {
		distribution, _ := json.Marshal(record.MachineDistribution)
		utilization, _ := json.Marshal(record.MachineUtilization)
		recommendations, _ := json.Marshal(record.Recommendations)
		row := []string{
			record.Algorithm, record.ScenarioID, strconv.FormatInt(record.InterferenceSeed, 10),
			strings.Join(record.InterferenceActivities, "|"), formatFloat(record.InterferenceRate),
			formatFloat(record.Makespan), formatFloat(record.BudgetUsed), formatFloat(record.BudgetLimit),
			formatFloat(record.DeadlineLimit), formatFloat(record.BudgetViolation), formatFloat(record.DeadlineViolation),
			strconv.FormatBool(record.Feasible), formatFloat(record.InterferenceTime),
			strconv.Itoa(record.InterferencePairs), formatFloat(record.AlgorithmMilliseconds),
			string(distribution), string(utilization), string(recommendations),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	return writer.Error()
}

func writeExperimentSummaryCSV(path string, records []ExperimentRecord) error {
	type groupKey struct{ Scenario, Algorithm string }
	groups := map[groupKey][]ExperimentRecord{}
	baselineAlgorithm := "heft_classic"
	for _, record := range records {
		key := groupKey{record.ScenarioID, record.Algorithm}
		groups[key] = append(groups[key], record)
		if record.Algorithm == "heft_colocation" {
			baselineAlgorithm = record.Algorithm
		}
	}
	experimentAlgorithms := []string{"prism_cc_time", "prism_cc_cost", baselineAlgorithm}
	scenarioSet := map[string]bool{}
	for _, record := range records {
		scenarioSet[record.ScenarioID] = true
	}
	scenarios := []string{}
	for _, scenario := range experimentSupportedScenarioIDs {
		if scenarioSet[scenario] {
			scenarios = append(scenarios, scenario)
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{
		"scenario_id", "algorithm", "runs", "feasible_ratio",
		"budget_limit", "deadline_limit",
		"makespan_mean", "makespan_median", "makespan_stddev", "makespan_ci95",
		"budget_mean", "budget_median", "budget_stddev", "budget_ci95",
		"interference_time_mean", "algorithm_milliseconds_mean",
		"makespan_gain_vs_heft_percent", "budget_gain_vs_heft_percent",
	}); err != nil {
		return err
	}
	for _, scenario := range scenarios {
		for _, algorithm := range experimentAlgorithms {
			items := groups[groupKey{scenario, algorithm}]
			makespans, budgets, interference, elapsed := []float64{}, []float64{}, []float64{}, []float64{}
			feasible := 0
			for _, item := range items {
				makespans = append(makespans, item.Makespan)
				budgets = append(budgets, item.BudgetUsed)
				interference = append(interference, item.InterferenceTime)
				elapsed = append(elapsed, item.AlgorithmMilliseconds)
				if item.Feasible {
					feasible++
				}
			}
			mMean, mMedian, mStd, mCI := descriptiveStats(makespans)
			bMean, bMedian, bStd, bCI := descriptiveStats(budgets)
			makespanGain, budgetGain := 0.0, 0.0
			if algorithm != baselineAlgorithm {
				heftMakespan, _, _, _ := descriptiveStats(valuesFor(groups[groupKey{scenario, baselineAlgorithm}], func(r ExperimentRecord) float64 { return r.Makespan }))
				heftBudget, _, _, _ := descriptiveStats(valuesFor(groups[groupKey{scenario, baselineAlgorithm}], func(r ExperimentRecord) float64 { return r.BudgetUsed }))
				makespanGain = 100 * (heftMakespan - mMean) / maxf(heftMakespan, 0.001)
				budgetGain = 100 * (heftBudget - bMean) / maxf(heftBudget, 0.001)
			}
			row := []string{
				scenario, algorithm, strconv.Itoa(len(items)), formatFloat(float64(feasible) / float64(max(1, len(items)))),
				formatFloat(items[0].BudgetLimit), formatFloat(items[0].DeadlineLimit),
				formatFloat(mMean), formatFloat(mMedian), formatFloat(mStd), formatFloat(mCI),
				formatFloat(bMean), formatFloat(bMedian), formatFloat(bStd), formatFloat(bCI),
				formatFloat(mean(interference)), formatFloat(mean(elapsed)),
				formatFloat(makespanGain), formatFloat(budgetGain),
			}
			if err := writer.Write(row); err != nil {
				return err
			}
		}
	}
	return writer.Error()
}

func valuesFor(records []ExperimentRecord, value func(ExperimentRecord) float64) []float64 {
	out := make([]float64, 0, len(records))
	for _, record := range records {
		out = append(out, value(record))
	}
	return out
}

func descriptiveStats(values []float64) (float64, float64, float64, float64) {
	if len(values) == 0 {
		return 0, 0, 0, 0
	}
	sorted := append([]float64{}, values...)
	sort.Float64s(sorted)
	average := mean(sorted)
	median := sorted[len(sorted)/2]
	if len(sorted)%2 == 0 {
		median = (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2
	}
	variance := 0.0
	for _, value := range sorted {
		variance += (value - average) * (value - average)
	}
	stddev := 0.0
	if len(sorted) > 1 {
		stddev = math.Sqrt(variance / float64(len(sorted)-1))
	}
	return average, median, stddev, 1.96 * stddev / math.Sqrt(float64(len(sorted)))
}

func mean(values []float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(max(1, len(values)))
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}

func writeJSONFile(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeExperimentSchedule(
	outputDirectory, scenarioID string, seed int64, algorithm string, result SimulationResult,
) error {
	directory := filepath.Join(
		outputDirectory, "schedules", scenarioID, fmt.Sprintf("seed-%d", seed),
	)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(directory, algorithm+".json"), result)
}

func disableContainerOverhead(generated *GeneratedSimulation) {
	for taskID, byResource := range generated.Matrices.ContainerOverhead {
		for resourceID := range byResource {
			generated.Matrices.ContainerOverhead[taskID][resourceID] = 0
		}
	}
}
