import ControlInput from '../controls/ControlInput.jsx';
import ControlSelect from '../controls/ControlSelect.jsx';
import MachinePatternEditor from './MachinePatternEditor.jsx';

export default function ResourcesSection({ controller }) {
  const { request } = controller;
  const usingExperimentScenario = Boolean(request.experiment_scenario_id);
  return (
    <div className="setup-section">
      <h2>Resources</h2>
      <ControlSelect label="Experiment scenario" value={request.experiment_scenario_id || ""} onChange={(value) => controller.updateRequest("experiment_scenario_id", value)}>
        <option value="">Manual machine pattern</option>
        {(controller.experimentScenarios || []).map((scenario) => (
          <option key={scenario.id} value={scenario.id}>
            {scenario.label} ({scenario.machine_count} machines)
          </option>
        ))}
      </ControlSelect>
      <div className="setup-grid">
        <ControlInput label="Cluster machines" value={request.cluster_machines} min={1} max={20} step={1} onChange={(value) => controller.updateRequest("cluster_machines", value)} />
        <ControlInput label="Cloud machines" value={request.cloud_machines} min={0} max={20} step={1} onChange={(value) => controller.updateRequest("cloud_machines", value)} />
      </div>
      {usingExperimentScenario ? (
        <p className="status-message">Machine pattern will be loaded from experiments/machine_simulators.csv.</p>
      ) : (
        <MachinePatternEditor resources={request.resource_specs || []} onChange={controller.updateResourceSpec} onReset={controller.resetResourceSpecs} />
      )}
    </div>
  );
}
