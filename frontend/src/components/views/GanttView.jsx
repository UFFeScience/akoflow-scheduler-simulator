import { useGanttLayout } from '../../hooks/useGanttLayout.js';
import GanttChart from './GanttChart.jsx';
import GanttToolbar from './GanttToolbar.jsx';

export default function GanttView({ result, selectedTaskId, onSelect }) {
  const layout = useGanttLayout(result);

  return (
    <div className="gantt-wrap">
      <GanttToolbar visibleTiming={layout.visibleTiming} showDependencies={layout.showDependencies} onToggleTiming={layout.toggleTiming} onToggleDependencies={layout.toggleDependencies} />
      <GanttChart result={result} layout={layout} selectedTaskId={selectedTaskId} onSelect={onSelect} />
    </div>
  );
}
