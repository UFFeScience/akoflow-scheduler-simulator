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

var experimentAlgorithms = []string{"prism_cc_time", "prism_cc_cost", "heft_classic"}

const (
	c3dStandard16ReferencePricePerHourUSD = 0.726384
	c3dStandard16ReferenceMakespanSeconds = 2815.0
	c3dStandard16ReferenceBudgetUSD       = c3dStandard16ReferencePricePerHourUSD * c3dStandard16ReferenceMakespanSeconds / 3600
)

type ExperimentRunOptions struct {
	OutputDirectory     string
	Repetitions         int
	StructuralSeed      int64
	BeamWidth           int
	RecommendationCount int
	Workers             int
	PRISMCCPriority     string
	WorkflowID          string
}

type ExperimentRecord struct {
	Algorithm              string
	ScenarioID             string
	InterferenceSeed       int64
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
	GeneratedAt         string   `json:"generated_at"`
	StructuralSeed      int64    `json:"structural_seed"`
	InterferenceSeeds   []int64  `json:"interference_seeds"`
	Scenarios           []string `json:"scenarios"`
	Algorithms          []string `json:"algorithms"`
	WorkflowID          string   `json:"workflow_id"`
	TaskCount           int      `json:"task_count"`
	InterferenceRate    float64  `json:"interference_rate"`
	SelectedActivities  int      `json:"selected_activities"`
	BudgetLimit         float64  `json:"budget_limit"`
	DeadlineLimit       float64  `json:"deadline_limit"`
	ReferencePolicy     string   `json:"reference_policy"`
	Calibration         string   `json:"calibration"`
	HEFTMode            string   `json:"heft_mode"`
	PRISMCCPriority     string   `json:"prism_cc_priority"`
	BeamWidth           int      `json:"beam_width"`
	RecommendationCount int      `json:"recommendation_count"`
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
	if options.WorkflowID != "montage_050d" && options.WorkflowID != montageDSS20WorkflowID {
		return fmt.Errorf("unsupported experiment workflow %q", options.WorkflowID)
	}
	if options.PRISMCCPriority != "topological_order" && options.PRISMCCPriority != "upward_rank" {
		return fmt.Errorf("unsupported PRISM-CC priority policy %q", options.PRISMCCPriority)
	}
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
		result           SimulationResult
		elapsed          float64
	}
	heftRuns := make([]calibratedHEFTRun, 0, len(experimentScenarioIDs)*options.Repetitions)
	heftMakespans := make([]float64, 0, cap(heftRuns))
	heftBudgets := make([]float64, 0, cap(heftRuns))
	records := make([]ExperimentRecord, 0, len(experimentScenarioIDs)*len(experimentAlgorithms)*options.Repetitions)
	seeds := make([]int64, 0, options.Repetitions)
	for repetition := 1; repetition <= options.Repetitions; repetition++ {
		seeds = append(seeds, int64(repetition))
	}
	for _, scenarioID := range experimentScenarioIDs {
		var referenceResult SimulationResult
		referenceElapsed := 0.0
		for repetition := 1; repetition <= options.Repetitions; repetition++ {
			interferenceSeed := int64(repetition)
			heftGenerated, generationErr := generateExperimentSimulationForWorkflow(
				scenarioID, options.WorkflowID, options.StructuralSeed, interferenceSeed, false, options.BeamWidth,
			)
			if generationErr != nil {
				return fmt.Errorf("%s seed %d HEFT generation: %w", scenarioID, interferenceSeed, generationErr)
			}
			var heftResult SimulationResult
			if repetition == 1 {
				heftGenerated.Experimental.Algorithm = "heft_classic"
				runStarted := time.Now()
				var scheduleErr error
				referenceResult, scheduleErr = scheduleHEFTClassic(heftGenerated)
				if scheduleErr != nil {
					return fmt.Errorf("%s/heft seed %d: %w", scenarioID, interferenceSeed, scheduleErr)
				}
				referenceElapsed = float64(time.Since(runStarted).Microseconds()) / 1000
			}
			heftResult = referenceResult
			heftResult.Experimental = heftGenerated.Experimental
			heftRuns = append(heftRuns, calibratedHEFTRun{
				scenarioID: scenarioID, interferenceSeed: interferenceSeed, result: heftResult,
				elapsed: referenceElapsed,
			})
			heftMakespans = append(heftMakespans, heftResult.TimingVariables.Makespan)
			heftBudgets = append(heftBudgets, heftResult.CostVariables.BUsed)
		}
	}

	deadlineLimit := mean(heftMakespans)
	budgetLimit := mean(heftBudgets)
	taskCount := 0
	if len(heftRuns) > 0 {
		taskCount = len(heftRuns[0].result.Workflow.Tasks)
	}
	for _, heftRun := range heftRuns {
		records = append(records, experimentRecordFromResult(
			heftRun.result, "heft_classic", heftRun.scenarioID, heftRun.interferenceSeed,
			budgetLimit, deadlineLimit, heftRun.elapsed,
		))
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
	jobCount := options.Repetitions * len(experimentScenarioIDs)
	workerCount := options.Workers
	if workerCount <= 0 {
		workerCount = min(4, runtime.GOMAXPROCS(0))
	}
	workerCount = min(workerCount, runtime.GOMAXPROCS(0), jobCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			for job := range jobs {
				started := time.Now()
				jobRecords, jobErr := runPRISMExperimentJob(
					options, job.scenarioID, job.seed, budgetLimit, deadlineLimit,
				)
				jobResults <- prismJobResult{
					scenarioID: job.scenarioID, seed: job.seed, elapsed: time.Since(started),
					records: jobRecords, err: jobErr,
				}
			}
		}()
	}
	protocolStarted := time.Now()
	log.Printf(
		"PRISM-CC experiment started: %d environment/seed combinations, %d workers, %d result rows expected",
		jobCount, workerCount, jobCount*2,
	)
	go func() {
		for repetition := 1; repetition <= options.Repetitions; repetition++ {
			for _, scenarioID := range experimentScenarioIDs {
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
	manifest := ExperimentManifest{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339), StructuralSeed: options.StructuralSeed,
		InterferenceSeeds: seeds, Scenarios: experimentScenarioIDs, Algorithms: experimentAlgorithms,
		WorkflowID: options.WorkflowID, TaskCount: taskCount,
		InterferenceRate: 0.20, SelectedActivities: taskCount / 2,
		BudgetLimit: budgetLimit, DeadlineLimit: deadlineLimit,
		ReferencePolicy: "Global classic HEFT mean: deadline is the mean makespan and budget is the mean cost across all classic HEFT scenario/seed executions",
		Calibration:     "Global mean of all classic HEFT runs, without margin",
		HEFTMode:        "classic_no_colocation", PRISMCCPriority: options.PRISMCCPriority,
		BeamWidth: options.BeamWidth, RecommendationCount: options.RecommendationCount,
	}
	return writeJSONFile(filepath.Join(options.OutputDirectory, "manifest.json"), manifest)
}

func runPRISMExperimentJob(options ExperimentRunOptions, scenarioID string, interferenceSeed int64, budgetLimit, deadlineLimit float64) ([]ExperimentRecord, error) {
	generated, generationErr := generateExperimentSimulationForWorkflow(
		scenarioID, options.WorkflowID, options.StructuralSeed, interferenceSeed, false, options.BeamWidth,
	)
	if generationErr != nil {
		return nil, fmt.Errorf("%s seed %d generation: %w", scenarioID, interferenceSeed, generationErr)
	}
	generated.SLA.BudgetLimit = &budgetLimit
	generated.SLA.DeadlineLimit = &deadlineLimit
	generated.Experimental.PriorityPolicy = options.PRISMCCPriority
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
		record := experimentRecordFromResult(
			response[0].Result, algorithm, scenarioID, interferenceSeed, budgetLimit, deadlineLimit,
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
	req := defaultRequest()
	req.Preset = "Montage"
	req.ExperimentScenarioID = scenarioID
	req.ExperimentWorkflowID = workflowID
	req.Seed = structuralSeed
	req.TaskCount = 58
	if workflowID == montageDSS20WorkflowID {
		req.TaskCount = 6448
	}
	req.OptionCount = 1
	req.BeamWidth = beamWidth
	generated, err := generateSimulation(req)
	if err != nil {
		return GeneratedSimulation{}, err
	}
	applyControlledInterference(&generated, interferenceSeed, 0.20, disabled)
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
	for _, scenarioID := range experimentScenarioIDs {
		if counts[scenarioID] != 4 {
			return fmt.Errorf("scenario %s must define exactly 4 machines, got %d", scenarioID, counts[scenarioID])
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
		Makespan: result.TimingVariables.Makespan, BudgetUsed: result.CostVariables.BUsed,
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
			strings.Join(record.InterferenceActivities, "|"), "0.2",
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
	for _, record := range records {
		key := groupKey{record.ScenarioID, record.Algorithm}
		groups[key] = append(groups[key], record)
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
		"makespan_mean", "makespan_median", "makespan_stddev", "makespan_ci95",
		"budget_mean", "budget_median", "budget_stddev", "budget_ci95",
		"interference_time_mean", "algorithm_milliseconds_mean",
		"makespan_gain_vs_heft_percent", "budget_gain_vs_heft_percent",
	}); err != nil {
		return err
	}
	for _, scenario := range experimentScenarioIDs {
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
			if algorithm != "heft_classic" {
				heftMakespan, _, _, _ := descriptiveStats(valuesFor(groups[groupKey{scenario, "heft_classic"}], func(r ExperimentRecord) float64 { return r.Makespan }))
				heftBudget, _, _, _ := descriptiveStats(valuesFor(groups[groupKey{scenario, "heft_classic"}], func(r ExperimentRecord) float64 { return r.BudgetUsed }))
				makespanGain = 100 * (heftMakespan - mMean) / maxf(heftMakespan, 0.001)
				budgetGain = 100 * (heftBudget - bMean) / maxf(heftBudget, 0.001)
			}
			row := []string{
				scenario, algorithm, strconv.Itoa(len(items)), formatFloat(float64(feasible) / float64(max(1, len(items)))),
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
