package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

var presets = []map[string]any{
	{"id": "Montage", "label": "Montage", "stages": []string{"mProject", "mDiffFit", "mConcatFit", "mBgModel", "mAdd"}},
	{"id": "CyberShake", "label": "CyberShake", "stages": []string{"ExtractSGT", "Seismogram", "PeakValCalc", "ZipPSA"}},
	{"id": "Epigenomics", "label": "Epigenomics", "stages": []string{"FastQSplit", "FilterContams", "Sol2Sanger", "Map", "Merge"}},
	{"id": "Random", "label": "Random DAG", "stages": []string{"ingest", "transform", "analyze", "reduce", "publish"}},
}

const (
	defaultBeamWidth   = 2000
	minBeamWidth       = 120
	maxBeamWidth       = 10000
	maxScheduleOptions = 1000
)

func round(v float64, places int) float64 {
	p := math.Pow10(places)
	return math.Round(v*p) / p
}

func maxf(values ...float64) float64 {
	out := values[0]
	for _, v := range values[1:] {
		if v > out {
			out = v
		}
	}
	return out
}

func minf(values ...float64) float64 {
	out := values[0]
	for _, v := range values[1:] {
		if v < out {
			out = v
		}
	}
	return out
}

func defaultRequest() SimulationRequest {
	return SimulationRequest{
		Preset: "Montage", Seed: 42, TaskCount: 12, EdgeDensity: 0.22,
		ClusterMachines: 3, CloudMachines: 2, CoresPerMachine: 2,
		WeightTime: 0.60, WeightCost: 0.40, OptionCount: 5, BeamWidth: defaultBeamWidth,
	}
}

func validateRequest(r SimulationRequest) error {
	if r.Preset == "" {
		return errors.New("preset is required")
	}
	if r.TaskCount < 3 || r.TaskCount > 100 {
		return errors.New("task_count must be between 3 and 100")
	}
	if r.EdgeDensity < 0 || r.EdgeDensity > 0.8 {
		return errors.New("edge_density must be between 0 and 0.8")
	}
	if r.ClusterMachines < 1 || r.ClusterMachines > 20 {
		return errors.New("cluster_machines must be between 1 and 20")
	}
	if r.CloudMachines < 0 || r.CloudMachines > 20 {
		return errors.New("cloud_machines must be between 0 and 20")
	}
	if r.CoresPerMachine < 1 || r.CoresPerMachine > 16 {
		return errors.New("cores_per_machine must be between 1 and 16")
	}
	if r.WeightTime < 0 || r.WeightTime > 1 || r.WeightCost < 0 || r.WeightCost > 1 {
		return errors.New("weights must be between 0 and 1")
	}
	if r.BudgetLimit != nil && *r.BudgetLimit <= 0 {
		return errors.New("budget_limit must be greater than 0")
	}
	if r.DeadlineLimit != nil && *r.DeadlineLimit <= 0 {
		return errors.New("deadline_limit must be greater than 0")
	}
	if r.OptionCount < 1 || r.OptionCount > maxScheduleOptions {
		return fmt.Errorf("option_count must be between 1 and %d", maxScheduleOptions)
	}
	if r.BeamWidth < minBeamWidth || r.BeamWidth > maxBeamWidth {
		return fmt.Errorf("beam_width must be between %d and %d", minBeamWidth, maxBeamWidth)
	}
	for _, spec := range r.ResourceSpecs {
		if spec.ID == "" || spec.Name == "" {
			return errors.New("resource_specs require id and name")
		}
		if spec.Kind != "cluster" && spec.Kind != "cloud" {
			return fmt.Errorf("invalid resource kind: %s", spec.Kind)
		}
		if spec.Cores < 1 || spec.Cores > 64 {
			return errors.New("resource cores must be between 1 and 64")
		}
		if spec.Memory <= 0 || spec.Bandwidth <= 0 || spec.BootOverhead < 0 {
			return errors.New("resource memory and bandwidth must be positive, boot_overhead cannot be negative")
		}
	}
	return nil
}

func decodeRequest(data []byte) (SimulationRequest, error) {
	req := defaultRequest()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return req, err
	}
	if err := validateRequest(req); err != nil {
		return req, err
	}
	return req, nil
}

func stagesForPreset(id string) (string, []string) {
	for _, preset := range presets {
		if preset["id"] == id {
			return id, preset["stages"].([]string)
		}
	}
	return presets[0]["id"].(string), presets[0]["stages"].([]string)
}
