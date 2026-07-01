import ControlInput from '../controls/ControlInput.jsx';
import MachinePatternEditor from './MachinePatternEditor.jsx';

export default function ResourcesSection({ controller }) {
  const { request } = controller;
  return (
    <div className="setup-section">
      <h2>Resources</h2>
      <div className="setup-grid">
        <ControlInput label="Cluster machines" value={request.cluster_machines} min={1} max={20} step={1} onChange={(value) => controller.updateRequest("cluster_machines", value)} />
        <ControlInput label="Cloud machines" value={request.cloud_machines} min={0} max={20} step={1} onChange={(value) => controller.updateRequest("cloud_machines", value)} />
      </div>
      <MachinePatternEditor resources={request.resource_specs || []} onChange={controller.updateResourceSpec} onReset={controller.resetResourceSpecs} />
    </div>
  );
}
