package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	machineSimulatorsCSV          = "machine_simulators.csv"
	edgeCloudMachinesCSV          = "machine_simulators_edge_cloud_extreme.csv"
	communicationMachinesCSV      = "machine_simulators_communication_dominant.csv"
	interferenceMachinesCSV       = "machine_simulators_interference_aware.csv"
	hybridRaspberryMachinesCSV    = "machine_simulators_hybrid_raspberry_500mbps.csv"
	communicationTrapMachinesCSV  = "machine_simulators_hybrid_communication_trap.csv"
	heftNetworkTrapMachinesCSV    = "machine_simulators_hybrid_heft_network_trap.csv"
	wfcommonsChameleonMachinesCSV = "machine_simulators_wfcommons_chameleon.csv"
	networkCriticalMachinesCSV    = "machine_simulators_network_critical.csv"
	montageRuntimesCSV            = "montage_c3d_standard_16_runtimes.csv"
	montageWorkflowYAML           = "wf-montage-050d-gcp.yaml"
	montageDSS20WorkflowID        = "montage_dss_20d"
	montageDSS20WorkflowYAML      = "wf-montage-chameleon-dss-20d-001.yaml"
	montageDSS20RuntimesCSV       = "montage_chameleon_dss_20d_001_runtimes.csv"
	montageDSS20DependenciesCSV   = "montage_chameleon_dss_20d_001_dependencies.csv"
	imageDataflow8WorkflowID      = "image_dataflow_8"
	imageDataflow8WorkflowYAML    = "wf-image-dataflow-8.yaml"
	megabitsPerGigabit            = 1000.0
	bitsPerByte                   = 8.0
	clusterBandwidthLimitMbps     = 500.0
)

type wfcommonsWorkflowDataset struct {
	YAMLFile         string
	RuntimesFile     string
	DependenciesFile string
	TaskCount        int
}

var wfcommonsWorkflowDatasets = map[string]wfcommonsWorkflowDataset{
	"wfcommons_1000genome_902": {
		"wf-wfcommons-1000genome-902.yaml", "wfcommons_1000genome_902_runtimes.csv", "wfcommons_1000genome_902_dependencies.csv", 902,
	},
	"wfcommons_cycles_6543": {
		"wf-wfcommons-cycles-6543.yaml", "wfcommons_cycles_6543_runtimes.csv", "wfcommons_cycles_6543_dependencies.csv", 6543,
	},
	"wfcommons_epigenomics_1695": {
		"wf-wfcommons-epigenomics-1695.yaml", "wfcommons_epigenomics_1695_runtimes.csv", "wfcommons_epigenomics_1695_dependencies.csv", 1695,
	},
	"wfcommons_seismology_1101": {
		"wf-wfcommons-seismology-1101.yaml", "wfcommons_seismology_1101_runtimes.csv", "wfcommons_seismology_1101_dependencies.csv", 1101,
	},
	"wfcommons_soykb_676": {
		"wf-wfcommons-soykb-676.yaml", "wfcommons_soykb_676_runtimes.csv", "wfcommons_soykb_676_dependencies.csv", 676,
	},
	"wfcommons_srasearch_104": {
		"wf-wfcommons-srasearch-104.yaml", "wfcommons_srasearch_104_runtimes.csv", "wfcommons_srasearch_104_dependencies.csv", 104,
	},
}

func experimentWorkflowSupported(workflowID string) bool {
	if workflowID == "montage_050d" || workflowID == montageDSS20WorkflowID || workflowID == imageDataflow8WorkflowID {
		return true
	}
	_, exists := wfcommonsWorkflowDatasets[workflowID]
	return exists
}

func experimentWorkflowTaskCount(workflowID string) int {
	if workflowID == montageDSS20WorkflowID {
		return 6448
	}
	if workflowID == imageDataflow8WorkflowID {
		return 8
	}
	if dataset, exists := wfcommonsWorkflowDatasets[workflowID]; exists {
		return dataset.TaskCount
	}
	return 58
}

func experimentWorkflowYAMLFile(workflowID string) string {
	if workflowID == montageDSS20WorkflowID {
		return montageDSS20WorkflowYAML
	}
	if workflowID == imageDataflow8WorkflowID {
		return imageDataflow8WorkflowYAML
	}
	if dataset, exists := wfcommonsWorkflowDatasets[workflowID]; exists {
		return dataset.YAMLFile
	}
	return montageWorkflowYAML
}

type ExperimentScenario struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Homogeneity  string `json:"homogeneity"`
	MachineCount int    `json:"machine_count"`
}

type experimentMachineRow struct {
	ScenarioID           string
	Homogeneity          string
	MachineID            string
	Kind                 string
	Provider             string
	MachineType          string
	Cores                int
	MemoryGB             float64
	Bandwidth            float64
	Location             string
	Speedup              float64
	PricePerHourUSD      float64
	NetworkPricePerGBUSD float64
	PricingModel         string
	NetworkLatencyMS     float64
}

type montageRuntimeRow struct {
	ActivityID string
	Stage      string
	ET0Seconds float64
}

type montageDSS20RuntimeRow struct {
	ActivityID string
	Stage      string
	ET0Seconds float64
}

type montageDSS20DependencyRow struct {
	Source    string
	Target    string
	DataMB    float64
	FileCount int
}

func experimentScenarioResources(scenarioID string) ([]ResourceSpec, error) {
	sourceScenarioID := realNetworkStressBaseScenario(scenarioID)
	rows, err := readExperimentMachines()
	if err != nil {
		return nil, err
	}
	specs := []ResourceSpec{}
	for _, row := range rows {
		if row.ScenarioID != sourceScenarioID {
			continue
		}
		bandwidth := gigabitsPerSecondToMegabytesPerSecond(row.Bandwidth)
		bandwidth500MbpsScenario := row.ScenarioID == "cluster_homo" ||
			row.ScenarioID == "cluster_hetero" ||
			row.ScenarioID == "cloud_homo" ||
			row.ScenarioID == "cloud_hetero" ||
			row.ScenarioID == "hybrid_homo" ||
			row.ScenarioID == "hybrid_hetero" ||
			row.ScenarioID == "hybrid_raspberry_500mbps"
		if bandwidth500MbpsScenario && !strings.HasPrefix(row.MachineID, "rpi-edge-") {
			bandwidth = megabitsPerSecondToMegabytesPerSecond(clusterBandwidthLimitMbps)
		}
		if bandwidth <= 0 {
			if row.Kind == "cluster" {
				bandwidth = gigabitsPerSecondToMegabytesPerSecond(10)
			} else {
				bandwidth = gigabitsPerSecondToMegabytesPerSecond(2.5)
			}
		}
		boot := 0.0
		if row.Kind == "cloud" {
			boot = 12
		}
		name := row.MachineType
		if row.Provider != "" {
			name = row.Provider + " " + row.MachineType
		}
		specs = append(specs, ResourceSpec{
			ID: row.MachineID, Name: name, Kind: row.Kind, Cores: row.Cores,
			Memory: row.MemoryGB, Bandwidth: bandwidth, BootOverhead: boot, Location: row.Location, Speedup: row.Speedup,
			PricePerHourUSD: row.PricePerHourUSD, NetworkPricePerGBUSD: row.NetworkPricePerGBUSD, PricingModel: row.PricingModel,
			NetworkLatencyMS: row.NetworkLatencyMS,
		})
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("unknown experiment scenario: %s", scenarioID)
	}
	return specs, nil
}

func realNetworkStressBaseScenario(scenarioID string) string {
	const prefix = "real_network_stress_"
	if strings.HasPrefix(scenarioID, prefix) {
		return strings.TrimPrefix(scenarioID, prefix)
	}
	return scenarioID
}

func isRealNetworkStressScenario(scenarioID string) bool {
	return strings.HasPrefix(scenarioID, "real_network_stress_")
}

func gigabitsPerSecondToMegabytesPerSecond(value float64) float64 {
	return value * megabitsPerGigabit / bitsPerByte
}

func megabitsPerSecondToMegabytesPerSecond(value float64) float64 {
	return value / bitsPerByte
}

func experimentScenarios() ([]ExperimentScenario, error) {
	rows, err := readExperimentMachines()
	if err != nil {
		return nil, err
	}
	byID := map[string]*ExperimentScenario{}
	order := []string{}
	for _, row := range rows {
		item := byID[row.ScenarioID]
		if item == nil {
			label := strings.ReplaceAll(row.ScenarioID, "_", " ")
			item = &ExperimentScenario{ID: row.ScenarioID, Label: strings.Title(label), Homogeneity: row.Homogeneity}
			byID[row.ScenarioID] = item
			order = append(order, row.ScenarioID)
		}
		item.MachineCount++
	}
	out := make([]ExperimentScenario, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}

func applyMontageExperimentRuntimes(workflow *Workflow) error {
	rows, err := readMontageRuntimes()
	if err != nil {
		return err
	}
	runtimeByActivity := map[string]montageRuntimeRow{}
	runtimesByStage := map[string][]float64{}
	for _, row := range rows {
		runtimeByActivity[row.ActivityID] = row
		stage := normalizeStage(row.Stage)
		runtimesByStage[stage] = append(runtimesByStage[stage], row.ET0Seconds)
	}
	stageCursor := map[string]int{}
	for i := range workflow.Tasks {
		task := &workflow.Tasks[i]
		if row, ok := runtimeByActivity[task.ID]; ok {
			task.BaseRuntime = round(row.ET0Seconds, 3)
			task.WorkflowStage = row.Stage
			task.Label = row.ActivityID
			continue
		}
		if len(workflow.Tasks) == len(rows) {
			return fmt.Errorf("runtime missing for Montage activity %s", task.ID)
		}
		stage := normalizeStage(task.WorkflowStage)
		values := runtimesByStage[stage]
		if len(values) == 0 {
			continue
		}
		index := stageCursor[stage] % len(values)
		task.BaseRuntime = round(values[index], 3)
		stageCursor[stage]++
	}
	if len(workflow.Tasks) == len(rows) {
		seen := map[string]bool{}
		for _, task := range workflow.Tasks {
			seen[task.ID] = true
		}
		for _, row := range rows {
			if !seen[row.ActivityID] {
				return fmt.Errorf("runtime activity %s is absent from the Montage workflow", row.ActivityID)
			}
		}
	}
	return nil
}

func applyMontageDSS20ExperimentData(workflow *Workflow) error {
	runtimes, err := readMontageDSS20Runtimes()
	if err != nil {
		return err
	}
	dependencies, err := readMontageDSS20Dependencies()
	if err != nil {
		return err
	}
	if len(workflow.Tasks) != len(runtimes) {
		return fmt.Errorf("Montage DSS 20d task/runtime mismatch: %d tasks, %d runtimes", len(workflow.Tasks), len(runtimes))
	}
	runtimeByID := make(map[string]montageDSS20RuntimeRow, len(runtimes))
	for _, row := range runtimes {
		runtimeByID[row.ActivityID] = row
	}
	for index := range workflow.Tasks {
		task := &workflow.Tasks[index]
		row, ok := runtimeByID[task.ID]
		if !ok {
			return fmt.Errorf("runtime missing for Montage DSS 20d activity %s", task.ID)
		}
		task.BaseRuntime = round(row.ET0Seconds, 6)
		task.WorkflowStage = row.Stage
		task.Label = row.ActivityID
	}
	dataByEdge := make(map[string]montageDSS20DependencyRow, len(dependencies))
	for _, row := range dependencies {
		dataByEdge[row.Source+"\x00"+row.Target] = row
	}
	if len(workflow.Dependencies) != len(dependencies) {
		return fmt.Errorf("Montage DSS 20d edge mismatch: %d YAML edges, %d data edges", len(workflow.Dependencies), len(dependencies))
	}
	for index := range workflow.Dependencies {
		dependency := &workflow.Dependencies[index]
		row, ok := dataByEdge[dependency.Source+"\x00"+dependency.Target]
		if !ok {
			return fmt.Errorf("data dependency missing for %s -> %s", dependency.Source, dependency.Target)
		}
		dependency.DataMB = round(row.DataMB, 9)
		dependency.FileCount = row.FileCount
	}
	return nil
}

func applyWfcommonsExperimentData(workflowID string, workflow *Workflow) error {
	dataset, exists := wfcommonsWorkflowDatasets[workflowID]
	if !exists {
		return fmt.Errorf("unknown WfCommons workflow %q", workflowID)
	}
	runtimes, err := readWfcommonsRuntimes(dataset.RuntimesFile)
	if err != nil {
		return err
	}
	dependencies, err := readWfcommonsDependencies(dataset.DependenciesFile)
	if err != nil {
		return err
	}
	if len(workflow.Tasks) != dataset.TaskCount || len(runtimes) != dataset.TaskCount {
		return fmt.Errorf("%s task/runtime mismatch: %d tasks, %d runtimes, expected %d", workflowID, len(workflow.Tasks), len(runtimes), dataset.TaskCount)
	}
	runtimeByID := make(map[string]montageDSS20RuntimeRow, len(runtimes))
	for _, row := range runtimes {
		runtimeByID[row.ActivityID] = row
	}
	for index := range workflow.Tasks {
		task := &workflow.Tasks[index]
		row, ok := runtimeByID[task.ID]
		if !ok {
			return fmt.Errorf("runtime missing for %s activity %s", workflowID, task.ID)
		}
		task.BaseRuntime = round(row.ET0Seconds, 6)
		task.WorkflowStage = row.Stage
		task.Label = row.ActivityID
	}
	dataByEdge := make(map[string]montageDSS20DependencyRow, len(dependencies))
	for _, row := range dependencies {
		dataByEdge[row.Source+"\x00"+row.Target] = row
	}
	if len(workflow.Dependencies) != len(dependencies) {
		return fmt.Errorf("%s edge mismatch: %d YAML edges, %d data edges", workflowID, len(workflow.Dependencies), len(dependencies))
	}
	for index := range workflow.Dependencies {
		dependency := &workflow.Dependencies[index]
		row, ok := dataByEdge[dependency.Source+"\x00"+dependency.Target]
		if !ok {
			return fmt.Errorf("data dependency missing for %s edge %s -> %s", workflowID, dependency.Source, dependency.Target)
		}
		dependency.DataMB = round(row.DataMB, 9)
		dependency.FileCount = row.FileCount
	}
	return nil
}

func applyImageDataflow8ExperimentData(workflow *Workflow) error {
	runtimes := map[string]float64{
		"t0": 60, "t1": 45, "t2": 90, "t3": 70,
		"t4": 50, "t5": 80, "t6": 120, "t7": 65,
	}
	dataByEdge := map[string]float64{
		"t0\x00t1": 10000, // d1: 10 GB
		"t1\x00t2": 10000, // d2: 10 GB
		"t2\x00t6": 40000, // d3 + d5 + d9 + d10: 4 x 10 GB
		"t3\x00t4": 10000, // d7: 10 GB
		"t4\x00t6": 10000, // d8: 10 GB
		"t5\x00t7": 20000, // d11 + d12: 2 x 10 GB
		"t7\x00t6": 10000, // d13: 10 GB
	}
	if len(workflow.Tasks) != len(runtimes) || len(workflow.Dependencies) != len(dataByEdge) {
		return fmt.Errorf(
			"image dataflow topology mismatch: got %d tasks and %d dependencies",
			len(workflow.Tasks), len(workflow.Dependencies),
		)
	}
	for index := range workflow.Tasks {
		task := &workflow.Tasks[index]
		runtime, ok := runtimes[task.ID]
		if !ok {
			return fmt.Errorf("image dataflow runtime missing for %s", task.ID)
		}
		task.BaseRuntime = runtime
		task.WorkflowStage = "dataflow"
	}
	for index := range workflow.Dependencies {
		dependency := &workflow.Dependencies[index]
		dataMB, ok := dataByEdge[dependency.Source+"\x00"+dependency.Target]
		if !ok {
			return fmt.Errorf("image dataflow dependency missing for %s -> %s", dependency.Source, dependency.Target)
		}
		dependency.DataMB = dataMB
	}
	return nil
}

func readExperimentText(name string) (string, error) {
	for _, base := range []string{"/experiments", "experiments", "../experiments"} {
		path := filepath.Join(base, name)
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data), nil
		}
	}
	return "", fmt.Errorf("experiment file not found: %s", name)
}

func readExperimentMachines() ([]experimentMachineRow, error) {
	rows := []experimentMachineRow{}
	for _, filename := range []string{
		machineSimulatorsCSV,
		edgeCloudMachinesCSV,
		communicationMachinesCSV,
		interferenceMachinesCSV,
		hybridRaspberryMachinesCSV,
		communicationTrapMachinesCSV,
		heftNetworkTrapMachinesCSV,
		wfcommonsChameleonMachinesCSV,
		networkCriticalMachinesCSV,
	} {
		records, err := readExperimentCSV(filename)
		if err != nil {
			return nil, err
		}
		for i, record := range records[1:] {
			cores, err := strconv.Atoi(record[8])
			if err != nil {
				return nil, fmt.Errorf("%s: invalid physical_cores on row %d: %w", filename, i+2, err)
			}
			memory, err := strconv.ParseFloat(record[10], 64)
			if err != nil {
				return nil, fmt.Errorf("%s: invalid memory_gb on row %d: %w", filename, i+2, err)
			}
			bandwidth, _ := strconv.ParseFloat(record[11], 64)
			speedup, err := strconv.ParseFloat(record[14], 64)
			if err != nil {
				return nil, fmt.Errorf("%s: invalid speedup on row %d: %w", filename, i+2, err)
			}
			pricePerHour, err := strconv.ParseFloat(record[16], 64)
			if err != nil {
				return nil, fmt.Errorf("%s: invalid price_per_hour_usd on row %d: %w", filename, i+2, err)
			}
			networkPricePerGB, err := strconv.ParseFloat(record[17], 64)
			if err != nil {
				return nil, fmt.Errorf("%s: invalid network_price_per_gb_usd on row %d: %w", filename, i+2, err)
			}
			networkLatencyMS := 0.0
			if len(record) > 20 && strings.TrimSpace(record[20]) != "" {
				networkLatencyMS, err = strconv.ParseFloat(record[20], 64)
				if err != nil {
					return nil, fmt.Errorf("%s: invalid network_latency_ms on row %d: %w", filename, i+2, err)
				}
			}
			rows = append(rows, experimentMachineRow{
				ScenarioID: record[0], Homogeneity: record[1], MachineID: record[2], Kind: record[3],
				Provider: record[4], MachineType: record[5], Cores: cores, MemoryGB: memory,
				Bandwidth: bandwidth, Location: record[12], Speedup: speedup,
				PricePerHourUSD: pricePerHour, NetworkPricePerGBUSD: networkPricePerGB, PricingModel: record[18],
				NetworkLatencyMS: networkLatencyMS,
			})
		}
	}
	return rows, nil
}

func readMontageRuntimes() ([]montageRuntimeRow, error) {
	records, err := readExperimentCSV(montageRuntimesCSV)
	if err != nil {
		return nil, err
	}
	rows := []montageRuntimeRow{}
	for i, record := range records {
		if i == 0 {
			continue
		}
		et0, err := strconv.ParseFloat(record[7], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid et0_seconds on row %d: %w", i+1, err)
		}
		rows = append(rows, montageRuntimeRow{ActivityID: record[0], Stage: record[1], ET0Seconds: et0})
	}
	return rows, nil
}

func readMontageDSS20Runtimes() ([]montageDSS20RuntimeRow, error) {
	records, err := readExperimentCSV(montageDSS20RuntimesCSV)
	if err != nil {
		return nil, err
	}
	rows := make([]montageDSS20RuntimeRow, 0, len(records)-1)
	for i, record := range records {
		if i == 0 {
			continue
		}
		et0, err := strconv.ParseFloat(record[9], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid DSS 20d et0_c3d_seconds on row %d: %w", i+1, err)
		}
		rows = append(rows, montageDSS20RuntimeRow{ActivityID: record[0], Stage: record[1], ET0Seconds: et0})
	}
	return rows, nil
}

func readMontageDSS20Dependencies() ([]montageDSS20DependencyRow, error) {
	records, err := readExperimentCSV(montageDSS20DependenciesCSV)
	if err != nil {
		return nil, err
	}
	rows := make([]montageDSS20DependencyRow, 0, len(records)-1)
	for i, record := range records {
		if i == 0 {
			continue
		}
		dataMB, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid DSS 20d data_mb on row %d: %w", i+1, err)
		}
		rows = append(rows, montageDSS20DependencyRow{
			Source: record[0], Target: record[1], DataMB: dataMB,
			FileCount: dependencyCSVFileCount(record),
		})
	}
	return rows, nil
}

func readWfcommonsRuntimes(filename string) ([]montageDSS20RuntimeRow, error) {
	records, err := readExperimentCSV(filename)
	if err != nil {
		return nil, err
	}
	rows := make([]montageDSS20RuntimeRow, 0, len(records)-1)
	for index, record := range records[1:] {
		if len(record) < 10 {
			return nil, fmt.Errorf("%s: invalid runtime row %d", filename, index+2)
		}
		et0, err := strconv.ParseFloat(record[9], 64)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid et0_c3d_seconds on row %d: %w", filename, index+2, err)
		}
		rows = append(rows, montageDSS20RuntimeRow{ActivityID: record[0], Stage: record[1], ET0Seconds: et0})
	}
	return rows, nil
}

func readWfcommonsDependencies(filename string) ([]montageDSS20DependencyRow, error) {
	records, err := readExperimentCSV(filename)
	if err != nil {
		return nil, err
	}
	rows := make([]montageDSS20DependencyRow, 0, len(records)-1)
	for index, record := range records[1:] {
		if len(record) < 3 {
			return nil, fmt.Errorf("%s: invalid dependency row %d", filename, index+2)
		}
		dataMB, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid data_mb on row %d: %w", filename, index+2, err)
		}
		rows = append(rows, montageDSS20DependencyRow{
			Source: record[0], Target: record[1], DataMB: dataMB,
			FileCount: dependencyCSVFileCount(record),
		})
	}
	return rows, nil
}

func dependencyCSVFileCount(record []string) int {
	if len(record) < 5 || strings.TrimSpace(record[4]) == "" {
		return 1
	}
	count := 0
	for _, filename := range strings.Split(record[4], "|") {
		if strings.TrimSpace(filename) != "" {
			count++
		}
	}
	return max(1, count)
}

func readExperimentCSV(name string) ([][]string, error) {
	for _, base := range []string{"/experiments", "experiments", "../experiments"} {
		path := filepath.Join(base, name)
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		defer file.Close()
		return csv.NewReader(file).ReadAll()
	}
	return nil, fmt.Errorf("experiment file not found: %s", name)
}

func normalizeStage(stage string) string {
	return strings.ToLower(strings.TrimSpace(stage))
}
