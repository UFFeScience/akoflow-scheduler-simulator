import Metric from '../Metric.jsx';
import { fmt } from '../../lib/format.js';

export default function StatsSummary({ result, machineDistribution }) {
  const totalInterference = result.assignments.reduce((sum, assignment) => {
    const baseRuntime = result.matrices.et_0[assignment.task_id]?.[assignment.resource_id] ?? assignment.effective_runtime;
    return sum + Math.max(0, assignment.effective_runtime - baseRuntime);
  }, 0);
  return (
    <div className="stats-grid">
      <Metric label="Activities" value={result.workflow.tasks.length} />
      <Metric label="Machines used" value={machineDistribution.filter((item) => item.count > 0).length} />
      <Metric label="Total interference time" value={fmt(totalInterference)} />
      <Metric label="Total candidates" value={(result.scheduler_steps || []).reduce((sum, step) => sum + step.candidates.length, 0)} />
    </div>
  );
}
