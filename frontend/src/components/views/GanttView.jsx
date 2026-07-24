import { useGanttLayout } from '../../hooks/useGanttLayout.js';
import GanttChart from './GanttChart.jsx';
import GanttToolbar from './GanttToolbar.jsx';

export default function GanttView({ result, selectedTaskId, onSelect }) {
  const layout = useGanttLayout(result);

  return (
    <div className="gantt-wrap">
      <GanttToolbar
        visibleTiming={layout.visibleTiming}
        showDependencies={layout.showDependencies}
        laneMode={layout.laneMode}
        onToggleTiming={layout.toggleTiming}
        onToggleDependencies={layout.toggleDependencies}
        onLaneModeChange={layout.setLaneMode}
      />
      <GanttChart result={result} layout={layout} selectedTaskId={selectedTaskId} onSelect={onSelect} />
    </div>
  );
}
