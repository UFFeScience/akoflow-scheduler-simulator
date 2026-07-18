import { useEffect, useMemo, useState } from 'react';
import { API_URL, defaultRequest } from '../lib/constants.js';
import { buildResourceSpecs, syncResourceSpecs } from '../lib/resourceSpecs.js';
import { buildSimulationSnapshot, restoreSimulationSnapshot } from '../lib/simulationSnapshot.js';

export function useSimulatorController() {
  const [request, setRequest] = useState(defaultRequest);
  const [presets, setPresets] = useState([]);
  const [generated, setGenerated] = useState(null);
  const [result, setResult] = useState(null);
  const [scheduleResponse, setScheduleResponse] = useState(null);
  const [selectedOptionId, setSelectedOptionId] = useState(null);
  const [phase, setPhase] = useState("workflow");
  const [activeTab, setActiveTab] = useState("Workflow");
  const [selectedTaskId, setSelectedTaskId] = useState(null);
  const [status, setStatus] = useState("idle");
  const [statusMessage, setStatusMessage] = useState("");
  const [workflowMode, setWorkflowMode] = useState("random");
  const [workflowYaml, setWorkflowYaml] = useState("");
  const [workflowFileName, setWorkflowFileName] = useState("");
  const [theme, setTheme] = useState("light");

  useEffect(() => {
    fetch(`${API_URL}/api/presets`)
      .then((response) => response.json())
      .then((data) => setPresets(data.presets || []))
      .catch(() => setPresets([]));
  }, []);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
  }, [theme]);

  function generatedWithCurrentSla() {
    if (!generated) return null;
    return {
      ...generated,
      sla: {
        ...generated.sla,
        weight_time: request.weight_time,
        weight_cost: request.weight_cost,
        budget_limit: request.budget_limit,
        deadline_limit: request.deadline_limit,
        option_count: request.option_count,
        beam_width: request.beam_width,
      },
    };
  }

  async function generateWorkflowAndMatrices() {
    setStatus("running");
    setStatusMessage("");
    const response = await fetch(`${API_URL}/api/simulations/generate-only`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ...request, workflow_yaml: workflowMode === "yaml" ? workflowYaml || null : null }),
    });
    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      setStatus("error");
      setStatusMessage(error.detail || "Workflow generation failed");
      return;
    }
    const data = await response.json();
    setGenerated(data);
    setResult(null);
    setScheduleResponse(null);
    setSelectedOptionId(null);
    setSelectedTaskId(data.workflow.tasks[0]?.id || null);
    setPhase("matrices");
    setActiveTab("DAG");
    setStatus("ready");
  }

  async function runSchedule({ preserveView = false } = {}) {
    const schedulePayload = generatedWithCurrentSla();
    if (!schedulePayload) return;
    setStatus("running");
    setStatusMessage("");
    const response = await fetch(`${API_URL}/api/simulations/schedule`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(schedulePayload),
    });
    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      setStatus("error");
      setStatusMessage(error.detail || "Simulation scheduling failed");
      return;
    }
    const data = await response.json();
    const selectedOption = data.options?.find((option) => option.id === data.selected_option_id) || data.options?.[0] || null;
    setGenerated(schedulePayload);
    setScheduleResponse(data);
    setSelectedOptionId(selectedOption?.id || null);
    setResult(selectedOption?.result || null);
    setSelectedTaskId(selectedOption?.result?.workflow.tasks[0]?.id || null);
    if (!preserveView) {
      setPhase("results");
      setActiveTab("DAG");
    }
    setStatus("ready");
  }

  async function saveMatricesAndSchedule() {
    await runSchedule();
  }

  async function calculateCurrentSchedule() {
    await runSchedule({ preserveView: true });
  }

  function resetFlow() {
    setRequest(defaultRequest);
    setWorkflowYaml("");
    setWorkflowFileName("");
    setWorkflowMode("random");
    setGenerated(null);
    setResult(null);
    setScheduleResponse(null);
    setSelectedOptionId(null);
    setSelectedTaskId(null);
    setPhase("workflow");
    setActiveTab("Workflow");
    setStatus("idle");
    setStatusMessage("");
  }

  async function importWorkflowFile(file) {
    if (!file) return;
    setWorkflowYaml(await file.text());
    setWorkflowFileName(file.name);
    setWorkflowMode("yaml");
  }

  async function importSnapshotFile(file) {
    if (!file) return;
    try {
      const restored = restoreSimulationSnapshot(await file.text());
      setRequest(restored.request);
      setWorkflowMode(restored.workflowMode);
      setWorkflowYaml(restored.workflowYaml);
      setWorkflowFileName(restored.workflowFileName);
      setGenerated(restored.generated);
      setScheduleResponse(restored.scheduleResponse);
      setSelectedOptionId(restored.selectedOptionId);
      setResult(restored.result);
      setPhase(restored.phase);
      setActiveTab(restored.activeTab);
      setSelectedTaskId(restored.selectedTaskId);
      setStatus("ready");
      setStatusMessage(restored.message);
    } catch (error) {
      setStatus("error");
      setStatusMessage(error.message || "Snapshot import failed");
    }
  }

  function updateRequest(key, value) {
    setRequest((current) => {
      const next = { ...current, [key]: value };
      if (["cluster_machines", "cloud_machines", "cores_per_machine"].includes(key)) {
        next.resource_specs = syncResourceSpecs(current.resource_specs || [], next.cluster_machines, next.cloud_machines, next.cores_per_machine);
      }
      return next;
    });
  }

  const selectedAssignment = useMemo(
    () => (phase === "results" ? result?.assignments.find((assignment) => assignment.task_id === selectedTaskId) : null),
    [phase, result, selectedTaskId],
  );

  function selectScheduleOption(optionId) {
    const option = scheduleResponse?.options.find((item) => item.id === optionId);
    if (!option) return;
    setSelectedOptionId(option.id);
    setResult(option.result);
    setSelectedTaskId(option.result.workflow.tasks[0]?.id || null);
  }

  return {
    request, presets, generated, result, scheduleResponse, selectedOptionId, phase, activeTab, selectedTaskId, status, statusMessage,
    workflowMode, workflowYaml, workflowFileName, theme, selectedAssignment,
    setGenerated, setPhase, setActiveTab, setSelectedTaskId, setWorkflowMode, setTheme,
    generateWorkflowAndMatrices, saveMatricesAndSchedule, calculateCurrentSchedule, resetFlow, importWorkflowFile, selectScheduleOption,
    importSnapshotFile,
    exportSnapshot: () => buildSimulationSnapshot({
      request, workflowMode, workflowYaml, workflowFileName, generated, scheduleResponse, selectedOptionId, result, phase, activeTab, selectedTaskId,
    }),
    clearWorkflowFile: () => { setWorkflowYaml(""); setWorkflowFileName(""); setWorkflowMode("random"); },
    updateRequest,
    updateResourceSpec: (id, key, value) => setRequest((current) => ({
      ...current,
      resource_specs: (current.resource_specs || []).map((resource) => (resource.id === id ? { ...resource, [key]: value } : resource)),
    })),
    updateWeights: (nextWeights) => setRequest((current) => ({ ...current, ...nextWeights })),
    resetResourceSpecs: () => setRequest((current) => ({
      ...current,
      resource_specs: buildResourceSpecs(current.cluster_machines, current.cloud_machines, current.cores_per_machine),
    })),
  };
}
