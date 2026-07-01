import { Play } from 'lucide-react';
import { normalizeWeights } from '../../slaControls.js';
import WeightSliderControl from '../controls/WeightSliderControl.jsx';

export default function CalculateNavbar({ controller }) {
  const { request } = controller;
  const total = Math.round((request.weight_time + request.weight_cost + request.weight_interference) * 100);
  const updateWeight = (key, value) => controller.updateWeights(normalizeWeights(request, key, value));

  return (
    <section className="calculate-navbar">
      <div className="calculate-title">
        <strong>SLA policy</strong>
        <span>Calculate current workflow and matrices</span>
      </div>
      <div className="calculate-controls">
        <WeightSliderControl label="Finish earlier" value={request.weight_time} help="Higher values prioritize lower finish time." onChange={(value) => updateWeight("weight_time", value)} />
        <WeightSliderControl label="Spend less" value={request.weight_cost} help="Higher values prioritize lower cost." onChange={(value) => updateWeight("weight_cost", value)} />
        <WeightSliderControl label="Avoid interference" value={request.weight_interference} help="Higher values avoid co-location slowdown." onChange={(value) => updateWeight("weight_interference", value)} />
      </div>
      <button className="primary-button calculate-button" onClick={controller.calculateCurrentSchedule} disabled={controller.status === "running" || !controller.generated}>
        <Play size={17} />
        {controller.status === "running" ? "Calculating" : `Calculate (${total}%)`}
      </button>
    </section>
  );
}
