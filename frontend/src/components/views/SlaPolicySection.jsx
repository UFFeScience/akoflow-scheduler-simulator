import { normalizeWeights } from '../../slaControls.js';
import SliderNumberControl from '../controls/SliderNumberControl.jsx';
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
          <header><strong>Scheduling targets</strong><span>Used while ranking candidate machines.</span></header>
          <div className="setup-grid">
            <SliderNumberControl label="Deadline" value={request.deadline} min={1} max={500} step={1} suffix="s" help="Candidate time score = finish time / deadline. Lower values push the scheduler toward earlier finishes." onChange={(value) => controller.updateRequest("deadline", value)} />
            <SliderNumberControl label="Budget" value={request.budget} min={0} max={1000} step={1} help="Candidate cost score = execution cost / budget. A zero budget disables cloud machines." onChange={(value) => controller.updateRequest("budget", value)} />
          </div>
        </section>
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
