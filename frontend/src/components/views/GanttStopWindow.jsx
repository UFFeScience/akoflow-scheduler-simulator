import { fmt } from '../../lib/format.js';

export default function GanttStopWindow({ item, resource, timelineOrigin, scale, index }) {
  return (
    <span
      className="machine-stop-window"
      style={{ left: (item.stop_time - timelineOrigin) * scale, width: Math.max(6, (item.boot_start_time - item.stop_time) * scale) }}
      title={`${resource.name} stopped ${fmt(item.stop_time)}-${fmt(item.boot_start_time)}s; boot ${fmt(item.boot_start_time)}-${fmt(item.boot_finish_time)}s`}
      key={`${item.resource_id}-${item.stop_time}-${index}`}
    />
  );
}
