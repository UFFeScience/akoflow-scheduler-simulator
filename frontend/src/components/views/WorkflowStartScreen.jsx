import { Play } from 'lucide-react';
import Metric from '../Metric.jsx';
import ResourcesSection from './ResourcesSection.jsx';
import SlaPolicySection from './SlaPolicySection.jsx';
import WorkflowSourceSection from './WorkflowSourceSection.jsx';

export default function WorkflowStartScreen({ controller }) {
  return (
    <div className="steps-view">
      <section className="data-section start-screen">
        <span>Step 1</span>
        <h2>Choose workflow source</h2>
        <p>Select a synthetic random workflow or import an Akoflow YAML workflow. The next screen shows the DAG and dependencies before matrices are saved.</p>
      </section>
      <section className="data-section setup-panel">
        <WorkflowSourceSection controller={controller} />
        <ResourcesSection controller={controller} />
        <SlaPolicySection controller={controller} />
        <button className="primary-button setup-submit" onClick={controller.generateWorkflowAndMatrices} disabled={controller.status === "running" || (controller.workflowMode === "yaml" && !controller.workflowYaml)}>
          <Play size={17} />
          {controller.workflowMode === "yaml" ? "Import workflow" : "Generate workflow"}
        </button>
        {controller.statusMessage && <p className={controller.status === "error" ? "status-message error" : "status-message"}>{controller.statusMessage}</p>}
      </section>
      <div className="stats-grid">
        <Metric label="Workflow source" value={controller.workflowMode === "yaml" ? "YAML import" : "Synthetic random"} />
        <Metric label="YAML file" value={controller.workflowFileName || "-"} />
      </div>
    </div>
  );
}
