import Metric from '../Metric.jsx';
import { fmt } from '../../lib/format.js';

export default function StatsSummary({ result, machineDistribution }) {
  const totalInterference = result.interference_variables?.total_interference_time ?? 0;
  const averagePhi = result.interference_variables?.average_phi_n ?? 0;
  return (
    <div className="stats-grid">
      <Metric label="Activities" value={result.workflow.tasks.length} />
      <Metric label="Machines used" value={machineDistribution.filter((item) => item.count > 0).length} />
      <Metric label="Total interference time" value={fmt(totalInterference)} />
      <Metric label="Average phi_n" value={fmt(averagePhi)} />
      <Metric label="Total candidates" value={(result.scheduler_steps || []).reduce((sum, step) => sum + step.candidates.length, 0)} />
    </div>
  );
}
