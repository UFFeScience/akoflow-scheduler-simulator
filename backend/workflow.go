package main

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func generateWorkflow(req SimulationRequest) (Workflow, error) {
	if req.WorkflowYAML != nil && strings.TrimSpace(*req.WorkflowYAML) != "" {
		return workflowFromYAML(req)
	}
	rng := rand.New(rand.NewSource(req.Seed))
	presetID, stages := stagesForPreset(req.Preset)
	maxTaskCores := req.CoresPerMachine
	for _, spec := range req.ResourceSpecs {
		if spec.Cores > maxTaskCores {
			maxTaskCores = spec.Cores
		}
	}
	tasks := make([]Task, 0, req.TaskCount)
	deps := []Dependency{}
	for i := 0; i < req.TaskCount; i++ {
		stage := stages[i%len(stages)]
		tasks = append(tasks, Task{
			ID: fmt.Sprintf("t%d", i+1), Label: fmt.Sprintf("%s-%d", stage, i+1), WorkflowStage: stage,
			CPU: float64(rng.Intn(maxTaskCores) + 1), Memory: round(1.0+rng.Float64()*7.0, 2),
			BaseRuntime: round(8.0+rng.Float64()*27.0, 2), Image: strings.ToLower(stage) + ":latest",
			Predecessors: []string{}, Successors: []string{},
		})
	}
	for i := 1; i < req.TaskCount; i++ {
		candidates := make([]int, i)
		for j := 0; j < i; j++ {
			candidates[j] = j
		}
		rng.Shuffle(len(candidates), func(a, b int) { candidates[a], candidates[b] = candidates[b], candidates[a] })
		edgeSources := map[int]bool{candidates[0]: true}
		for _, candidate := range candidates[1:] {
			distanceFactor := 1.0 / float64(max(1, i-candidate))
			if rng.Float64() < req.EdgeDensity*(0.35+distanceFactor) {
				edgeSources[candidate] = true
			}
		}
		sources := make([]int, 0, len(edgeSources))
		for source := range edgeSources {
			sources = append(sources, source)
		}
		sort.Ints(sources)
		for _, sourceIndex := range sources {
			source := &tasks[sourceIndex]
			target := &tasks[i]
			target.Predecessors = append(target.Predecessors, source.ID)
			source.Successors = append(source.Successors, target.ID)
			deps = append(deps, Dependency{Source: source.ID, Target: target.ID, DataMB: round(20.0+rng.Float64()*880.0, 2)})
		}
	}
	return workflowWithPredecessors(presetID, tasks, deps), nil
}

type yamlDocument struct {
	Name string `yaml:"name"`
	Spec struct {
		Runtime    any            `yaml:"runtime"`
		Image      any            `yaml:"image"`
		Activities []yamlActivity `yaml:"activities"`
	} `yaml:"spec"`
}

type yamlActivity struct {
	Name        string `yaml:"name"`
	Runtime     any    `yaml:"runtime"`
	Image       any    `yaml:"image"`
	Run         any    `yaml:"run"`
	MemoryLimit any    `yaml:"memoryLimit"`
	CPULimit    any    `yaml:"cpuLimit"`
	DependsOn   any    `yaml:"dependsOn"`
}

func workflowFromYAML(req SimulationRequest) (Workflow, error) {
	var doc yamlDocument
	if err := yaml.Unmarshal([]byte(*req.WorkflowYAML), &doc); err != nil {
		return Workflow{}, fmt.Errorf("Invalid workflow YAML: %w", err)
	}
	if len(doc.Spec.Activities) == 0 {
		return Workflow{}, errors.New("Workflow YAML must contain spec.activities")
	}
	rng := rand.New(rand.NewSource(req.Seed))
	workflowName := doc.Name
	if workflowName == "" {
		workflowName = req.Preset
	}
	if workflowName == "" {
		workflowName = "Akoflow"
	}
	globalRuntime := fmt.Sprint(doc.Spec.Runtime)
	if doc.Spec.Runtime == nil {
		globalRuntime = ""
	}
	globalImage := fmt.Sprint(doc.Spec.Image)
	if doc.Spec.Image == nil {
		globalImage = ""
	}
	seen := map[string]bool{}
	tasks := make([]Task, 0, len(doc.Spec.Activities))
	for i, activity := range doc.Spec.Activities {
		taskID := strings.TrimSpace(activity.Name)
		if taskID == "" {
			return Workflow{}, fmt.Errorf("Activity at index %d must define name", i)
		}
		if seen[taskID] {
			return Workflow{}, fmt.Errorf("Duplicate activity name: %s", taskID)
		}
		seen[taskID] = true
		runCommand := stringAny(activity.Run)
		runtime := stringAny(activity.Runtime)
		if runtime == "" {
			runtime = globalRuntime
		}
		stage := activityStage(taskID, runCommand)
		image := stringAny(activity.Image)
		if image == "" {
			image = globalImage
		}
		if image == "" {
			image = strings.ToLower(stage) + ":latest"
		}
		t := Task{
			ID: taskID, Label: taskID, WorkflowStage: stage,
			CPU: parseCPULimit(activity.CPULimit, rng), Memory: parseMemoryLimit(activity.MemoryLimit, rng),
			BaseRuntime: round(8.0+rng.Float64()*27.0, 2), Image: image,
			Predecessors: []string{}, Successors: []string{},
		}
		if runtime != "" {
			t.Runtime = &runtime
		}
		if runCommand != "" {
			t.Run = &runCommand
		}
		tasks = append(tasks, t)
	}
	taskIndex := map[string]int{}
	for i, task := range tasks {
		taskIndex[task.ID] = i
	}
	deps := []Dependency{}
	for _, activity := range doc.Spec.Activities {
		taskID := strings.TrimSpace(activity.Name)
		depNames, err := dependsOnList(activity.DependsOn)
		if err != nil {
			return Workflow{}, fmt.Errorf("dependsOn for %s must be a list", taskID)
		}
		for _, sourceID := range depNames {
			sourceIndex, ok := taskIndex[sourceID]
			if !ok {
				return Workflow{}, fmt.Errorf("Activity %s depends on unknown activity %s", taskID, sourceID)
			}
			targetIndex := taskIndex[taskID]
			tasks[targetIndex].Predecessors = append(tasks[targetIndex].Predecessors, sourceID)
			tasks[sourceIndex].Successors = append(tasks[sourceIndex].Successors, taskID)
			deps = append(deps, Dependency{Source: sourceID, Target: taskID, DataMB: round(20.0+rng.Float64()*880.0, 2)})
		}
	}
	workflow := workflowWithPredecessors(workflowName, tasks, deps)
	if _, err := topologicalOrder(GeneratedSimulation{Workflow: workflow}); err != nil {
		return Workflow{}, errors.New("Workflow YAML contains a dependency cycle")
	}
	return workflow, nil
}

func workflowWithPredecessors(preset string, tasks []Task, deps []Dependency) Workflow {
	predecessors := map[string][]string{}
	for _, task := range tasks {
		values := append([]string{}, task.Predecessors...)
		sort.Strings(values)
		predecessors[task.ID] = values
	}
	return Workflow{Preset: preset, Tasks: tasks, Dependencies: deps, PredecessorSets: predecessors}
}

func stringAny(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func dependsOnList(v any) ([]string, error) {
	if v == nil {
		return []string{}, nil
	}
	switch value := v.(type) {
	case string:
		return []string{strings.TrimSpace(value)}, nil
	case []any:
		out := []string{}
		for _, item := range value {
			out = append(out, strings.TrimSpace(fmt.Sprint(item)))
		}
		return out, nil
	default:
		return nil, errors.New("invalid dependsOn")
	}
}

func activityStage(name, runCommand string) string {
	re := regexp.MustCompile(`(?:^|&&\s*)([A-Za-z][A-Za-z0-9_-]*)`)
	skip := map[string]bool{"cd": true, "cp": true, "mkdir": true, "mv": true, "rm": true, "set": true}
	for _, match := range re.FindAllStringSubmatch(runCommand, -1) {
		if !skip[match[1]] {
			return match[1]
		}
	}
	for _, token := range regexp.MustCompile(`[^A-Za-z]+`).Split(name, -1) {
		if token != "" {
			return token
		}
	}
	return "activity"
}

func parseCPULimit(value any, rng *rand.Rand) float64 {
	if f, ok := numericAny(value); ok {
		return maxf(0.1, round(f, 2))
	}
	if s := stringAny(value); s != "" {
		if f, ok := firstNumber(s); ok {
			return maxf(0.1, round(f, 2))
		}
	}
	return round(0.7+rng.Float64()*2.7, 2)
}

func parseMemoryLimit(value any, rng *rand.Rand) float64 {
	if f, ok := numericAny(value); ok {
		return maxf(0.1, round(f/1024, 2))
	}
	if s := stringAny(value); s != "" {
		if f, ok := firstNumber(s); ok {
			unit := strings.ToLower(strings.TrimSpace(s[strings.Index(s, fmt.Sprintf("%.0f", math.Trunc(f)))+len(fmt.Sprintf("%.0f", math.Trunc(f))):]))
			if strings.Contains(unit, "g") {
				return maxf(0.1, round(f, 2))
			}
			return maxf(0.1, round(f/1024, 2))
		}
	}
	return round(1.0+rng.Float64()*7.0, 2)
}

func numericAny(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case float32:
		return float64(v), true
	default:
		return 0, false
	}
}

func firstNumber(s string) (float64, bool) {
	re := regexp.MustCompile(`\d+(?:\.\d+)?`)
	match := re.FindString(s)
	if match == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(match, 64)
	return f, err == nil
}
