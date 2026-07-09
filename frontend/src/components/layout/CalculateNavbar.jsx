import { Play } from 'lucide-react';
import { normalizeWeights } from '../../slaControls.js';
import WeightSliderControl from '../controls/WeightSliderControl.jsx';

export default function CalculateNavbar({ controller }) {
  const { request } = controller;
  const total = Math.round((request.weight_time + request.weight_cost) * 100);
  const updateWeight = (key, value) => controller.updateWeights(normalizeWeights(request, key, value));
  const updateOptionalNumber = (key, value) => {
    const numericValue = Number(value);
    controller.updateRequest(key, value === "" || numericValue <= 0 ? null : numericValue);
  };

  return (
    <section className="calculate-navbar">
      <div className="calculate-title">
        <strong>SLA policy</strong>
        <span>Calculate current workflow and matrices</span>
      </div>
      <div className="calculate-controls">
        <WeightSliderControl label="Finish earlier" value={request.weight_time} help="Higher values prioritize lower finish time." onChange={(value) => updateWeight("weight_time", value)} />
        <WeightSliderControl label="Spend less" value={request.weight_cost} help="Higher values prioritize lower cost." onChange={(value) => updateWeight("weight_cost", value)} />
        <label className="compact-control">
          <span>Budget</span>
          <input type="number" min="0" step="0.01" value={request.budget_limit ?? ""} onChange={(event) => updateOptionalNumber("budget_limit", event.target.value)} />
        </label>
        <label className="compact-control">
          <span>Deadline</span>
          <input type="number" min="0" step="0.1" value={request.deadline_limit ?? ""} onChange={(event) => updateOptionalNumber("deadline_limit", event.target.value)} />
        </label>
        <label className="compact-control options-control">
          <span>N</span>
          <input type="number" min="1" max="100" step="1" value={request.option_count} onChange={(event) => controller.updateRequest("option_count", Math.min(100, Math.max(1, Number(event.target.value) || 1)))} />
        </label>
      </div>
      <button className="primary-button calculate-button" onClick={controller.calculateCurrentSchedule} disabled={controller.status === "running" || !controller.generated}>
        <Play size={17} />
        {controller.status === "running" ? "Calculating" : `Calculate (${total}%)`}
      </button>
    </section>
  );
}
