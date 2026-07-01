import { fmt } from '../../lib/format.js';

export default function MachineView({ result, selectedTaskId, onSelect }) {
  const stopIntervals = result.machine_stop_intervals || [];
  return (
    <div className="machine-grid">
      {result.resources.map((resource) => (
        <section className="machine" key={resource.id}>
          <header>
            <strong>{resource.name}</strong>
            <span>{resource.kind} / {resource.status}</span>
          </header>
          <div className="machine-specs">
            <span>Cores {resource.cores.length}</span>
            <span>Mem {resource.memory} GB</span>
            <span>BW {fmt(resource.bandwidth)} MB/s</span>
            <span>Boot {fmt(resource.boot_overhead)}s</span>
            <span>{resource.location}</span>
            <span>Stops {stopIntervals.filter((item) => item.resource_id === resource.id).length}</span>
          </div>
          {stopIntervals.filter((item) => item.resource_id === resource.id).map((item, index) => (
            <div className="core-lane stop-lane" key={`${item.resource_id}-${item.stop_time}-${index}`}>
              <span>Stopped</span>
              <div>
                <span className="pill stop-pill">
                  off {fmt(item.stop_time)}-{fmt(item.boot_start_time)} / boot {fmt(item.boot_start_time)}-{fmt(item.boot_finish_time)}
                </span>
              </div>
            </div>
          ))}
          {resource.cores.map((core) => (
            <div className="core-lane" key={core.id}>
              <span>Core {core.index + 1}</span>
              <div>
                {result.assignments.filter((item) => item.core_id === core.id).map((item) => (
                  <button key={item.task_id} className={selectedTaskId === item.task_id ? "pill selected" : "pill"} onClick={() => onSelect(item.task_id)}>
                    {item.task_id} {fmt(item.start_time)}-{fmt(item.finish_time)}
                  </button>
                ))}
              </div>
            </div>
          ))}
        </section>
      ))}
    </div>
  );
}
