import { defaultRequest, tabs } from './constants.js';

export const SNAPSHOT_SCHEMA = "scheduler-simulator.snapshot";
export const SNAPSHOT_VERSION = 1;

const phases = new Set(["workflow", "matrices", "results"]);

export function buildSimulationSnapshot(state) {
  return {
    schema: SNAPSHOT_SCHEMA,
    version: SNAPSHOT_VERSION,
    exported_at: new Date().toISOString(),
    request: state.request,
    workflow_mode: state.workflowMode,
    workflow_yaml: state.workflowYaml,
    workflow_file_name: state.workflowFileName,
    generated: state.generated,
    schedule_response: state.scheduleResponse,
    selected_option_id: state.selectedOptionId,
    result: state.result,
    phase: state.phase,
    active_tab: state.activeTab,
    selected_task_id: state.selectedTaskId,
  };
}

export function restoreSimulationSnapshot(payload) {
  const data = parseSnapshotPayload(payload);
  if (isCurrentSnapshot(data)) {
    return restoreCurrentSnapshot(data);
  }
  if (isLegacyResult(data)) {
    return restoreLegacyResult(data);
  }
  throw new Error("Invalid simulator snapshot");
}

function parseSnapshotPayload(payload) {
  if (typeof payload === "string") {
    try {
      return JSON.parse(payload);
    } catch {
      throw new Error("Invalid JSON file");
    }
  }
  return payload;
}

function restoreCurrentSnapshot(snapshot) {
  if (snapshot.version !== SNAPSHOT_VERSION) {
    throw new Error(`Unsupported simulator snapshot version: ${snapshot.version}`);
  }
  const generated = snapshot.generated || null;
  const scheduleResponse = snapshot.schedule_response || null;
  const selectedOption = selectedScheduleOption(scheduleResponse, snapshot.selected_option_id);
  const result = selectedOption?.result || snapshot.result || null;
  const phase = normalizePhase(snapshot.phase, generated, result);
  return {
    request: normalizeRequest(snapshot.request, generated, result),
    workflowMode: snapshot.workflow_mode === "yaml" ? "yaml" : "random",
    workflowYaml: typeof snapshot.workflow_yaml === "string" ? snapshot.workflow_yaml : "",
    workflowFileName: typeof snapshot.workflow_file_name === "string" ? snapshot.workflow_file_name : "",
    generated,
    scheduleResponse,
    selectedOptionId: selectedOption?.id || snapshot.selected_option_id || null,
    result,
    phase,
    activeTab: normalizeTab(snapshot.active_tab, phase),
    selectedTaskId: normalizeTaskId(snapshot.selected_task_id, result || generated),
    message: "",
  };
}

function restoreLegacyResult(result) {
  const generated = {
    id: result.id,
    seed: result.seed,
    workflow: result.workflow,
    resources: result.resources,
    sla: result.sla,
    matrices: result.matrices,
  };
  const optionId = `${result.id || "legacy"}-option-1`;
  return {
    request: normalizeRequest(null, generated, result),
    workflowMode: "random",
    workflowYaml: "",
    workflowFileName: "",
    generated,
    scheduleResponse: {
      selected_option_id: optionId,
      constraints: {
        budget_limit: result.sla?.budget_limit ?? null,
        deadline_limit: result.sla?.deadline_limit ?? null,
        option_count: 1,
        beam_width: result.sla?.beam_width ?? defaultRequest.beam_width,
      },
      options: [{
        id: optionId,
        rank: 1,
        feasible: true,
        recommended: true,
        budget_used: result.scheduler_variables?.b_used ?? 0,
        budget_limit: result.sla?.budget_limit ?? null,
        budget_violation: 0,
        makespan: result.scheduler_variables?.makespan ?? 0,
        deadline_limit: result.sla?.deadline_limit ?? null,
        deadline_violation: 0,
        machine_signature: "",
        machine_distribution: {},
        weighted_score: 0,
        weighted_time_percent: 0,
        weighted_cost_percent: 0,
        diversity_score: 0,
        result,
      }],
    },
    selectedOptionId: optionId,
    result,
    phase: "results",
    activeTab: "DAG",
    selectedTaskId: result.workflow?.tasks?.[0]?.id || null,
    message: "Imported legacy result JSON with partial session state.",
  };
}

function isCurrentSnapshot(value) {
  return value?.schema === SNAPSHOT_SCHEMA;
}

function isLegacyResult(value) {
  return Boolean(value?.workflow?.tasks && value?.resources && value?.matrices && value?.assignments);
}

function selectedScheduleOption(scheduleResponse, selectedOptionId) {
  const options = scheduleResponse?.options || [];
  return options.find((option) => option.id === selectedOptionId) || options[0] || null;
}

function normalizePhase(phase, generated, result) {
  if (phases.has(phase)) return phase;
  if (result) return "results";
  if (generated) return "matrices";
  return "workflow";
}

function normalizeTab(tab, phase) {
  if (phase === "workflow") return "Workflow";
  return tabs.includes(tab) ? tab : "DAG";
}

function normalizeTaskId(taskId, source) {
  const tasks = source?.workflow?.tasks || [];
  if (tasks.some((task) => task.id === taskId)) return taskId;
  return tasks[0]?.id || null;
}

function normalizeRequest(request, generated, result) {
  const sla = generated?.sla || result?.sla || {};
  const resources = generated?.resources || result?.resources || [];
  const clusterMachines = resources.filter((resource) => resource.kind === "cluster").length || defaultRequest.cluster_machines;
  const cloudMachines = resources.filter((resource) => resource.kind === "cloud").length || defaultRequest.cloud_machines;
  const coresPerMachine = resources[0]?.cores?.length || defaultRequest.cores_per_machine;
  return {
    ...defaultRequest,
    ...(request || {}),
    preset: request?.preset || generated?.workflow?.preset || result?.workflow?.preset || defaultRequest.preset,
    seed: request?.seed ?? generated?.seed ?? result?.seed ?? defaultRequest.seed,
    task_count: request?.task_count ?? generated?.workflow?.tasks?.length ?? result?.workflow?.tasks?.length ?? defaultRequest.task_count,
    cluster_machines: request?.cluster_machines ?? clusterMachines,
    cloud_machines: request?.cloud_machines ?? cloudMachines,
    cores_per_machine: request?.cores_per_machine ?? coresPerMachine,
    weight_time: request?.weight_time ?? sla.weight_time ?? defaultRequest.weight_time,
    weight_cost: request?.weight_cost ?? sla.weight_cost ?? defaultRequest.weight_cost,
    budget_limit: request?.budget_limit ?? sla.budget_limit ?? defaultRequest.budget_limit,
    deadline_limit: request?.deadline_limit ?? sla.deadline_limit ?? defaultRequest.deadline_limit,
    option_count: request?.option_count ?? sla.option_count ?? defaultRequest.option_count,
    beam_width: request?.beam_width ?? sla.beam_width ?? defaultRequest.beam_width,
  };
}
