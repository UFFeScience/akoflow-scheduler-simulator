export default function GanttTaskBar({ item, result, layout, visibleTiming, color, selected, title, onSelect, scale }) {
  const baseRuntime = result.matrices.et_0[item.task_id]?.[item.resource_id] ?? item.effective_runtime;
  const interferenceRuntime = Math.max(0, item.effective_runtime - baseRuntime);
  const segments = [
    { key: "boot", className: "bar-boot", duration: item.boot_overhead, visible: visibleTiming.boot && item.boot_overhead > 0 },
    { key: "container", className: "bar-container", duration: item.container_overhead, visible: visibleTiming.container && item.container_overhead > 0 },
    { key: "base", className: "bar-execution", duration: baseRuntime, visible: true },
    { key: "interference", className: "bar-interference", duration: interferenceRuntime, visible: visibleTiming.interference && interferenceRuntime > 0 },
    { key: "transfer", className: "bar-transfer", duration: item.transfer_delay, visible: visibleTiming.transfer && item.transfer_delay > 0 },
  ];
  let offset = 0;
  return (
    <button className={`bar ${item.phi_n > 0 && interferenceRuntime > 0 ? "has-interference" : ""} ${selected ? "selected" : ""}`} style={{ left: layout.x, width: layout.width, "--bar-color": color }} onClick={() => onSelect(item.task_id)} title={title || item.task_id}>
      {segments.map((segment) => {
        if (!segment.visible) return null;
        const left = offset * scale;
        const width = Math.max(6, segment.duration * scale);
        offset += segment.duration;
        return <span key={segment.key} className={segment.className} style={{ left, width }}>{segment.key === "base" ? item.task_id : ""}</span>;
      })}
    </button>
  );
}
