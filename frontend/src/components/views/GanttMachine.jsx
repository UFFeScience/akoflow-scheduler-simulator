import GanttStopWindow from './GanttStopWindow.jsx';
import GanttTaskBar from './GanttTaskBar.jsx';

export default function GanttMachine({ resource, y, result, layout, selectedTaskId, onSelect }) {
  return (
    <section className="gantt-machine" key={resource.id} style={{ top: y }}>
      <header className="gantt-machine-header">
        <div><strong>{resource.name}</strong><span>{resource.id}</span></div>
        <div className="machine-badges">
          <span className={`status-badge ${resource.kind}`}>{resource.kind}</span>
          <span className={`status-badge ${resource.status}`}>{resource.status}</span>
        </div>
      </header>
      <div className="gantt-cores">
        {resource.cores.map((core) => (
          <div className="gantt-row" key={core.id}>
            <div className="lane-label">Core {core.index + 1}</div>
            <div className="lane-track">
              {layout.visibleTiming.stopped && layout.stopIntervals
                .filter((item) => item.resource_id === resource.id)
                .map((item, index) => <GanttStopWindow key={`${item.resource_id}-${item.stop_time}-${index}`} item={item} resource={resource} timelineOrigin={layout.timelineOrigin} scale={layout.scale} index={index} />)}
              {result.assignments.filter((item) => item.core_id === core.id).map((item) => (
                <GanttTaskBar
                  key={item.task_id}
                  item={item}
                  result={result}
                  layout={layout.assignmentLayoutByTask[item.task_id]}
                  visibleTiming={layout.visibleTiming}
                  color={layout.colorByResource[item.resource_id]}
                  selected={selectedTaskId === item.task_id}
                  title={layout.ganttContextByTask[item.task_id]?.title}
                  onSelect={onSelect}
                  scale={layout.scale}
                />
              ))}
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
