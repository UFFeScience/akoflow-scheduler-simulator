import { Upload } from 'lucide-react';
import ControlInput from '../controls/ControlInput.jsx';
import ControlSelect from '../controls/ControlSelect.jsx';

export default function WorkflowSourceSection({ controller }) {
  const c = controller;
  return (
    <div className="setup-section">
      <h2>Workflow source</h2>
      <div className="mode-selector">
        {["random", "yaml"].map((mode) => (
          <button type="button" key={mode} className={c.workflowMode === mode ? "mode-card active" : "mode-card"} onClick={() => c.setWorkflowMode(mode)}>
            <strong>{mode === "random" ? "Generate random workflow" : "Import Akoflow YAML"}</strong>
            <span>{mode === "random" ? "Use preset, task count, seed, and edge density." : "Use activities and dependsOn from a workflow file."}</span>
          </button>
        ))}
      </div>
      <div className="setup-grid">
        {c.workflowMode === "random" && (
          <>
            <ControlSelect label="Workflow preset" value={c.request.preset} onChange={(value) => c.updateRequest("preset", value)}>
              {(c.presets.length ? c.presets : [{ id: "Montage", label: "Montage" }]).map((preset) => (
                <option key={preset.id} value={preset.id}>{preset.label}</option>
              ))}
            </ControlSelect>
            <ControlInput label="Seed" value={c.request.seed} min={1} step={1} onChange={(value) => c.updateRequest("seed", value)} />
            <ControlInput label="Tasks" value={c.request.task_count} min={3} max={80} step={1} onChange={(value) => c.updateRequest("task_count", value)} />
            <ControlInput label="Edge density" value={c.request.edge_density} min={0} max={0.8} step={0.01} onChange={(value) => c.updateRequest("edge_density", value)} />
          </>
        )}
        {c.workflowMode === "yaml" && (
          <section className="workflow-import inline-import">
            <div><span>Akoflow workflow YAML</span><strong>{c.workflowFileName || "No YAML selected"}</strong></div>
            <label className="file-button">
              <Upload size={16} /> Import YAML
              <input type="file" accept=".yaml,.yml,text/yaml,text/x-yaml" onChange={(event) => c.importWorkflowFile(event.target.files?.[0])} />
            </label>
            {c.workflowYaml && <button className="secondary-button" type="button" onClick={c.clearWorkflowFile}>Clear YAML</button>}
          </section>
        )}
      </div>
    </div>
  );
}
