import GanttDependencies from './GanttDependencies.jsx';
import GanttMachine from './GanttMachine.jsx';

export default function GanttChart({ result, layout, selectedTaskId, onSelect }) {
  return (
    <div
      className="gantt"
      style={{
        "--timeline-width": `${layout.timelineWidth}px`,
        "--gantt-body-height": `${layout.ganttBodyHeight}px`,
        "--gantt-track-offset": `${layout.trackOffsetX}px`,
      }}
    >
      {layout.showDependencies && (
        <GanttDependencies
          dependencies={result.workflow.dependencies}
          layouts={layout.assignmentLayoutByTask}
          width={layout.timelineWidth}
          height={layout.ganttBodyHeight}
        />
      )}
      {layout.machinePositions.map(({ resource, y }) => (
        <GanttMachine key={resource.id} resource={resource} y={y} result={result} layout={layout} selectedTaskId={selectedTaskId} onSelect={onSelect} />
      ))}
    </div>
  );
}
