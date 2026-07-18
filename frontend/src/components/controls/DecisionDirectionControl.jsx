import { decisionDirections, getDecisionDirection } from '../../slaControls.js';

export default function DecisionDirectionControl({ request, onChange, compact = false, label = "Objective" }) {
  const selectedDirection = getDecisionDirection(request);
  const isCostDirection = selectedDirection === "cost";

  return (
    <div className={compact ? "decision-direction compact-decision-direction" : "decision-direction"}>
      {label && <span>{label}</span>}
      <label className="decision-switch">
        <strong className={!isCostDirection ? "active" : ""}>{decisionDirections.time.label}</strong>
        <input
          type="checkbox"
          checked={isCostDirection}
          role="switch"
          aria-label="Decision direction"
          onChange={(event) => onChange(event.target.checked ? "cost" : "time")}
        />
        <span className="decision-switch-track" aria-hidden="true" />
        <strong className={isCostDirection ? "active" : ""}>{decisionDirections.cost.label}</strong>
      </label>
      {!compact && <small>{decisionDirections[selectedDirection].help}</small>}
    </div>
  );
}
