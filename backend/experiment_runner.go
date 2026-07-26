package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
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
	OutputDirectory string
	Repetitions     int
	StructuralSeed  int64
	BeamWidth       int
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
}

type ExperimentManifest struct {
	GeneratedAt        string   `json:"generated_at"`
	StructuralSeed     int64    `json:"structural_seed"`
	InterferenceSeeds  []int64  `json:"interference_seeds"`
	Scenarios          []string `json:"scenarios"`
	Algorithms         []string `json:"algorithms"`
	InterferenceRate   float64  `json:"interference_rate"`
	SelectedActivities int      `json:"selected_activities"`
	BudgetLimit        float64  `json:"budget_limit"`
	DeadlineLimit      float64  `json:"deadline_limit"`
	ReferencePolicy    string   `json:"reference_policy"`
	Calibration        string   `json:"calibration"`
	HEFTMode           string   `json:"heft_mode"`
	PRISMCCPriority    string   `json:"prism_cc_priority"`
	BeamWidth          int      `json:"beam_width"`
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
		interferenceSeed := int64(repetition)
		seeds = append(seeds, interferenceSeed)
		for _, scenarioID := range experimentScenarioIDs {
			heftGenerated, generationErr := generateExperimentSimulation(scenarioID, options.StructuralSeed, interferenceSeed, false, options.BeamWidth)
			if generationErr != nil {
				return fmt.Errorf("%s seed %d HEFT generation: %w", scenarioID, interferenceSeed, generationErr)
			}
			heftGenerated.Experimental.Algorithm = "heft_classic"
			runStarted := time.Now()
			heftResult, scheduleErr := scheduleHEFTClassic(heftGenerated)
			if scheduleErr != nil {
				return fmt.Errorf("%s/heft seed %d: %w", scenarioID, interferenceSeed, scheduleErr)
			}
			heftRuns = append(heftRuns, calibratedHEFTRun{
				scenarioID: scenarioID, interferenceSeed: interferenceSeed, result: heftResult,
				elapsed: float64(time.Since(runStarted).Microseconds()) / 1000,
			})
			heftMakespans = append(heftMakespans, heftResult.TimingVariables.Makespan)
			heftBudgets = append(heftBudgets, heftResult.CostVariables.BUsed)
		}
	}

	deadlineLimit := mean(heftMakespans)
	budgetLimit := mean(heftBudgets)
	for _, heftRun := range heftRuns {
		records = append(records, experimentRecordFromResult(
			heftRun.result, "heft_classic", heftRun.scenarioID, heftRun.interferenceSeed,
			budgetLimit, deadlineLimit, heftRun.elapsed,
		))
	}

	var err error
	for repetition := 1; repetition <= options.Repetitions; repetition++ {
		interferenceSeed := int64(repetition)
		for _, scenarioID := range experimentScenarioIDs {
			for _, algorithm := range []string{"prism_cc_time", "prism_cc_cost"} {
				generated, generationErr := generateExperimentSimulation(scenarioID, options.StructuralSeed, interferenceSeed, false, options.BeamWidth)
				if generationErr != nil {
					return fmt.Errorf("%s seed %d generation: %w", scenarioID, interferenceSeed, generationErr)
				}
				generated.SLA.BudgetLimit = &budgetLimit
				generated.SLA.DeadlineLimit = &deadlineLimit
				generated.Experimental.Algorithm = algorithm
				switch algorithm {
				case "prism_cc_time":
					generated.SLA.WeightTime = 1
					generated.SLA.WeightCost = 0
				case "prism_cc_cost":
					generated.SLA.WeightTime = 0
					generated.SLA.WeightCost = 1
				}
				runStarted := time.Now()
				var response ScheduleOptimizationResponse
				response, err = optimizeSchedule(generated)
				if err == nil {
					if len(response.Options) == 0 {
						err = fmt.Errorf("beam returned no schedule")
					} else {
						result := response.Options[0].Result
						records = append(records, experimentRecordFromResult(
							result, algorithm, scenarioID, interferenceSeed, budgetLimit, deadlineLimit,
							float64(time.Since(runStarted).Microseconds())/1000,
						))
					}
				}
				if err != nil {
					return fmt.Errorf("%s/%s seed %d: %w", scenarioID, algorithm, interferenceSeed, err)
				}
			}
		}
	}

	if err := writeExperimentRawCSV(filepath.Join(options.OutputDirectory, "raw_results.csv"), records); err != nil {
		return err
	}
	if err := writeExperimentSummaryCSV(filepath.Join(options.OutputDirectory, "summary.csv"), records); err != nil {
		return err
	}
	manifest := ExperimentManifest{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339), StructuralSeed: options.StructuralSeed,
		InterferenceSeeds: seeds, Scenarios: experimentScenarioIDs, Algorithms: experimentAlgorithms,
		InterferenceRate: 0.20, SelectedActivities: 29,
		BudgetLimit: budgetLimit, DeadlineLimit: deadlineLimit,
		ReferencePolicy: "Global classic HEFT mean: deadline is the mean makespan and budget is the mean cost across all classic HEFT scenario/seed executions",
		Calibration:     "Global mean of all classic HEFT runs, without margin",
		HEFTMode:        "classic_no_colocation", PRISMCCPriority: "topological_order",
		BeamWidth: options.BeamWidth,
	}
	return writeJSONFile(filepath.Join(options.OutputDirectory, "manifest.json"), manifest)
}

func generateExperimentSimulation(scenarioID string, structuralSeed, interferenceSeed int64, disabled bool, beamWidth int) (GeneratedSimulation, error) {
	req := defaultRequest()
	req.Preset = "Montage"
	req.ExperimentScenarioID = scenarioID
	req.Seed = structuralSeed
	req.TaskCount = 58
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
	}
}

func countEffectiveInterferencePairs(result SimulationResult) int {
	selected := map[string]bool{}
	if result.Experimental != nil {
		for _, id := range result.Experimental.InterferenceActivityIDs {
			selected[id] = true
		}
	}
	count := 0
	for i, left := range result.Assignments {
		if !selected[left.TaskID] {
			continue
		}
		for _, right := range result.Assignments[i+1:] {
			if selected[right.TaskID] && left.ResourceID == right.ResourceID &&
				maxf(left.StartTime, right.StartTime) < minf(left.FinishTime, right.FinishTime) {
				count++
			}
		}
	}
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
		"machine_distribution", "machine_utilization",
	}
	if err := writer.Write(header); err != nil {
		return err
	}
	for _, record := range records {
		distribution, _ := json.Marshal(record.MachineDistribution)
		utilization, _ := json.Marshal(record.MachineUtilization)
		row := []string{
			record.Algorithm, record.ScenarioID, strconv.FormatInt(record.InterferenceSeed, 10),
			strings.Join(record.InterferenceActivities, "|"), "0.2",
			formatFloat(record.Makespan), formatFloat(record.BudgetUsed), formatFloat(record.BudgetLimit),
			formatFloat(record.DeadlineLimit), formatFloat(record.BudgetViolation), formatFloat(record.DeadlineViolation),
			strconv.FormatBool(record.Feasible), formatFloat(record.InterferenceTime),
			strconv.Itoa(record.InterferencePairs), formatFloat(record.AlgorithmMilliseconds),
			string(distribution), string(utilization),
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
