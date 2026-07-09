import { normalizeWeights } from '../../slaControls.js';
import WeightSliderControl from '../controls/WeightSliderControl.jsx';

export default function SlaPolicySection({ controller }) {
  const { request } = controller;
  const weightTotal = request.weight_time + request.weight_cost;
  const updateWeight = (key, value) => controller.updateWeights(normalizeWeights(request, key, value));
  const updateOptionalNumber = (key, value) => {
    const numericValue = Number(value);
    controller.updateRequest(key, value === "" || numericValue <= 0 ? null : numericValue);
  };

  return (
    <div className="setup-section">
      <h2>SLA policy</h2>
      <div className="sla-sections">
        <section className="sla-subsection">
          <header><strong>Decision weights</strong><span>Total {Math.round(weightTotal * 100)}%</span></header>
          <div className="weight-slider-stack">
            <WeightSliderControl label="Finish earlier" value={request.weight_time} help="Multiplies the time score. Higher values prefer candidates with lower finish times." onChange={(value) => updateWeight("weight_time", value)} />
            <WeightSliderControl label="Spend less" value={request.weight_cost} help="Multiplies the cost score. Higher values prefer lower CPU and memory execution cost." onChange={(value) => updateWeight("weight_cost", value)} />
          </div>
        </section>
        <section className="sla-subsection">
          <header><strong>Optimization limits</strong><span>Beam search options</span></header>
          <div className="constraint-grid">
            <label className="control">
              <span>Budget limit</span>
              <input type="number" min="0" step="0.01" value={request.budget_limit ?? ""} onChange={(event) => updateOptionalNumber("budget_limit", event.target.value)} />
            </label>
            <label className="control">
              <span>Deadline makespan</span>
              <input type="number" min="0" step="0.1" value={request.deadline_limit ?? ""} onChange={(event) => updateOptionalNumber("deadline_limit", event.target.value)} />
            </label>
            <label className="control">
              <span>Options</span>
              <input type="number" min="1" max="100" step="1" value={request.option_count} onChange={(event) => controller.updateRequest("option_count", Math.min(100, Math.max(1, Number(event.target.value) || 1)))} />
            </label>
          </div>
        </section>
      </div>
    </div>
  );
}
