import { getDecisionDirection, weightsForDecisionDirection } from '../../slaControls.js';
import DecisionDirectionControl from '../controls/DecisionDirectionControl.jsx';

export default function SlaPolicySection({ controller }) {
  const { request } = controller;
  const decisionDirection = getDecisionDirection(request);
  const updateDecisionDirection = (direction) => controller.updateWeights(weightsForDecisionDirection(direction));
  const clampInteger = (value, min, max) => Math.min(max, Math.max(min, Number(value) || min));
  const updateOptionalNumber = (key, value) => {
    const numericValue = Number(value);
    controller.updateRequest(key, value === "" || numericValue <= 0 ? null : numericValue);
  };

  return (
    <div className="setup-section">
      <h2>SLA policy</h2>
      <div className="sla-sections">
        <section className="sla-subsection">
          <header><strong>Optimization limits</strong><span>Beam search recommendations</span></header>
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
              <span>Beam width</span>
              <input type="number" min="120" max="10000" step="10" value={request.beam_width} onChange={(event) => controller.updateRequest("beam_width", clampInteger(event.target.value, 120, 10000))} />
            </label>
            <label className="control">
              <span>Recommendations</span>
              <input type="number" min="1" max="1000" step="1" value={request.option_count} onChange={(event) => controller.updateRequest("option_count", clampInteger(event.target.value, 1, 1000))} />
            </label>
          </div>
        </section>
        <section className="sla-subsection">
          <header><strong>Objective</strong><span>{decisionDirection === "time" ? "Finish earlier" : "Spend less"}</span></header>
          <DecisionDirectionControl request={request} onChange={updateDecisionDirection} label={null} />
        </section>
      </div>
    </div>
  );
}
