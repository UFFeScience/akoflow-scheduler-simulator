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
	machineSimulatorsCSV = "machine_simulators.csv"
	montageRuntimesCSV   = "montage_c3d_standard_16_runtimes.csv"
)

type ExperimentScenario struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Homogeneity  string `json:"homogeneity"`
	MachineCount int    `json:"machine_count"`
}

type experimentMachineRow struct {
	ScenarioID  string
	Homogeneity string
	MachineID   string
	Kind        string
	Provider    string
	MachineType string
	Cores       int
	MemoryGB    float64
	Bandwidth   float64
	Location    string
	Speedup     float64
}

type montageRuntimeRow struct {
	ActivityID string
	Stage      string
	ET0Seconds float64
}

func experimentScenarioResources(scenarioID string) ([]ResourceSpec, error) {
	rows, err := readExperimentMachines()
	if err != nil {
		return nil, err
	}
	specs := []ResourceSpec{}
	for _, row := range rows {
		if row.ScenarioID != scenarioID {
			continue
		}
		bandwidth := row.Bandwidth * 1000
		if bandwidth <= 0 {
			if row.Kind == "cluster" {
				bandwidth = 10000
			} else {
				bandwidth = 2500
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
		})
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("unknown experiment scenario: %s", scenarioID)
	}
	return specs, nil
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
		stage := normalizeStage(task.WorkflowStage)
		values := runtimesByStage[stage]
		if len(values) == 0 {
			continue
		}
		index := stageCursor[stage] % len(values)
		task.BaseRuntime = round(values[index], 3)
		stageCursor[stage]++
	}
	return nil
}

func readExperimentMachines() ([]experimentMachineRow, error) {
	records, err := readExperimentCSV(machineSimulatorsCSV)
	if err != nil {
		return nil, err
	}
	rows := []experimentMachineRow{}
	for i, record := range records {
		if i == 0 {
			continue
		}
		cores, err := strconv.Atoi(record[8])
		if err != nil {
			return nil, fmt.Errorf("invalid physical_cores on row %d: %w", i+1, err)
		}
		memory, err := strconv.ParseFloat(record[10], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid memory_gb on row %d: %w", i+1, err)
		}
		bandwidth, _ := strconv.ParseFloat(record[11], 64)
		speedup, _ := strconv.ParseFloat(record[15], 64)
		rows = append(rows, experimentMachineRow{
			ScenarioID: record[0], Homogeneity: record[1], MachineID: record[2], Kind: record[3],
			Provider: record[4], MachineType: record[5], Cores: cores, MemoryGB: memory,
			Bandwidth: bandwidth, Location: record[12], Speedup: speedup,
		})
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
