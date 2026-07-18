import assert from "node:assert/strict";
import test from "node:test";

import { buildSimulationSnapshot, restoreSimulationSnapshot, SNAPSHOT_SCHEMA, SNAPSHOT_VERSION } from "./lib/simulationSnapshot.js";

function generatedFixture() {
  return {
    id: "sim-1",
    seed: 42,
    workflow: {
      preset: "Montage",
      tasks: [{ id: "t1" }, { id: "t2" }],
      dependencies: [],
      predecessor_sets: {},
    },
    resources: [
      { id: "c1", kind: "cluster", cores: [{ id: "c1-0" }] },
      { id: "v1", kind: "cloud", cores: [{ id: "v1-0" }] },
    ],
    sla: {
      weight_time: 1,
      weight_cost: 0,
      budget_limit: 50,
      deadline_limit: 100,
      option_count: 2,
      beam_width: 120,
    },
    matrices: { et_0: {} },
  };
}

function resultFixture(id = "result-1") {
  return {
    ...generatedFixture(),
    id,
    assignments: [{ task_id: "t1" }],
    machine_stop_intervals: [],
    scheduler_steps: [],
    scheduler_variables: { makespan: 25, b_used: 12 },
    timing_variables: {},
    cost_variables: {},
    interference_variables: {},
    deviation_variables: {},
  };
}

test("builds a versioned snapshot with all session state", () => {
  const generated = generatedFixture();
  const result = resultFixture();
  const snapshot = buildSimulationSnapshot({
    request: { preset: "Montage", seed: 42 },
    workflowMode: "yaml",
    workflowYaml: "activities: []",
    workflowFileName: "workflow.yaml",
    generated,
    scheduleResponse: { selected_option_id: "option-1", options: [{ id: "option-1", result }] },
    selectedOptionId: "option-1",
    result,
    phase: "results",
    activeTab: "Gantt",
    selectedTaskId: "t2",
  });

  assert.equal(snapshot.schema, SNAPSHOT_SCHEMA);
  assert.equal(snapshot.version, SNAPSHOT_VERSION);
  assert.equal(snapshot.workflow_yaml, "activities: []");
  assert.equal(snapshot.schedule_response.options[0].result.id, "result-1");
  assert.equal(snapshot.selected_task_id, "t2");
});

test("restores a complete snapshot and keeps the selected schedule option", () => {
  const generated = generatedFixture();
  const first = resultFixture("first");
  const second = resultFixture("second");
  const restored = restoreSimulationSnapshot(JSON.stringify({
    schema: SNAPSHOT_SCHEMA,
    version: SNAPSHOT_VERSION,
    request: { preset: "Imported", seed: 7, weight_time: 0, weight_cost: 1 },
    workflow_mode: "yaml",
    workflow_yaml: "activities: []",
    workflow_file_name: "workflow.yaml",
    generated,
    schedule_response: {
      selected_option_id: "second",
      options: [{ id: "first", result: first }, { id: "second", result: second }],
    },
    selected_option_id: "second",
    result: first,
    phase: "results",
    active_tab: "Variables",
    selected_task_id: "t2",
  }));

  assert.equal(restored.workflowMode, "yaml");
  assert.equal(restored.result.id, "second");
  assert.equal(restored.selectedOptionId, "second");
  assert.equal(restored.phase, "results");
  assert.equal(restored.activeTab, "Variables");
  assert.equal(restored.selectedTaskId, "t2");
});

test("falls back to the first option when selected option is missing", () => {
  const restored = restoreSimulationSnapshot({
    schema: SNAPSHOT_SCHEMA,
    version: SNAPSHOT_VERSION,
    generated: generatedFixture(),
    schedule_response: {
      options: [{ id: "first", result: resultFixture("first") }],
    },
    selected_option_id: "missing",
    phase: "results",
  });

  assert.equal(restored.selectedOptionId, "first");
  assert.equal(restored.result.id, "first");
});

test("accepts legacy result JSON as partial imported session state", () => {
  const restored = restoreSimulationSnapshot(JSON.stringify(resultFixture("legacy-result")));

  assert.equal(restored.phase, "results");
  assert.equal(restored.generated.id, "legacy-result");
  assert.equal(restored.scheduleResponse.options.length, 1);
  assert.equal(restored.result.id, "legacy-result");
  assert.match(restored.message, /legacy result/i);
});

test("rejects invalid JSON and unsupported payloads", () => {
  assert.throws(() => restoreSimulationSnapshot("{"), /Invalid JSON/);
  assert.throws(() => restoreSimulationSnapshot({ hello: "world" }), /Invalid simulator snapshot/);
});
