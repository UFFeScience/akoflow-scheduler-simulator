import { normalizeWeights } from '../../slaControls.js';
import WeightSliderControl from '../controls/WeightSliderControl.jsx';

export default function SlaPolicySection({ controller }) {
  const { request } = controller;
  const weightTotal = request.weight_time + request.weight_cost + request.weight_interference;
  const updateWeight = (key, value) => controller.updateWeights(normalizeWeights(request, key, value));

  return (
    <div className="setup-section">
      <h2>SLA policy</h2>
      <div className="sla-sections">
        <section className="sla-subsection">
          <header><strong>Decision weights</strong><span>Total {Math.round(weightTotal * 100)}%</span></header>
          <div className="weight-slider-stack">
            <WeightSliderControl label="Finish earlier" value={request.weight_time} help="Multiplies the time score. Higher values prefer candidates with lower finish times." onChange={(value) => updateWeight("weight_time", value)} />
            <WeightSliderControl label="Spend less" value={request.weight_cost} help="Multiplies the cost score. Higher values prefer lower CPU and memory execution cost." onChange={(value) => updateWeight("weight_cost", value)} />
            <WeightSliderControl label="Avoid interference" value={request.weight_interference} help="Multiplies phi_n. Higher values avoid overlapping colocated tasks that inflate ET*." onChange={(value) => updateWeight("weight_interference", value)} />
          </div>
        </section>
      </div>
    </div>
  );
}
