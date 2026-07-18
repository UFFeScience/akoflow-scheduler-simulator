import { Play, SlidersHorizontal } from 'lucide-react';
import { weightsForDecisionDirection } from '../../slaControls.js';
import DecisionDirectionControl from '../controls/DecisionDirectionControl.jsx';

export default function CalculateNavbar({ controller }) {
  const { request } = controller;
  const updateDecisionDirection = (direction) => controller.updateWeights(weightsForDecisionDirection(direction));
  const clampInteger = (value, min, max) => Math.min(max, Math.max(min, Number(value) || min));
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
        <label className="compact-control">
          <span>Budget</span>
          <input type="number" min="0" step="0.01" value={request.budget_limit ?? ""} onChange={(event) => updateOptionalNumber("budget_limit", event.target.value)} />
        </label>
        <label className="compact-control">
          <span>Deadline</span>
          <input type="number" min="0" step="0.1" value={request.deadline_limit ?? ""} onChange={(event) => updateOptionalNumber("deadline_limit", event.target.value)} />
        </label>
        <section className="calculate-objective-section">
          <header>Objective</header>
          <DecisionDirectionControl request={request} onChange={updateDecisionDirection} compact label={null} />
        </section>
        <details className="calculate-advanced-controls">
          <summary>
            <SlidersHorizontal size={15} />
            Advanced
          </summary>
          <div>
            <label className="compact-control options-control">
              <span>Beam</span>
              <input type="number" min="120" max="10000" step="10" value={request.beam_width} onChange={(event) => controller.updateRequest("beam_width", clampInteger(event.target.value, 120, 10000))} />
            </label>
            <label className="compact-control options-control">
              <span>Recommendations</span>
              <input type="number" min="1" max="1000" step="1" value={request.option_count} onChange={(event) => controller.updateRequest("option_count", clampInteger(event.target.value, 1, 1000))} />
            </label>
          </div>
        </details>
      </div>
      <button className="primary-button calculate-button" onClick={controller.calculateCurrentSchedule} disabled={controller.status === "running" || !controller.generated}>
        <Play size={17} />
        {controller.status === "running" ? "Calculating" : "Calculate"}
      </button>
    </section>
  );
}
