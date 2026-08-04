package main

import "testing"

func TestImportedWfcommonsApplicationWorkflowsAreCompleteDAGs(t *testing.T) {
	for workflowID, dataset := range wfcommonsWorkflowDatasets {
		t.Run(workflowID, func(t *testing.T) {
			yamlText, err := readExperimentText(dataset.YAMLFile)
			if err != nil {
				t.Fatal(err)
			}
			req := defaultRequest()
			req.WorkflowYAML = &yamlText
			req.TaskCount = dataset.TaskCount
			workflow, err := generateWorkflow(req)
			if err != nil {
				t.Fatal(err)
			}
			if err := applyWfcommonsExperimentData(workflowID, &workflow); err != nil {
				t.Fatal(err)
			}
			if len(workflow.Tasks) != dataset.TaskCount {
				t.Fatalf("tasks=%d, want %d", len(workflow.Tasks), dataset.TaskCount)
			}
			positiveRuntime := 0
			for _, task := range workflow.Tasks {
				if task.BaseRuntime < 0 {
					t.Fatalf("task %s has negative runtime %v", task.ID, task.BaseRuntime)
				}
				if task.BaseRuntime > 0 {
					positiveRuntime++
				}
			}
			if positiveRuntime == 0 {
				t.Fatal("workflow has no positive runtime")
			}
			dataMB := 0.0
			for _, dependency := range workflow.Dependencies {
				if dependency.DataMB < 0 {
					t.Fatalf("edge %s -> %s has negative data", dependency.Source, dependency.Target)
				}
				if dependency.FileCount < 1 {
					t.Fatalf("edge %s -> %s has invalid file count %d", dependency.Source, dependency.Target, dependency.FileCount)
				}
				dataMB += dependency.DataMB
			}
			if dataMB <= 0 {
				t.Fatal("workflow has no transferred data")
			}
			if _, err := topologicalOrder(GeneratedSimulation{Workflow: workflow}); err != nil {
				t.Fatalf("invalid DAG: %v", err)
			}
		})
	}
}
