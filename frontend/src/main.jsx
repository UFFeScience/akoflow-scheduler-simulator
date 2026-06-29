import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import { ChevronLeft, ChevronRight, Download, Moon, Play, RefreshCw, Sun, Upload } from "lucide-react";
import "./styles.css";

const API_URL = import.meta.env.VITE_API_URL || "http://localhost:8000";
const tabs = ["DAG", "Gantt", "Steps", "Stats", "Pairwise", "Machines", "Matrices", "Variables", "Tables"];
const interferenceDimensions = ["cpu", "memory", "io", "network"];
const aggregateInterferenceDimension = "phi_n aggregated";
const interferenceDimensionOptions = [...interferenceDimensions, aggregateInterferenceDimension];
const scalableTimeMatrices = new Set(["et_0", "et_star", "transfer_delay", "container_overhead"]);

const defaultRequest = {
  preset: "Montage",
  seed: 42,
  task_count: 12,
  edge_density: 0.22,
  cluster_machines: 3,
  cloud_machines: 2,
  cores_per_machine: 2,
  deadline: 160,
  budget: 260,
  weight_time: 0.55,
  weight_cost: 0.3,
  weight_interference: 0.15,
  penalty_deadline: 3,
  penalty_budget: 2,
};

function App() {
  const [request, setRequest] = useState(defaultRequest);
  const [presets, setPresets] = useState([]);
  const [generated, setGenerated] = useState(null);
  const [result, setResult] = useState(null);
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

  function requestPayload() {
    return { ...request, workflow_yaml: workflowMode === "yaml" ? workflowYaml || null : null };
  }

  async function generateWorkflowAndMatrices() {
    setStatus("running");
    setStatusMessage("");
    const response = await fetch(`${API_URL}/api/simulations/generate-only`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(requestPayload()),
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
    setSelectedTaskId(data.workflow.tasks[0]?.id || null);
    setPhase("matrices");
    setActiveTab("DAG");
    setStatus("ready");
  }

  async function saveMatricesAndSchedule() {
    if (!generated) return;
    setStatus("running");
    setStatusMessage("");
    const response = await fetch(`${API_URL}/api/simulations/schedule`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(generated),
    });
    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      setStatus("error");
      setStatusMessage(error.detail || "Simulation scheduling failed");
      return;
    }
    const data = await response.json();
    setResult(data);
    setSelectedTaskId(data.workflow.tasks[0]?.id || null);
    setPhase("results");
    setActiveTab("DAG");
    setStatus("ready");
  }

  function resetFlow() {
    setRequest(defaultRequest);
    setWorkflowYaml("");
    setWorkflowFileName("");
    setWorkflowMode("random");
    setGenerated(null);
    setResult(null);
    setSelectedTaskId(null);
    setPhase("workflow");
    setActiveTab("Workflow");
    setStatus("idle");
    setStatusMessage("");
  }

  async function importWorkflowFile(file) {
    if (!file) return;
    const text = await file.text();
    setWorkflowYaml(text);
    setWorkflowFileName(file.name);
    setWorkflowMode("yaml");
  }

  function clearWorkflowFile() {
    setWorkflowYaml("");
    setWorkflowFileName("");
    setWorkflowMode("random");
  }

  function updateRequest(key, value) {
    setRequest((current) => ({ ...current, [key]: value }));
  }

  function exportJson() {
    if (!result) return;
    const blob = new Blob([JSON.stringify(result, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `${result.id}.json`;
    anchor.click();
    URL.revokeObjectURL(url);
  }

  const selectedAssignment = useMemo(
    () => (phase === "results" ? result?.assignments.find((assignment) => assignment.task_id === selectedTaskId) : null),
    [phase, result, selectedTaskId],
  );

  return (
    <div className={`app-shell ${phase === "workflow" ? "setup-shell" : phase === "matrices" ? "matrix-shell" : "result-shell"}`}>
      {phase !== "workflow" && (
        <aside className="left-panel">
          <ActionPanel
            phase={phase}
            generated={generated}
            status={status}
            statusMessage={statusMessage}
            onSave={saveMatricesAndSchedule}
            onBackToWorkflow={() => { setPhase("workflow"); setActiveTab("Workflow"); }}
            onEditMatrices={() => { setPhase("matrices"); setActiveTab("Matrices"); }}
          />
        </aside>
      )}

      <main className="workspace">
        <header className="topbar">
          <div>
            <strong>{phase === "results" ? result?.workflow.preset : generated?.workflow.preset || request.preset}</strong>
            <span>
              {phase === "results" && result
                ? `${result.workflow.tasks.length} tasks / ${result.resources.length} machines${workflowYaml ? " / imported YAML" : ""}`
                : generated
                  ? `${generated.workflow.tasks.length} tasks / ${generated.resources.length} machines / editing matrices`
                  : "Select synthetic generation or import YAML"}
            </span>
          </div>
          <div className="metrics">
            <Metric label="Makespan" value={fmt(phase === "results" ? result?.scheduler_variables.makespan : null)} />
            <Metric label="Budget used" value={fmt(phase === "results" ? result?.scheduler_variables.b_used : null)} />
            <Metric label="Cost C_W" value={fmt(phase === "results" ? result?.cost_variables.c_w : null)} />
            <button className="icon-button" title="Reset flow" onClick={resetFlow}>
              <RefreshCw size={18} />
            </button>
            <button className="icon-button" title="Toggle theme" onClick={() => setTheme((current) => (current === "light" ? "dark" : "light"))}>
              {theme === "light" ? <Moon size={18} /> : <Sun size={18} />}
            </button>
            <button className="icon-button" title="Export JSON" onClick={exportJson} disabled={phase !== "results" || !result}>
              <Download size={18} />
            </button>
          </div>
        </header>

        <nav className="tabs">
          {(phase === "results" ? tabs : phase === "matrices" ? ["DAG", "Matrices"] : ["Workflow"]).map((tab) => (
            <button key={tab} className={activeTab === tab ? "active" : ""} onClick={() => setActiveTab(tab)}>
              {tab}
            </button>
          ))}
        </nav>

        <section className="canvas">
          {phase === "workflow" && (
            <WorkflowStartScreen
              request={request}
              presets={presets}
              workflowMode={workflowMode}
              workflowYaml={workflowYaml}
              workflowFileName={workflowFileName}
              status={status}
              statusMessage={statusMessage}
              onUpdateRequest={updateRequest}
              onWorkflowModeChange={setWorkflowMode}
              onImportWorkflowFile={importWorkflowFile}
              onClearWorkflowFile={clearWorkflowFile}
              onGenerate={generateWorkflowAndMatrices}
            />
          )}
          {phase === "matrices" && generated && activeTab === "DAG" && <WorkflowPreviewView generated={generated} selectedTaskId={selectedTaskId} onSelect={setSelectedTaskId} />}
          {phase === "matrices" && generated && activeTab === "Matrices" && <EditableMatricesView generated={generated} onChange={setGenerated} />}
          {phase === "results" && result && activeTab === "DAG" && <DagView result={result} selectedTaskId={selectedTaskId} onSelect={setSelectedTaskId} />}
          {phase === "results" && result && activeTab === "Gantt" && <GanttView result={result} selectedTaskId={selectedTaskId} onSelect={setSelectedTaskId} />}
          {phase === "results" && result && activeTab === "Steps" && <StepsView result={result} onSelect={setSelectedTaskId} />}
          {phase === "results" && result && activeTab === "Stats" && <ActivityStatsView result={result} onSelect={setSelectedTaskId} />}
          {phase === "results" && result && activeTab === "Pairwise" && <PairwiseInterferenceView result={result} onSelect={setSelectedTaskId} />}
          {phase === "results" && result && activeTab === "Machines" && <MachineView result={result} selectedTaskId={selectedTaskId} onSelect={setSelectedTaskId} />}
          {phase === "results" && result && activeTab === "Matrices" && <MatricesView result={result} />}
          {phase === "results" && result && activeTab === "Variables" && <VariablesView result={result} />}
          {phase === "results" && result && activeTab === "Tables" && <TablesView result={result} onSelect={setSelectedTaskId} />}
        </section>
      </main>

      {phase === "results" && (
        <aside className="right-panel">
          <DetailsPanel result={result} assignment={selectedAssignment} taskId={selectedTaskId} />
        </aside>
      )}
    </div>
  );
}

function ControlInput({ label, value, min, max, step, onChange }) {
  return (
    <label className="control">
      <span>{label}</span>
      <input type="number" value={value} min={min} max={max} step={step} onChange={(event) => onChange(Number(event.target.value))} />
    </label>
  );
}

function ControlSelect({ label, value, onChange, children }) {
  return (
    <label className="control">
      <span>{label}</span>
      <select value={value} onChange={(event) => onChange(event.target.value)}>
        {children}
      </select>
    </label>
  );
}

function Metric({ label, value }) {
  return (
    <div className="metric">
      <span>{label}</span>
      <strong>{value ?? "-"}</strong>
    </div>
  );
}

function ActionPanel({ phase, generated, status, statusMessage, onSave, onBackToWorkflow, onEditMatrices }) {
  return (
    <>
      <div className="brand-row">
        <div>
          <h1>Scheduler Simulator</h1>
          <p>{phase === "matrices" ? "Matrix review" : "Scheduled result"}</p>
        </div>
      </div>
      <section className="workflow-import">
        <div>
          <span>Current workflow</span>
          <strong>{generated?.workflow.preset || "-"}</strong>
        </div>
        <div className="mini-stats">
          <span>{generated?.workflow.tasks.length || 0} activities</span>
          <span>{generated?.workflow.dependencies.length || 0} dependencies</span>
          <span>{generated?.resources.length || 0} machines</span>
        </div>
      </section>
      {phase === "matrices" && (
        <>
          <button className="primary-button" onClick={onSave} disabled={status === "running" || !generated}>
            <Play size={17} />
            Save and schedule
          </button>
          <button className="secondary-button full-width-button" type="button" onClick={onBackToWorkflow}>
            Back to workflow
          </button>
        </>
      )}
      {phase === "results" && (
        <button className="primary-button" onClick={onEditMatrices} disabled={!generated}>
          <ChevronLeft size={17} />
          Edit matrices
        </button>
      )}
      {statusMessage && <p className="status-message error">{statusMessage}</p>}
    </>
  );
}

function WorkflowStartScreen({
  request,
  presets,
  workflowMode,
  workflowYaml,
  workflowFileName,
  status,
  statusMessage,
  onUpdateRequest,
  onWorkflowModeChange,
  onImportWorkflowFile,
  onClearWorkflowFile,
  onGenerate,
}) {
  return (
    <div className="steps-view">
      <section className="data-section start-screen">
        <span>Step 1</span>
        <h2>Choose workflow source</h2>
        <p>Select a synthetic random workflow or import an Akoflow YAML workflow. The next screen shows the DAG and dependencies before matrices are saved.</p>
      </section>
      <section className="data-section setup-panel">
        <div className="setup-section">
          <h2>Workflow source</h2>
          <div className="mode-selector">
            <button
              type="button"
              className={workflowMode === "random" ? "mode-card active" : "mode-card"}
              onClick={() => onWorkflowModeChange("random")}
            >
              <strong>Generate random workflow</strong>
              <span>Use preset, task count, seed, and edge density.</span>
            </button>
            <button
              type="button"
              className={workflowMode === "yaml" ? "mode-card active" : "mode-card"}
              onClick={() => onWorkflowModeChange("yaml")}
            >
              <strong>Import Akoflow YAML</strong>
              <span>Use activities and dependsOn from a workflow file.</span>
            </button>
          </div>
          <div className="setup-grid">
            {workflowMode === "random" && (
              <>
                <ControlSelect label="Workflow preset" value={request.preset} onChange={(value) => onUpdateRequest("preset", value)}>
                  {(presets.length ? presets : [{ id: "Montage", label: "Montage" }]).map((preset) => (
                    <option key={preset.id} value={preset.id}>
                      {preset.label}
                    </option>
                  ))}
                </ControlSelect>
                <ControlInput label="Seed" value={request.seed} min={1} step={1} onChange={(value) => onUpdateRequest("seed", value)} />
                <ControlInput label="Tasks" value={request.task_count} min={3} max={80} step={1} onChange={(value) => onUpdateRequest("task_count", value)} />
                <ControlInput label="Edge density" value={request.edge_density} min={0} max={0.8} step={0.01} onChange={(value) => onUpdateRequest("edge_density", value)} />
              </>
            )}
            {workflowMode === "yaml" && (
              <section className="workflow-import inline-import">
                <div>
                  <span>Akoflow workflow YAML</span>
                  <strong>{workflowFileName || "No YAML selected"}</strong>
                </div>
                <label className="file-button">
                  <Upload size={16} />
                  Import YAML
                  <input type="file" accept=".yaml,.yml,text/yaml,text/x-yaml" onChange={(event) => onImportWorkflowFile(event.target.files?.[0])} />
                </label>
                {workflowYaml && (
                  <button className="secondary-button" type="button" onClick={onClearWorkflowFile}>
                    Clear YAML
                  </button>
                )}
              </section>
            )}
          </div>
        </div>

        <div className="setup-section">
          <h2>Resources</h2>
          <div className="setup-grid">
            <ControlInput label="Cluster machines" value={request.cluster_machines} min={1} max={20} step={1} onChange={(value) => onUpdateRequest("cluster_machines", value)} />
            <ControlInput label="Cloud machines" value={request.cloud_machines} min={0} max={20} step={1} onChange={(value) => onUpdateRequest("cloud_machines", value)} />
            <ControlInput label="Cores per machine" value={request.cores_per_machine} min={1} max={16} step={1} onChange={(value) => onUpdateRequest("cores_per_machine", value)} />
          </div>
        </div>

        <div className="setup-section">
          <h2>SLA and weights</h2>
          <div className="setup-grid">
            <ControlInput label="SLA deadline" value={request.deadline} min={1} step={1} onChange={(value) => onUpdateRequest("deadline", value)} />
            <ControlInput label="Budget" value={request.budget} min={0} step={1} onChange={(value) => onUpdateRequest("budget", value)} />
            <ControlInput label="Weight time" value={request.weight_time} min={0} max={1} step={0.01} onChange={(value) => onUpdateRequest("weight_time", value)} />
            <ControlInput label="Weight cost" value={request.weight_cost} min={0} max={1} step={0.01} onChange={(value) => onUpdateRequest("weight_cost", value)} />
            <ControlInput label="Weight interference" value={request.weight_interference} min={0} max={1} step={0.01} onChange={(value) => onUpdateRequest("weight_interference", value)} />
            <ControlInput label="Deadline penalty" value={request.penalty_deadline} min={0} step={0.1} onChange={(value) => onUpdateRequest("penalty_deadline", value)} />
            <ControlInput label="Budget penalty" value={request.penalty_budget} min={0} step={0.1} onChange={(value) => onUpdateRequest("penalty_budget", value)} />
          </div>
        </div>

        <button className="primary-button setup-submit" onClick={onGenerate} disabled={status === "running" || (workflowMode === "yaml" && !workflowYaml)}>
          <Play size={17} />
          {workflowMode === "yaml" ? "Import workflow" : "Generate workflow"}
        </button>
        {statusMessage && <p className="status-message error">{statusMessage}</p>}
      </section>
      <div className="stats-grid">
        <Metric label="Workflow source" value={workflowMode === "yaml" ? "YAML import" : "Synthetic random"} />
        <Metric label="YAML file" value={workflowFileName || "-"} />
      </div>
    </div>
  );
}

function WorkflowPreviewView({ generated, selectedTaskId, onSelect }) {
  return (
    <div className="steps-view">
      <section className="data-section step-header">
        <div>
          <span>Step 2</span>
          <h2>{generated.workflow.preset}</h2>
          <p>{generated.workflow.tasks.length} activities and {generated.workflow.dependencies.length} dependencies generated.</p>
        </div>
      </section>
      <DagView result={generated} selectedTaskId={selectedTaskId} onSelect={onSelect} />
      <section className="data-section">
        <h2>Dependencies</h2>
        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                <th>source</th>
                <th>target</th>
                <th>data MB</th>
              </tr>
            </thead>
            <tbody>
              {generated.workflow.dependencies.map((dependency) => (
                <tr key={`${dependency.source}-${dependency.target}`}>
                  <td>{dependency.source}</td>
                  <td>{dependency.target}</td>
                  <td>{fmt(dependency.data_mb)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}

function EditableMatricesView({ generated, onChange }) {
  const matrixEntries = [
    ["et_0", "ET_0"],
    ["et_star", "ET*"],
    ["bandwidth_bw", "Bandwidth BW"],
    ["transfer_delay", "Transfer delay"],
    ["financial_network_cost", "Financial cost"],
    ["container_overhead", "Container overhead"],
  ];
  const [activeMatrix, setActiveMatrix] = useState("et_0");
  const [interferenceResourceId, setInterferenceResourceId] = useState(generated.resources[0]?.id || "");
  const [interferenceDimension, setInterferenceDimension] = useState("cpu");
  const [timeMultiplier, setTimeMultiplier] = useState(1);
  const activeMatrixData = generated.matrices[activeMatrix] || {};
  const canScaleActiveMatrix = scalableTimeMatrices.has(activeMatrix);
  const isAggregateInterference = interferenceDimension === aggregateInterferenceDimension;
  const interferenceMatrix = isAggregateInterference
    ? buildAggregatedInterferenceMatrix(generated.matrices.interference_i_n, interferenceResourceId)
    : generated.matrices.interference_i_n[interferenceResourceId]?.[interferenceDimension] || {};

  function updateMatrixCell(matrixKey, rowKey, columnKey, value) {
    const numericValue = Number(value);
    if (!Number.isFinite(numericValue)) return;
    onChange((current) => {
      const next = structuredClone(current);
      next.matrices[matrixKey][rowKey][columnKey] = numericValue;
      if (matrixKey === "et_0") {
        next.matrices.et_star[rowKey][columnKey] = numericValue;
      }
      return next;
    });
  }

  function updateInterferenceCell(sourceTaskId, targetTaskId, value) {
    const numericValue = Number(value);
    if (!Number.isFinite(numericValue)) return;
    onChange((current) => {
      const next = structuredClone(current);
      next.matrices.interference_i_n[interferenceResourceId][interferenceDimension][sourceTaskId][targetTaskId] = numericValue;
      return next;
    });
  }

  function multiplyActiveMatrix() {
    const multiplier = Number(timeMultiplier);
    if (!Number.isFinite(multiplier) || multiplier < 0 || !canScaleActiveMatrix) return;
    onChange((current) => {
      const next = structuredClone(current);
      for (const rowKey of Object.keys(next.matrices[activeMatrix] || {})) {
        for (const columnKey of Object.keys(next.matrices[activeMatrix][rowKey] || {})) {
          next.matrices[activeMatrix][rowKey][columnKey] = Number((next.matrices[activeMatrix][rowKey][columnKey] * multiplier).toFixed(4));
        }
      }
      if (activeMatrix === "et_0") {
        next.matrices.et_star = structuredClone(next.matrices.et_0);
      }
      return next;
    });
  }

  return (
    <div className="steps-view">
      <section className="data-section step-header">
        <div>
          <span>Step 3</span>
          <h2>Edit generated matrices</h2>
          <p>Values are generated randomly first. Update any matrix values, then save and schedule from the left panel.</p>
        </div>
      </section>

      <section className="data-section pairwise-toolbar">
        <div className="filter-row">
          <label className="control compact-control">
            <span>Matrix</span>
            <select value={activeMatrix} onChange={(event) => setActiveMatrix(event.target.value)}>
              {matrixEntries.map(([key, label]) => <option key={key} value={key}>{label}</option>)}
            </select>
          </label>
          <label className="control compact-control">
            <span>Multiply time by X</span>
            <input type="number" min={0} step={0.1} value={timeMultiplier} onChange={(event) => setTimeMultiplier(event.target.value)} disabled={!canScaleActiveMatrix} />
          </label>
          <button className="secondary-button matrix-scale-button" type="button" onClick={multiplyActiveMatrix} disabled={!canScaleActiveMatrix}>
            Apply x
          </button>
          <label className="control compact-control">
            <span>Interference machine</span>
            <select value={interferenceResourceId} onChange={(event) => setInterferenceResourceId(event.target.value)}>
              {generated.resources.map((resource) => <option key={resource.id} value={resource.id}>{resource.name}</option>)}
            </select>
          </label>
          <label className="control compact-control">
            <span>Interference dimension</span>
            <select value={interferenceDimension} onChange={(event) => setInterferenceDimension(event.target.value)}>
              {interferenceDimensionOptions.map((dimension) => <option key={dimension} value={dimension}>{dimension}</option>)}
            </select>
          </label>
        </div>
      </section>

      <EditableMatrixTable
        title={matrixEntries.find(([key]) => key === activeMatrix)?.[1] || activeMatrix}
        matrix={activeMatrixData}
        onChange={(row, column, value) => updateMatrixCell(activeMatrix, row, column, value)}
      />

      <EditableMatrixTable
        title={`Interference I_n / ${interferenceResourceId} / ${interferenceDimension}`}
        matrix={interferenceMatrix}
        onChange={updateInterferenceCell}
        readOnly={isAggregateInterference}
      />
    </div>
  );
}

function EditableMatrixTable({ title, matrix, onChange, readOnly = false }) {
  const rows = Object.keys(matrix || {});
  const columns = Array.from(new Set(rows.flatMap((row) => Object.keys(matrix[row] || {}))));
  return (
    <section className="data-section">
      <h2>{title}</h2>
      <div className="table-scroll editable-table-scroll">
        <table>
          <thead>
            <tr><th></th>{columns.map((column) => <th key={column}>{column}</th>)}</tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row}>
                <th>{row}</th>
                {columns.map((column) => (
                  <td key={column}>
                    {readOnly ? (
                      <span className="matrix-readonly">{fmt(matrix[row]?.[column] ?? 0)}</span>
                    ) : (
                      <input
                        className="matrix-input"
                        type="number"
                        step="0.0001"
                        value={matrix[row]?.[column] ?? 0}
                        onChange={(event) => onChange(row, column, event.target.value)}
                      />
                    )}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function DagView({ result, selectedTaskId, onSelect }) {
  const assignmentByTask = Object.fromEntries((result.assignments || []).map((item) => [item.task_id, item]));
  const colorByResource = resourceColors(result.resources);
  const positions = result.workflow.tasks.map((task, index) => {
    const columns = Math.ceil(Math.sqrt(result.workflow.tasks.length * 1.8));
    const col = index % columns;
    const row = Math.floor(index / columns);
    return { task, x: 110 + col * 210, y: 70 + row * 115 };
  });
  const positionByTask = Object.fromEntries(positions.map((item) => [item.task.id, item]));
  const width = Math.max(900, Math.max(...positions.map((item) => item.x)) + 120);
  const height = Math.max(520, Math.max(...positions.map((item) => item.y)) + 110);

  return (
    <svg className="dag" viewBox={`0 0 ${width} ${height}`}>
      <defs>
        <marker id="arrow" markerWidth="10" markerHeight="10" refX="7" refY="3" orient="auto">
          <path d="M0,0 L0,6 L8,3 z" fill="#667085" />
        </marker>
      </defs>
      {result.workflow.dependencies.map((edge) => {
        const source = positionByTask[edge.source];
        const target = positionByTask[edge.target];
        return <line key={`${edge.source}-${edge.target}`} x1={source.x + 78} y1={source.y + 20} x2={target.x - 82} y2={target.y + 20} markerEnd="url(#arrow)" />;
      })}
      {positions.map(({ task, x, y }) => {
        const resourceId = assignmentByTask[task.id]?.resource_id;
        return (
          <g key={task.id} className={selectedTaskId === task.id ? "selected-node" : ""} onClick={() => onSelect(task.id)}>
            <rect x={x - 84} y={y - 18} width="168" height="52" rx="4" fill={colorByResource[resourceId] || "#0f62fe"} />
            <text x={x} y={y + 1} textAnchor="middle">{compactLabel(task.id, 22)}</text>
            <text x={x} y={y + 19} textAnchor="middle">{task.workflow_stage}</text>
          </g>
        );
      })}
    </svg>
  );
}

function GanttView({ result, selectedTaskId, onSelect }) {
  const [visibleTiming, setVisibleTiming] = useState({
    interference: false,
    container: false,
    boot: false,
    transfer: false,
  });
  const maxVisibleFinish = result.assignments.reduce((maxValue, item) => {
    const baseRuntime = result.matrices.et_0[item.task_id]?.[item.resource_id] ?? item.effective_runtime;
    const interferenceRuntime = Math.max(0, item.effective_runtime - baseRuntime);
    const visibleRuntime = baseRuntime
      + (visibleTiming.interference ? interferenceRuntime : 0)
      + (visibleTiming.container ? item.container_overhead : 0)
      + (visibleTiming.boot ? item.boot_overhead : 0)
      + (visibleTiming.transfer ? item.transfer_delay : 0);
    return Math.max(maxValue, item.start_time + visibleRuntime);
  }, result.scheduler_variables.makespan);
  const timelineWidth = Math.max(760, maxVisibleFinish * 7);
  const scale = timelineWidth / Math.max(maxVisibleFinish, 1);
  const colorByResource = resourceColors(result.resources);
  function toggleTiming(key) {
    setVisibleTiming((current) => ({ ...current, [key]: !current[key] }));
  }
  return (
    <div className="gantt-wrap">
      <div className="gantt-toolbar">
        <div className="gantt-checks">
          <label className="checkbox-control">
            <input type="checkbox" checked={visibleTiming.interference} onChange={() => toggleTiming("interference")} />
            <span>Interference</span>
          </label>
          <label className="checkbox-control">
            <input type="checkbox" checked={visibleTiming.container} onChange={() => toggleTiming("container")} />
            <span>Container overhead</span>
          </label>
          <label className="checkbox-control">
            <input type="checkbox" checked={visibleTiming.boot} onChange={() => toggleTiming("boot")} />
            <span>Boot overhead</span>
          </label>
          <label className="checkbox-control">
            <input type="checkbox" checked={visibleTiming.transfer} onChange={() => toggleTiming("transfer")} />
            <span>Transfer delay</span>
          </label>
        </div>
        <div className="gantt-legend">
          <span><i className="legend-swatch base" />Base execution</span>
          <span><i className="legend-swatch interference" />Interference increase</span>
          <span><i className="legend-swatch container" />Container</span>
          <span><i className="legend-swatch boot" />Boot</span>
          <span><i className="legend-swatch transfer" />Transfer</span>
        </div>
      </div>
      <div className="gantt" style={{ "--timeline-width": `${timelineWidth}px` }}>
        {result.resources.map((resource) => (
          <section className="gantt-machine" key={resource.id}>
            <header className="gantt-machine-header">
              <div>
                <strong>{resource.name}</strong>
                <span>{resource.id}</span>
              </div>
              <div className="machine-badges">
                <span className={`status-badge ${resource.kind}`}>{resource.kind}</span>
                <span className={`status-badge ${resource.status}`}>{resource.status}</span>
              </div>
            </header>
            <div className="gantt-cores">
              {resource.cores.map((core) => (
                <div className="gantt-row" key={core.id}>
                  <div className="lane-label">Core {core.index + 1}</div>
                  <div className="lane-track">
	                    {result.assignments.filter((item) => item.core_id === core.id).map((item) => {
	                      const baseRuntime = result.matrices.et_0[item.task_id]?.[item.resource_id] ?? item.effective_runtime;
	                      const interferenceRuntime = Math.max(0, item.effective_runtime - baseRuntime);
	                      const timingSegments = [
	                        { key: "interference", className: "bar-interference", duration: interferenceRuntime, visible: visibleTiming.interference && interferenceRuntime > 0 },
	                        { key: "container", className: "bar-container", duration: item.container_overhead, visible: visibleTiming.container && item.container_overhead > 0 },
	                        { key: "boot", className: "bar-boot", duration: item.boot_overhead, visible: visibleTiming.boot && item.boot_overhead > 0 },
	                        { key: "transfer", className: "bar-transfer", duration: item.transfer_delay, visible: visibleTiming.transfer && item.transfer_delay > 0 },
	                      ];
	                      const visualRuntime = baseRuntime + timingSegments.reduce((sum, segment) => sum + (segment.visible ? segment.duration : 0), 0);
	                      const visualWidth = Math.max(34, visualRuntime * scale);
	                      let segmentOffset = baseRuntime;
	                      const hasInterference = item.phi_n > 0 && interferenceRuntime > 0;
	                      return (
	                        <button
                          key={item.task_id}
                          className={`bar ${hasInterference ? "has-interference" : ""} ${selectedTaskId === item.task_id ? "selected" : ""}`}
                          style={{
                            left: item.start_time * scale,
                            width: visualWidth,
                            "--bar-color": colorByResource[item.resource_id],
                          }}
                          onClick={() => onSelect(item.task_id)}
	                          title={`${item.task_id}: base ${fmt(baseRuntime)}s / interference +${fmt(interferenceRuntime)}s / container +${fmt(item.container_overhead)}s / boot +${fmt(item.boot_overhead)}s / transfer +${fmt(item.transfer_delay)}s`}
	                        >
	                          <span className="bar-base">{item.task_id}</span>
	                          {timingSegments.map((segment) => {
	                            if (!segment.visible) return null;
	                            const left = segmentOffset * scale;
	                            const width = Math.max(6, segment.duration * scale);
	                            segmentOffset += segment.duration;
	                            return <span key={segment.key} className={segment.className} style={{ left, width }} />;
	                          })}
	                        </button>
	                      );
	                    })}
                  </div>
                </div>
              ))}
            </div>
          </section>
        ))}
      </div>
    </div>
  );
}

function StepsView({ result, onSelect }) {
  const [stepIndex, setStepIndex] = useState(0);
  const steps = result.scheduler_steps || [];
  const step = steps[Math.min(stepIndex, Math.max(steps.length - 1, 0))];
  const task = result.workflow.tasks.find((item) => item.id === step?.task_id);
  const selectedCandidate = step?.candidates.find((candidate) => candidate.selected);

  if (!step) {
    return <div className="empty-state">No scheduler steps were returned.</div>;
  }

  return (
    <div className="steps-view">
      <section className="step-header data-section">
        <div>
          <span>Step {step.step} of {steps.length}</span>
          <h2>{task?.label || step.task_id}</h2>
          <p>{task?.workflow_stage} selected {step.selected_resource_id} / {step.selected_core_id}</p>
        </div>
        <div className="step-controls">
          <button className="secondary-button" onClick={() => setStepIndex((current) => Math.max(0, current - 1))} disabled={stepIndex === 0}>
            <ChevronLeft size={16} />
            Previous
          </button>
          <button className="secondary-button" onClick={() => setStepIndex((current) => Math.min(steps.length - 1, current + 1))} disabled={stepIndex >= steps.length - 1}>
            Next
            <ChevronRight size={16} />
          </button>
        </div>
      </section>

      <div className="stats-grid">
        <Metric label="Candidates" value={step.candidates.length} />
        <Metric label="Selected score" value={fmt(step.selected_total_score)} />
        <Metric label="Selected ET*" value={fmt(selectedCandidate?.effective_runtime)} />
        <Metric label="Interference time" value={fmt(selectedCandidate?.interference_time)} />
      </div>

      {task?.run && (
        <section className="data-section">
          <h2>Activity command</h2>
          <pre>{task.run}</pre>
        </section>
      )}

      <section className="data-section">
        <h2>Candidate machine scores</h2>
        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                {["rank", "machine", "core", "selected", "ST", "FT", "ET_0", "ET*", "interference", "phi", "cost", "time score", "cost score", "total"].map((heading) => <th key={heading}>{heading}</th>)}
              </tr>
            </thead>
            <tbody>
              {step.candidates.map((candidate) => (
                <tr key={`${candidate.resource_id}-${candidate.core_id}`} className={candidate.selected ? "selected-row" : ""} onClick={() => onSelect(candidate.task_id)}>
                  <td>{candidate.rank}</td>
                  <td>{candidate.resource_id}</td>
                  <td>{candidate.core_id}</td>
                  <td>{candidate.selected ? "yes" : "no"}</td>
                  <td>{fmt(candidate.start_time)}</td>
                  <td>{fmt(candidate.finish_time)}</td>
                  <td>{fmt(candidate.base_runtime)}</td>
                  <td>{fmt(candidate.effective_runtime)}</td>
                  <td>{fmt(candidate.interference_time)}</td>
                  <td>{fmt(candidate.phi_n)}</td>
                  <td>{fmt(candidate.raw_cost)}</td>
                  <td>{fmt(candidate.score.time_score)}</td>
                  <td>{fmt(candidate.score.cost_score)}</td>
                  <td>{fmt(candidate.score.total_score)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="data-section">
        <h2>Pairwise interference for selected candidate</h2>
        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                {["other activity", "pair value", "cpu", "memory", "io", "network"].map((heading) => <th key={heading}>{heading}</th>)}
              </tr>
            </thead>
            <tbody>
              {(selectedCandidate?.pairwise_interference.length ? selectedCandidate.pairwise_interference : [{ other_task_id: "none", value: 0, dimensions: {} }]).map((pair) => (
                <tr key={pair.other_task_id}>
                  <td>{pair.other_task_id}</td>
                  <td>{fmt(pair.value)}</td>
                  <td>{fmt(pair.dimensions.cpu)}</td>
                  <td>{fmt(pair.dimensions.memory)}</td>
                  <td>{fmt(pair.dimensions.io)}</td>
                  <td>{fmt(pair.dimensions.network)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}

function ActivityStatsView({ result, onSelect }) {
  const stepByTask = Object.fromEntries((result.scheduler_steps || []).map((step) => [step.task_id, step]));
  const assignmentByTask = Object.fromEntries(result.assignments.map((assignment) => [assignment.task_id, assignment]));
  const machineUse = result.resources.map((resource) => ({
    resource,
    count: result.assignments.filter((assignment) => assignment.resource_id === resource.id).length,
  }));
  const totalInterference = result.assignments.reduce((sum, assignment) => {
    const baseRuntime = result.matrices.et_0[assignment.task_id]?.[assignment.resource_id] ?? assignment.effective_runtime;
    return sum + Math.max(0, assignment.effective_runtime - baseRuntime);
  }, 0);

  return (
    <div className="steps-view">
      <div className="stats-grid">
        <Metric label="Activities" value={result.workflow.tasks.length} />
        <Metric label="Machines used" value={machineUse.filter((item) => item.count > 0).length} />
        <Metric label="Total interference time" value={fmt(totalInterference)} />
        <Metric label="Total candidates" value={(result.scheduler_steps || []).reduce((sum, step) => sum + step.candidates.length, 0)} />
      </div>

      <section className="data-section">
        <h2>Activity statistics</h2>
        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                {["activity", "stage", "machine", "core", "rank", "candidates", "ET_0", "ET*", "interference", "phi", "FC", "score"].map((heading) => <th key={heading}>{heading}</th>)}
              </tr>
            </thead>
            <tbody>
              {result.workflow.tasks.map((task) => {
                const assignment = assignmentByTask[task.id];
                const step = stepByTask[task.id];
                const selected = step?.candidates.find((candidate) => candidate.selected);
                return (
                  <tr key={task.id} onClick={() => onSelect(task.id)}>
                    <td>{task.id}</td>
                    <td>{task.workflow_stage}</td>
                    <td>{assignment?.resource_id}</td>
                    <td>{assignment?.core_id}</td>
                    <td>{selected?.rank}</td>
                    <td>{step?.candidates.length}</td>
                    <td>{fmt(selected?.base_runtime)}</td>
                    <td>{fmt(selected?.effective_runtime)}</td>
                    <td>{fmt(selected?.interference_time)}</td>
                    <td>{fmt(assignment?.phi_n)}</td>
                    <td>{fmt(result.cost_variables.fc[task.id])}</td>
                    <td>{fmt(assignment?.score.total_score)}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </section>

      <section className="data-section">
        <h2>Machine usage</h2>
        <div className="machine-usage">
          {machineUse.map(({ resource, count }) => (
            <div className="usage-row" key={resource.id}>
              <span>{resource.name}</span>
              <div><i style={{ width: `${Math.max(4, (count / Math.max(result.assignments.length, 1)) * 100)}%` }} /></div>
              <strong>{count}</strong>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}

function PairwiseInterferenceView({ result, onSelect }) {
  const [sourceTaskId, setSourceTaskId] = useState(result.workflow.tasks[0]?.id || "");
  const [resourceFilter, setResourceFilter] = useState("all");
  const taskIds = result.workflow.tasks.map((task) => task.id);
  const rows = result.resources
    .filter((resource) => resourceFilter === "all" || resource.id === resourceFilter)
    .flatMap((resource) => {
      const resourceMatrix = result.matrices.interference_i_n[resource.id] || {};
      return taskIds
        .filter((targetTaskId) => targetTaskId !== sourceTaskId)
        .map((targetTaskId) => {
          const dimensions = {
            cpu: resourceMatrix.cpu?.[sourceTaskId]?.[targetTaskId] ?? 0,
            memory: resourceMatrix.memory?.[sourceTaskId]?.[targetTaskId] ?? 0,
            io: resourceMatrix.io?.[sourceTaskId]?.[targetTaskId] ?? 0,
            network: resourceMatrix.network?.[sourceTaskId]?.[targetTaskId] ?? 0,
          };
          const average = (dimensions.cpu + dimensions.memory + dimensions.io + dimensions.network) / 4;
          return {
            resource,
            sourceTaskId,
            targetTaskId,
            dimensions,
            average,
          };
        });
    })
    .sort((left, right) => right.average - left.average || left.resource.id.localeCompare(right.resource.id) || left.targetTaskId.localeCompare(right.targetTaskId));

  return (
    <div className="steps-view">
      <section className="data-section pairwise-toolbar">
        <div>
          <h2>Activity pair interference by machine</h2>
          <p>Rows show Activity A x Activity B on each machine.</p>
        </div>
        <div className="filter-row">
          <label className="control compact-control">
            <span>Activity A</span>
            <select value={sourceTaskId} onChange={(event) => setSourceTaskId(event.target.value)}>
              {taskIds.map((taskId) => <option key={taskId} value={taskId}>{taskId}</option>)}
            </select>
          </label>
          <label className="control compact-control">
            <span>Machine</span>
            <select value={resourceFilter} onChange={(event) => setResourceFilter(event.target.value)}>
              <option value="all">All machines</option>
              {result.resources.map((resource) => <option key={resource.id} value={resource.id}>{resource.name}</option>)}
            </select>
          </label>
        </div>
      </section>

      <div className="stats-grid">
        <Metric label="Pairs shown" value={rows.length} />
        <Metric label="Machines" value={resourceFilter === "all" ? result.resources.length : 1} />
        <Metric label="Max average" value={fmt(rows[0]?.average)} />
        <Metric label="Source activity" value={sourceTaskId} />
      </div>

      <section className="data-section">
        <h2>Pairwise matrix rows</h2>
        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                {["machine", "kind", "activity A", "activity B", "average", "cpu", "memory", "io", "network"].map((heading) => <th key={heading}>{heading}</th>)}
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={`${row.resource.id}-${row.sourceTaskId}-${row.targetTaskId}`} onClick={() => onSelect(row.sourceTaskId)}>
                  <td>{row.resource.name}</td>
                  <td>{row.resource.kind}</td>
                  <td>{row.sourceTaskId}</td>
                  <td>{row.targetTaskId}</td>
                  <td>{fmt(row.average)}</td>
                  <td>{fmt(row.dimensions.cpu)}</td>
                  <td>{fmt(row.dimensions.memory)}</td>
                  <td>{fmt(row.dimensions.io)}</td>
                  <td>{fmt(row.dimensions.network)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}

function MachineView({ result, selectedTaskId, onSelect }) {
  return (
    <div className="machine-grid">
      {result.resources.map((resource) => (
        <section className="machine" key={resource.id}>
          <header>
            <strong>{resource.name}</strong>
            <span>{resource.kind} / {resource.status}</span>
          </header>
          <div className="machine-specs">
            <span>CPU {resource.cpu}</span>
            <span>Mem {resource.memory} GB</span>
            <span>{resource.location}</span>
          </div>
          {resource.cores.map((core) => (
            <div className="core-lane" key={core.id}>
              <span>Core {core.index + 1}</span>
              <div>
                {result.assignments.filter((item) => item.core_id === core.id).map((item) => (
                  <button key={item.task_id} className={selectedTaskId === item.task_id ? "pill selected" : "pill"} onClick={() => onSelect(item.task_id)}>
                    {item.task_id} {fmt(item.start_time)}-{fmt(item.finish_time)}
                  </button>
                ))}
              </div>
            </div>
          ))}
        </section>
      ))}
    </div>
  );
}

function MatricesView({ result }) {
  const [interferenceResourceId, setInterferenceResourceId] = useState(result.resources[0]?.id || "");
  const [interferenceDimension, setInterferenceDimension] = useState("cpu");
  const isAggregateInterference = interferenceDimension === aggregateInterferenceDimension;
  const interferenceMatrix = isAggregateInterference
    ? buildAggregatedInterferenceMatrix(result.matrices.interference_i_n, interferenceResourceId)
    : result.matrices.interference_i_n[interferenceResourceId]?.[interferenceDimension] || {};
  const matrixEntries = [
    ["ET_0", result.matrices.et_0],
    ["ET*", result.matrices.et_star],
    ["Bandwidth BW", result.matrices.bandwidth_bw],
    ["Transfer delay", result.matrices.transfer_delay],
    ["Financial cost", result.matrices.financial_network_cost],
    ["Container overhead", result.matrices.container_overhead],
  ];
  return (
    <div className="matrix-stack">
      {matrixEntries.map(([title, matrix]) => <MatrixTable key={title} title={title} matrix={matrix} />)}
      <section className="data-section pairwise-toolbar">
        <div>
          <h2>Interference I_n</h2>
          <p>Interference is defined per machine and dimension.</p>
        </div>
        <div className="filter-row">
          <label className="control compact-control">
            <span>Machine</span>
            <select value={interferenceResourceId} onChange={(event) => setInterferenceResourceId(event.target.value)}>
              {result.resources.map((resource) => <option key={resource.id} value={resource.id}>{resource.name}</option>)}
            </select>
          </label>
          <label className="control compact-control">
            <span>Dimension</span>
            <select value={interferenceDimension} onChange={(event) => setInterferenceDimension(event.target.value)}>
              {interferenceDimensionOptions.map((dimension) => <option key={dimension} value={dimension}>{dimension}</option>)}
            </select>
          </label>
        </div>
      </section>
      <MatrixTable
        title={`Interference I_n / ${interferenceResourceId} / ${interferenceDimension}`}
        matrix={interferenceMatrix}
      />
    </div>
  );
}

function MatrixTable({ title, matrix }) {
  const rows = Object.keys(matrix || {});
  const columns = Array.from(new Set(rows.flatMap((row) => Object.keys(matrix[row] || {}))));
  return (
    <section className="data-section">
      <h2>{title}</h2>
      <div className="table-scroll">
        <table>
          <thead>
            <tr><th></th>{columns.map((column) => <th key={column}>{column}</th>)}</tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row}>
                <th>{row}</th>
                {columns.map((column) => <td key={column}>{fmt(matrix[row]?.[column])}</td>)}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function VariablesView({ result }) {
  const groups = [
    ["Timing", result.timing_variables],
    ["Scheduler", result.scheduler_variables],
    ["Cost", result.cost_variables],
    ["Interference", result.interference_variables],
    ["Deviation", result.deviation_variables],
  ];
  return (
    <div className="variables">
      {groups.map(([title, values]) => (
        <section className="data-section" key={title}>
          <h2>{title}</h2>
          <pre>{JSON.stringify(values, null, 2)}</pre>
        </section>
      ))}
    </div>
  );
}

function TablesView({ result, onSelect }) {
  return (
    <div className="data-section">
      <h2>Assignments</h2>
      <div className="table-scroll">
        <table>
          <thead>
            <tr>
              {["task", "resource", "core", "ST", "FT", "ET*", "transfer", "boot", "container", "Phi_n", "score"].map((heading) => <th key={heading}>{heading}</th>)}
            </tr>
          </thead>
          <tbody>
            {result.assignments.map((item) => (
              <tr key={item.task_id} onClick={() => onSelect(item.task_id)}>
                <td>{item.task_id}</td>
                <td>{item.resource_id}</td>
                <td>{item.core_id}</td>
                <td>{fmt(item.start_time)}</td>
                <td>{fmt(item.finish_time)}</td>
                <td>{fmt(item.effective_runtime)}</td>
                <td>{fmt(item.transfer_delay)}</td>
                <td>{fmt(item.boot_overhead)}</td>
                <td>{fmt(item.container_overhead)}</td>
                <td>{fmt(item.phi_n)}</td>
                <td>{fmt(item.score.total_score)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function DetailsPanel({ result, assignment, taskId }) {
  if (!result || !assignment) {
    return <div className="details-empty">Select a task to inspect its schedule.</div>;
  }
  const task = result.workflow.tasks.find((item) => item.id === taskId);
  const resource = result.resources.find((item) => item.id === assignment.resource_id);
  return (
    <div className="details">
      <h2>{task.label}</h2>
      <p>{task.workflow_stage} on {resource.name}</p>
      <Metric label="Start ST" value={fmt(assignment.start_time)} />
      <Metric label="Finish FT" value={fmt(assignment.finish_time)} />
      <Metric label="Effective ET*" value={fmt(assignment.effective_runtime)} />
      <Metric label="Base ET_0" value={fmt(result.matrices.et_0[task.id]?.[assignment.resource_id])} />
      <Metric label="Interference time" value={fmt(Math.max(0, assignment.effective_runtime - (result.matrices.et_0[task.id]?.[assignment.resource_id] ?? assignment.effective_runtime)))} />
      <Metric label="Transfer delay" value={fmt(assignment.transfer_delay)} />
      <Metric label="Boot overhead" value={fmt(assignment.boot_overhead)} />
      <Metric label="Container overhead" value={fmt(assignment.container_overhead)} />
      <Metric label="Phi_n" value={fmt(assignment.phi_n)} />
      <Metric label="C_cpu" value={fmt(result.cost_variables.c_cpu[task.id])} />
      <Metric label="C_mem" value={fmt(result.cost_variables.c_mem[task.id])} />
      <Metric label="C_fin" value={fmt(result.cost_variables.c_fin[task.id])} />
      <Metric label="FC" value={fmt(result.cost_variables.fc[task.id])} />
      <Metric label="ET_obs" value={fmt(result.deviation_variables.et_obs[task.id])} />
      <Metric label="D_N" value={fmt(result.deviation_variables.d_n[task.id])} />
      {task.runtime && <Metric label="Runtime" value={task.runtime} />}
      <section className="score-box">
        <strong>Score breakdown</strong>
        <span>time {fmt(assignment.score.time_score)}</span>
        <span>cost {fmt(assignment.score.cost_score)}</span>
        <span>interference {fmt(assignment.score.interference_score)}</span>
        <span>total {fmt(assignment.score.total_score)}</span>
      </section>
      <section className="score-box">
        <strong>Predecessors</strong>
        {(task.predecessors.length ? task.predecessors : ["none"]).map((item) => <span key={item}>{item}</span>)}
      </section>
      {task.run && (
        <section className="score-box">
          <strong>Command</strong>
          <code>{task.run}</code>
        </section>
      )}
    </div>
  );
}

function resourceColors(resources) {
  const clusterPalette = ["#24a148", "#42be65", "#08bdba", "#0f62fe"];
  const cloudPalette = ["#f1c21b", "#ff832b", "#da1e28", "#a56eff"];
  let clusterIndex = 0;
  let cloudIndex = 0;
  return Object.fromEntries(
    resources.map((resource) => {
      const palette = resource.kind === "cluster" ? clusterPalette : cloudPalette;
      const index = resource.kind === "cluster" ? clusterIndex++ : cloudIndex++;
      return [resource.id, palette[index % palette.length]];
    }),
  );
}

function buildAggregatedInterferenceMatrix(interferenceMatrixByResource, resourceId) {
  const resourceMatrix = interferenceMatrixByResource?.[resourceId] || {};
  const activityIds = Array.from(
    new Set(
      interferenceDimensions.flatMap((dimension) => {
        const dimensionMatrix = resourceMatrix[dimension] || {};
        return [
          ...Object.keys(dimensionMatrix),
          ...Object.values(dimensionMatrix).flatMap((targets) => Object.keys(targets || {})),
        ];
      }),
    ),
  );
  return Object.fromEntries(
    activityIds.map((sourceId) => [
      sourceId,
      Object.fromEntries(
        activityIds.map((targetId) => {
          const total = interferenceDimensions.reduce(
            (sum, dimension) => sum + (resourceMatrix[dimension]?.[sourceId]?.[targetId] || 0),
            0,
          );
          return [targetId, Number((total / interferenceDimensions.length).toFixed(4))];
        }),
      ),
    ]),
  );
}

function fmt(value) {
  if (value === undefined || value === null) return "-";
  if (typeof value !== "number") return value;
  return Number.isInteger(value) ? String(value) : value.toFixed(3).replace(/0+$/, "").replace(/\.$/, "");
}

function compactLabel(value, maxLength) {
  if (!value || value.length <= maxLength) return value;
  return `${value.slice(0, maxLength - 1)}...`;
}

createRoot(document.getElementById("root")).render(<App />);
