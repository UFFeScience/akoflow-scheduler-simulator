import { buildActivityStats } from '../../lib/activityStats.js';
import ActivityStatsTable from './ActivityStatsTable.jsx';
import MachineUsageSection from './MachineUsageSection.jsx';
import NodeDistributionHeatmap from './NodeDistributionHeatmap.jsx';
import NodeLoadHeatmap from './NodeLoadHeatmap.jsx';
import StatsSummary from './StatsSummary.jsx';

export default function ActivityStatsView({ result, onSelect }) {
  const stats = buildActivityStats(result);

  return (
    <div className="steps-view">
      <StatsSummary result={result} machineDistribution={stats.machineDistribution} />
      <NodeDistributionHeatmap machineDistribution={stats.machineDistribution} totalRuntime={stats.totalRuntime} maxima={stats.maxima} />
      <NodeLoadHeatmap timeBuckets={stats.timeBuckets} nodeTimeCells={stats.nodeTimeCells} onSelect={onSelect} />
      <ActivityStatsTable result={result} assignmentByTask={stats.assignmentByTask} stepByTask={stats.stepByTask} onSelect={onSelect} />
      <MachineUsageSection machineUse={stats.machineUse} assignmentCount={result.assignments.length} />
    </div>
  );
}
